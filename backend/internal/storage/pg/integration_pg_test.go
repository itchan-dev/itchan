// Package pg_test contains the integration tests for the PostgreSQL storage layer.
//
// Test Philosophy:
// This package employs a robust testing strategy to ensure reliability and isolation:
//  1. Test Containerization: `TestMain` uses testcontainers-go to spin up a fresh, ephemeral
//     PostgreSQL instance for each run of the test suite. This guarantees that tests run
//     in a clean, predictable environment, identical to the production setup.
//  2. Transactional Tests: Each top-level test function (e.g., `TestDeleteMessage`) is
//     responsible for creating a single database transaction. All setup, execution, and
//     assertions for that test are performed within this transaction. At the end of the
//     test, the transaction is rolled back, wiping out all changes.
//  3. Test Helpers with Querier: The test helper functions (`createTestUser`, `createTestBoard`, etc.)
//     are designed to work with the transactional test pattern. They accept a `Querier`
//     interface as their first argument, allowing them to operate within the transaction
//     created by the calling test function. They call the *internal* (unexported) storage
//     methods, bypassing the public transaction-managing wrappers.
//  4. Build Tags for Test Isolation: Tests that cannot use transactions (e.g., concurrent
//     materialized view refreshes) are separated into files with the 'polluting' build tag.
//     Run clean tests with `go test ./...` and polluting tests with `go test -tags=polluting ./...`
//
// This approach provides perfect test isolation, preventing one test from impacting another,
// and ensures that the core database logic works correctly under atomic conditions.
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	dbName        = "itchan_test"
	dbUser        = "user"
	dbPassword    = "password"
	initScriptRel = "migrations/init.sql" // Relative path from the test file
)

var (
	// storage is a global instance of our storage layer, initialized once for the suite.
	storage *Storage
)

// TestMain is the entry point for the entire test suite. It sets up the
// database container and tears it down after all tests have run.
func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// mustSetup handles the creation of the container and storage instance.
	container, err := mustSetup(ctx)
	if err != nil {
		log.Fatalf("Failed to setup test environment: %v", err)
	}

	// Run all tests.
	exitCode := m.Run()

	// Teardown the container.
	teardown(ctx, container)
	os.Exit(exitCode)
}

// mustSetup initializes the testcontainers environment and the Storage service.
func mustSetup(ctx context.Context) (testcontainers.Container, error) {
	initScriptPath, err := filepath.Abs(initScriptRel)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve init script path: %w", err)
	}

	container, err := postgres.Run(ctx,
		"postgres:16-alpine", // Using a recent, stable version
		postgres.WithInitScripts(initScriptPath),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(15*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	portStr, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}
	port, _ := strconv.Atoi(portStr.Port())

	cfg := &config.Config{
		Public: config.Public{
			ThreadsPerPage:              3,
			NLastMsg:                    3,
			BumpLimit:                   15,
			BoardPreviewRefreshInterval: 60, // Use a longer interval for tests
			MessagesPerThreadPage:       10,
		},
		Private: config.Private{
			Pg: config.Pg{
				Host:     host,
				Port:     port,
				User:     dbUser,
				Password: dbPassword,
				Dbname:   dbName,
			},
		},
	}

	// Use a canceled context for New to prevent the view refresher from starting during tests.
	// Tests will manage the view refresh manually if needed.
	initCtx, cancel := context.WithCancel(ctx)
	cancel()
	storage, err = New(initCtx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	return container, nil
}

// teardown handles the cleanup of the storage connection and the test container.
func teardown(ctx context.Context, container testcontainers.Container) {
	if storage != nil {
		if err := storage.Cleanup(); err != nil {
			log.Printf("Error cleaning up storage: %v", err)
		}
	}
	if container != nil {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("Error terminating container: %v", err)
		}
	}
}

// =========================================================================
// Transactional Test Helpers
// =========================================================================

// createTestUser creates a user within the given transaction.
func createTestUser(t *testing.T, q Querier, email string) domain.UserId {
	t.Helper()
	user := domain.User{Email: domain.Email(email), PassHash: "test_hash"}
	userID, err := storage.saveUser(q, user)
	require.NoError(t, err)
	return userID
}

// createTestBoard creates a board and its partitions within the given transaction.
func createTestBoard(t *testing.T, q Querier, shortName domain.BoardShortName) {
	t.Helper()
	err := storage.createBoard(q, domain.BoardCreationData{
		Name:      "Test Board " + string(shortName),
		ShortName: shortName,
	})
	require.NoError(t, err)
}

// createTestThread creates a thread and its OP message within the given transaction.
func createTestThread(t *testing.T, q Querier, data domain.ThreadCreationData) (domain.ThreadId, domain.MsgId) {
	t.Helper()
	threadID, createdTs, err := storage.createThread(q, data)
	require.NoError(t, err)

	data.OpMessage.ThreadId = threadID
	data.OpMessage.CreatedAt = &createdTs
	data.OpMessage.Board = data.Board

	opMsgID, _, err := storage.createMessage(q, data.OpMessage)
	require.NoError(t, err)
	return threadID, opMsgID
}

// createTestMessage creates a message within the given transaction.
func createTestMessage(t *testing.T, q Querier, data domain.MessageCreationData) domain.MsgId {
	t.Helper()
	msgID, _, err := storage.createMessage(q, data)
	require.NoError(t, err)
	return msgID
}

// =========================================================================
// General Test Utility Functions
// =========================================================================

// generateString creates a short, unique, alphanumeric string for test data.
func generateString(t *testing.T) string {
	t.Helper()
	return strings.ReplaceAll(uuid.New().String()[:8], "-", "")
}

// requireNotFoundError asserts that an error is a not-found error with status code 404.
func requireNotFoundError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err, "Expected a not-found error, but got nil")
	var e *internal_errors.ErrorWithStatusCode
	require.ErrorAs(t, err, &e, "Error is not of type ErrorWithStatusCode")
	require.Equal(t, http.StatusNotFound, e.StatusCode, "Expected status code 404")
}

// getRandomAttachments generates a sample attachments slice for use in tests.
func getRandomAttachments(t *testing.T) domain.Attachments {
	t.Helper()
	f1Name := generateString(t)
	f2Name := generateString(t)
	attachments := domain.Attachments{
		&domain.Attachment{
			File: &domain.File{
				FileCommonMetadata: domain.FileCommonMetadata{
					Filename:  f1Name,
					SizeBytes: 1024,
					MimeType:  "image/jpeg",
				},
				FilePath:         f1Name,
				OriginalFilename: f1Name,
			},
		},
		&domain.Attachment{
			File: &domain.File{
				FileCommonMetadata: domain.FileCommonMetadata{
					Filename:  f2Name,
					SizeBytes: 2048,
					MimeType:  "image/png",
				},
				FilePath:         f2Name,
				OriginalFilename: f2Name,
			},
		},
	}
	return attachments
}

// beginTx starts a new transaction and returns it, failing the test on error.
// Also returns a cleanup function that rolls back the transaction.
func beginTx(t *testing.T) (*sql.Tx, func()) {
	t.Helper()
	tx, err := storage.db.Begin()
	require.NoError(t, err)
	return tx, func() { tx.Rollback() }
}

// requireThreadOrder verifies that threads appear in the expected order by title.
func requireThreadOrder(t *testing.T, threads []*domain.Thread, expectedTitles []string) {
	t.Helper()
	require.Len(t, threads, len(expectedTitles), "Thread count mismatch")
	for i, expectedTitle := range expectedTitles {
		require.Equal(t, expectedTitle, string(threads[i].Title),
			"Thread at index %d has wrong title: expected %q, got %q",
			i, expectedTitle, threads[i].Title)
	}
}

// requireMessageOrder verifies that messages appear in the expected order by text.
func requireMessageOrder(t *testing.T, messages []*domain.Message, expectedTexts []string) {
	t.Helper()
	require.Len(t, messages, len(expectedTexts), "Message count mismatch")
	for i, expectedText := range expectedTexts {
		require.Equal(t, expectedText, string(messages[i].Text),
			"Message at index %d has wrong text: expected %q, got %q",
			i, expectedText, messages[i].Text)
	}
}
