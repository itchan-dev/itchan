// Package pg implements the storage layer for the application using a PostgreSQL database.
// It provides concrete implementations of the service layer's storage interfaces.
//
// This package follows a "Public/Private Method" pattern:
//  1. Public, exported methods match the service interfaces. Their primary role is to
//     manage database transactions (begin, commit, rollback) and call the corresponding
//     private methods.
//  2. Private, unexported methods contain the core database logic. They accept a `Querier`
//     interface, making them transaction-agnostic and highly testable.
//
// This design keeps the service layer clean and unaware of the database implementation
// details while allowing for robust, atomic, and testable database operations.
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"

	"github.com/lib/pq"
	_ "github.com/lib/pq" // Registers the PostgreSQL driver.
)

// Querier is an interface that abstracts database operations.
// It is satisfied by both the standard library's *sql.DB (for single operations
// on the connection pool) and *sql.Tx (for operations within a transaction).
// This abstraction is the key to creating testable and composable database logic.
type Querier interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// Interface satisfaction checks.
// These are compile-time assertions that ensure our *Storage struct correctly
// implements all the required methods from the service layer interfaces.
// If a method signature in the interface changes or is missing from the implementation,
// this will cause a compile error, immediately alerting developers.
var _ service.AuthStorage = (*Storage)(nil)
var _ service.BoardStorage = (*Storage)(nil)
var _ service.ThreadStorage = (*Storage)(nil)
var _ service.MessageStorage = (*Storage)(nil)

// Storage is the central struct for the PostgreSQL persistence layer.
// It holds the database connection pool and application configuration, and acts
// as the receiver for all storage methods.
type Storage struct {
	db  *sql.DB
	cfg *config.Config
}

// New creates and returns a new Storage instance.
// It establishes a connection to the database and starts any necessary
// background processes, such as the materialized view refresher.
// This function is the main entry point for initializing the persistence layer.
func New(ctx context.Context, cfg *config.Config) (*Storage, error) {
	log.Print("Connecting to database...")
	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	log.Print("Successfully connected to database.")

	storage := &Storage{db, cfg}
	storage.StartPeriodicViewRefresh(ctx, cfg.Public.BoardPreviewRefreshInterval*time.Second)

	return storage, nil
}

// Connect establishes and verifies a connection to the PostgreSQL database.
func Connect(cfg *config.Config) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Private.Pg.Host, cfg.Private.Pg.Port, cfg.Private.Pg.User, cfg.Private.Pg.Password, cfg.Private.Pg.Dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool for imageboard workload.
	// These settings balance resource usage with performance for high read/write ratios.
	db.SetMaxOpenConns(25)                  // Limit concurrent connections to avoid exhausting DB max_connections
	db.SetMaxIdleConns(10)                  // Keep connections warm for burst traffic
	db.SetConnMaxLifetime(5 * time.Minute)  // Recycle connections to prevent stale connections
	db.SetConnMaxIdleTime(1 * time.Minute)  // Close idle connections to free resources

	// Ping the database to verify that the connection is alive.
	if err = db.Ping(); err != nil {
		db.Close() // Close the connection if ping fails.
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// Cleanup gracefully closes the database connection pool.
// It should be called during application shutdown.
func (s *Storage) Cleanup() error {
	return s.db.Close()
}

// =========================================================================
// Transaction Helper
// =========================================================================

// withTx executes a function within a database transaction.
// It handles the boilerplate of beginning, committing, and rolling back transactions,
// and respects context cancellation/timeout.
//
// If the provided function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
//
// Usage:
//
//	func (s *Storage) SomeOperation(ctx context.Context, data ...) error {
//	    return s.withTx(ctx, func(tx *sql.Tx) error {
//	        // Your transaction logic here
//	        return s.somePrivateMethod(tx, data)
//	    })
//	}
func (s *Storage) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
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
// SQL Identifier Helper Functions
// =========================================================================

// partitionName generates the raw name for a board-specific partition table.
// Example: ("tech", "messages") -> "messages_tech"
func partitionName(shortName domain.BoardShortName, table string) string {
	return fmt.Sprintf("%s_%s", table, shortName)
}

// PartitionName generates a properly quoted and escaped partition table name for use in SQL queries.
// This prevents SQL injection issues with dynamic table names.
// Example: ("tech", "messages") -> `"messages_tech"`
func PartitionName(shortName domain.BoardShortName, table string) string {
	return pq.QuoteIdentifier(partitionName(shortName, table))
}

// viewTableName generates the raw name for a board-specific materialized view.
// Example: ("tech") -> "board_preview_tech"
func viewTableName(shortName domain.BoardShortName) string {
	return fmt.Sprintf("board_preview_%s", shortName)
}

// ViewTableName generates a properly quoted and escaped materialized view name for use in SQL queries.
// Example: ("tech") -> `"board_preview_tech"`
func ViewTableName(shortName domain.BoardShortName) string {
	return pq.QuoteIdentifier(viewTableName(shortName))
}
