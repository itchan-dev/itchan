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
	"time"

	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	sharedstorage "github.com/itchan-dev/itchan/shared/storage/pg"
)

// Querier is an alias to the shared Querier interface.
// It abstracts database operations and is satisfied by *sql.DB and *sql.Tx.
// This interface is defined in shared/storage/pg and used throughout the application.
type Querier = sharedstorage.Querier

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
	logger.Log.Info("connecting to database")
	db, err := sharedstorage.Connect(cfg, sharedstorage.DefaultConnectionConfig())
	if err != nil {
		return nil, err
	}
	logger.Log.Info("successfully connected to database")

	storage := &Storage{db, cfg}
	storage.StartPeriodicViewRefresh(ctx, cfg.Public.BoardPreviewRefreshInterval*time.Second)

	return storage, nil
}

// Cleanup gracefully closes the database connection pool.
// It should be called during application shutdown.
func (s *Storage) Cleanup() error {
	return s.db.Close()
}

// =========================================================================
// Transaction Helper
// =========================================================================

// withTx is a convenience wrapper around shared storage's WithTx helper.
// It provides a method-style API while delegating to the shared implementation.
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
	return sharedstorage.WithTx(ctx, s.db, fn)
}

// =========================================================================
// SQL Identifier Helper Functions (Wrappers to Shared Storage)
// =========================================================================

// PartitionName is a convenience wrapper around shared storage's PartitionName.
// It generates a properly quoted and escaped partition table name for use in SQL queries.
// Example: ("tech", "messages") -> `"messages_tech"`
func PartitionName(shortName domain.BoardShortName, table string) string {
	return sharedstorage.PartitionName(shortName, table)
}

// ViewTableName is a convenience wrapper around shared storage's ViewTableName.
// It generates a properly quoted and escaped materialized view name for use in SQL queries.
// Example: ("tech") -> `"board_preview_tech"`
func ViewTableName(shortName domain.BoardShortName) string {
	return sharedstorage.ViewTableName(shortName)
}

// =========================================================================
// Media Garbage Collection Methods
// =========================================================================

// GetAllFilePaths returns all file paths stored in the database.
// This is used by the garbage collector to identify orphaned files.
// Returns both original file paths and thumbnail paths.
func (s *Storage) GetAllFilePaths() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT file_path FROM files WHERE file_path IS NOT NULL
		UNION
		SELECT thumbnail_path FROM files WHERE thumbnail_path IS NOT NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query file paths: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("failed to scan file path: %w", err)
		}
		paths = append(paths, path)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file paths: %w", err)
	}

	return paths, nil
}
