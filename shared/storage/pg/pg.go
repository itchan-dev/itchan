// Package pg provides core PostgreSQL database primitives for storage layers.
//
// This package contains reusable abstractions and utilities that can be shared
// across multiple services (backend, frontend, etc.). It follows the principle
// of composition over inheritance, allowing services to build their own storage
// layers on top of these primitives.
//
// Core Components:
//   - Querier: Interface for transaction-agnostic database operations
//   - WithTx: Helper for managing database transactions
//   - Connect: Configurable database connection establishment
//   - SQL Identifier Utilities: Safe partition and view name generation
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/lib/pq"
	_ "github.com/lib/pq" // Registers the PostgreSQL driver
)

// =========================================================================
// Core Interfaces
// =========================================================================

// Querier is an interface that abstracts database operations.
// It is satisfied by both the standard library's *sql.DB (for single operations
// on the connection pool) and *sql.Tx (for operations within a transaction).
// This abstraction is the key to creating testable and composable database logic.
//
// By programming against this interface rather than concrete types, code becomes:
//   - Testable: Can be mocked for unit tests
//   - Composable: Works in both transactional and non-transactional contexts
//   - Reusable: Same logic can be used across different services
type Querier interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// =========================================================================
// Connection Management
// =========================================================================

// ConnectionConfig holds database connection pool settings.
// Different services have different concurrency requirements:
//   - Backend API: Higher connection limits for concurrent user requests
//   - Frontend: Lower limits as it's primarily a rendering service
//   - Workers: Custom limits based on workload characteristics
type ConnectionConfig struct {
	MaxOpenConns    int           // Maximum number of open connections to the database
	MaxIdleConns    int           // Maximum number of idle connections in the pool
	ConnMaxLifetime time.Duration // Maximum amount of time a connection may be reused
	ConnMaxIdleTime time.Duration // Maximum amount of time a connection may be idle
}

// DefaultConnectionConfig returns sensible defaults for connection pooling.
// These settings are suitable for a typical backend API server.
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// LightweightConnectionConfig returns conservative connection pool settings.
// Suitable for services that don't need many concurrent database connections,
// such as a frontend rendering service or background workers.
func LightweightConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// Connect establishes and verifies a connection to the PostgreSQL database.
// It configures the connection pool according to the provided settings and
// verifies connectivity with a ping operation.
//
// The connection string is built from the shared config structure, ensuring
// all services use consistent database credentials and settings.
//
// Example:
//
//	cfg := config.MustLoad("config")
//	db, err := pg.Connect(cfg, pg.DefaultConnectionConfig())
//	if err != nil {
//	    log.Fatalf("Failed to connect: %v", err)
//	}
//	defer db.Close()
func Connect(cfg *config.Config, connCfg ConnectionConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Private.Pg.Host, cfg.Private.Pg.Port,
		cfg.Private.Pg.User, cfg.Private.Pg.Password,
		cfg.Private.Pg.Dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(connCfg.MaxOpenConns)
	db.SetMaxIdleConns(connCfg.MaxIdleConns)
	db.SetConnMaxLifetime(connCfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(connCfg.ConnMaxIdleTime)

	// Verify connection
	if err = db.Ping(); err != nil {
		db.Close() // Close the connection if ping fails
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// =========================================================================
// Transaction Helpers
// =========================================================================

// WithTx executes a function within a database transaction.
// It handles the boilerplate of beginning, committing, and rolling back transactions,
// and respects context cancellation/timeout.
//
// If the provided function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
//
// This pattern ensures transactions are always properly cleaned up, even if the
// function panics or returns early. The deferred Rollback() is a no-op if the
// transaction has already been committed.
//
// Usage:
//
//	err := pg.WithTx(ctx, db, func(tx *sql.Tx) error {
//	    // Your transaction logic here
//	    if err := someOperation(tx, data); err != nil {
//	        return err // Triggers rollback
//	    }
//	    return nil // Triggers commit
//	})
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // No-op if transaction is already committed

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// =========================================================================
// SQL Identifier Utilities
// =========================================================================
//
// These functions generate properly quoted and escaped SQL identifiers for
// dynamic table and view names. They prevent SQL injection vulnerabilities
// that could occur when constructing queries with user-provided board names.
//
// The imageboard uses partitioned tables (one per board) for messages and threads,
// allowing for efficient per-board operations and easier data management.

// partitionName generates the raw name for a board-specific partition table.
// Example: ("tech", "messages") -> "messages_tech"
func PartitionNameUnquoted(shortName domain.BoardShortName, table string) string {
	return fmt.Sprintf("%s_%s", table, shortName)
}

// PartitionName generates a properly quoted and escaped partition table name for use in SQL queries.
// This prevents SQL injection issues with dynamic table names.
//
// Example: ("tech", "messages") -> `"messages_tech"`
//
// Usage:
//
//	tableName := pg.PartitionName(board, "messages")
//	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1", tableName)
//	rows, err := db.Query(query, messageID)
func PartitionName(shortName domain.BoardShortName, table string) string {
	return pq.QuoteIdentifier(PartitionNameUnquoted(shortName, table))
}

// viewTableName generates the raw name for a board-specific materialized view.
// Example: ("tech") -> "board_preview_tech"
func ViewTableNameUnquoted(shortName domain.BoardShortName) string {
	return fmt.Sprintf("board_preview_%s", shortName)
}

// ViewTableName generates a properly quoted and escaped materialized view name for use in SQL queries.
//
// Materialized views are used for efficient board previews, caching the last N messages
// per thread to avoid expensive joins on every page load.
//
// Example: ("tech") -> `"board_preview_tech"`
//
// Usage:
//
//	viewName := pg.ViewTableName(board)
//	query := fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY %s", viewName)
//	_, err := db.Exec(query)
func ViewTableName(shortName domain.BoardShortName) string {
	return pq.QuoteIdentifier(ViewTableNameUnquoted(shortName))
}
