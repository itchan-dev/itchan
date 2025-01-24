package pg

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	dbName        = "itchan"
	dbUser        = "user"
	dbPassword    = "password"
	initScriptRel = "migrations/init.sql" // Adjust path according to your project structure
)

var (
	storage    *Storage
	containers []testcontainers.Container
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	storage, containers = mustSetup(ctx)
	defer teardown(ctx)

	exitCode := m.Run()
	os.Exit(exitCode)
}

func mustSetup(ctx context.Context) (*Storage, []testcontainers.Container) {
	// Resolve absolute path for init script
	initScriptPath, err := filepath.Abs(initScriptRel)
	if err != nil {
		log.Fatalf("failed to resolve init script path: %v", err)
	}

	container, err := postgres.Run(ctx,
		"postgres:15.3-alpine",
		postgres.WithInitScripts(initScriptPath),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForListeningPort("5432/tcp"),
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2),
			).WithDeadline(10*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("failed to start container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get container host: %v", err)
	}

	portStr, err := container.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("failed to get container port: %v", err)
	}
	port, _ := strconv.Atoi(portStr.Port())

	cfg := config.Config{
		Public: config.Public{
			ThreadsPerPage: 3,
			NLastMsg:       3,
			BumpLimit:      15,
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

	storage, err := New(&cfg)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}

	return storage, []testcontainers.Container{container}
}

func teardown(ctx context.Context) {
	if storage != nil {
		if err := storage.Cleanup(); err != nil {
			log.Printf("error cleaning up storage: %v", err)
		}
	}

	for _, container := range containers {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("error terminating container: %v", err)
		}
	}
}

// Helper functions
func setupBoard(t *testing.T) string {
	t.Helper()
	boardName := "testboard"
	boardShortName := generateString(t)
	err := storage.CreateBoard(boardName, boardShortName, nil)
	require.NoError(t, err, "CreateBoard should not return an error")

	t.Cleanup(func() {
		// cleanup every thread and msg
		err := storage.DeleteBoard(boardShortName)
		require.NoError(t, err, "DeleteBoard should not return an error")
	})

	return boardShortName
}

func setupBoardAndThread(t *testing.T) (string, int64) {
	t.Helper()
	boardShortName := setupBoard(t)

	threadID, err := storage.CreateThread(generateString(t), boardShortName, &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP"})
	require.NoError(t, err, "CreateThread should not return an error")

	return boardShortName, threadID
}

func setupBoardAndThreadAndMessage(t *testing.T) (string, int64, int64) {
	t.Helper()
	boardShortName, threadID := setupBoardAndThread(t)

	author := &domain.User{Id: 2}
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID, err := storage.CreateMessage(boardShortName, author, generateString(t), attachments, threadID)
	require.NoError(t, err, "CreateMessage should not return an error")

	return boardShortName, threadID, msgID
}

func generateString(t *testing.T) string {
	t.Helper()
	return strings.ReplaceAll(uuid.NewString()[:10], "-", "_")
}

func requireNotFoundError(t *testing.T, err error) {
	t.Helper()
	var e *internal_errors.ErrorWithStatusCode
	require.ErrorAs(t, err, &e)
	require.Equal(t, 404, e.StatusCode)
}

func createTestThread(t *testing.T, boardShortName, title string, msg *domain.Message) int64 {
	threadID, err := storage.CreateThread(title, boardShortName, msg)
	require.NoError(t, err)
	return threadID
}

func createTestMessage(t *testing.T, boardShortName string, user *domain.User, text string, attachments *domain.Attachments, threadID int64) int64 {
	msgID, err := storage.CreateMessage(boardShortName, user, text, attachments, threadID)
	require.NoError(t, err)
	return msgID
}

func requireThreadOrder(t *testing.T, threads []*domain.Thread, expectedTitles []string) {
	t.Helper()
	require.Len(t, threads, len(expectedTitles))
	for i, title := range expectedTitles {
		require.Equal(t, title, threads[i].Title, "Position %d", i)
	}
}

func requireMessageOrder(t *testing.T, messages []*domain.Message, expectedTexts []string) {
	t.Helper()
	require.Len(t, messages, len(expectedTexts))
	for i, text := range expectedTexts {
		require.Equal(t, text, messages[i].Text, "Position %d", i)
	}
}
