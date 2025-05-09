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
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	dbName        = "itchan"
	dbUser        = "user"
	dbPassword    = "password"
	initScriptRel = "migrations/init.sql"
)

var (
	storage    *Storage
	containers []testcontainers.Container
	cancel     context.CancelFunc
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	storage, containers, cancel = mustSetup(ctx)
	cancel() // refresh board_preview manually
	defer teardown(ctx)

	exitCode := m.Run()
	os.Exit(exitCode)
}

func mustSetup(ctx context.Context) (*Storage, []testcontainers.Container, context.CancelFunc) {
	// Resolve absolute path for init script
	initScriptPath, err := filepath.Abs(initScriptRel)
	if err != nil {
		log.Fatalf("failed to resolve init script path: %v", err)
	}

	container, err := postgres.Run(ctx,
		"postgres:17.4-alpine",
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
			ThreadsPerPage:              3,
			NLastMsg:                    3,
			BumpLimit:                   15,
			BoardPreviewRefreshInterval: 1,
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

	ctx, cancel := context.WithCancel(ctx)
	storage, err := New(ctx, &cfg)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}

	return storage, []testcontainers.Container{container}, cancel
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
func setupBoard(t *testing.T) domain.BoardShortName {
	t.Helper()
	boardName := "testboard"
	boardShortName := generateString(t)
	err := storage.CreateBoard(domain.BoardCreationData{Name: boardName, ShortName: boardShortName, AllowedEmails: nil})
	require.NoError(t, err, "CreateBoard should not return an error")

	t.Cleanup(func() {
		// cleanup every thread and msg
		err := storage.DeleteBoard(boardShortName)
		if err != nil {
			requireNotFoundError(t, err) // already deleted in test
		}
	})

	return boardShortName
}

func setupBoardAndThread(t *testing.T) (domain.BoardShortName, domain.ThreadId) {
	t.Helper()
	boardShortName := setupBoard(t)

	threadID, err := storage.CreateThread(domain.ThreadCreationData{Title: generateString(t), Board: boardShortName, OpMessage: domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 1}, Text: "Test OP"}})
	require.NoError(t, err, "CreateThread should not return an error")

	return boardShortName, threadID
}

func setupBoardAndThreadAndMessage(t *testing.T) (domain.BoardShortName, domain.ThreadId, domain.MsgId) {
	t.Helper()
	boardShortName, threadID := setupBoardAndThread(t)

	author := domain.User{Id: 2}
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID, err := storage.CreateMessage(domain.MessageCreationData{Board: boardShortName, Author: author, Text: generateString(t), Attachments: attachments, ThreadId: threadID}, false, nil)
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

func createTestThread(t *testing.T, thread domain.ThreadCreationData) domain.ThreadId {
	t.Helper()
	threadID, err := storage.CreateThread(thread)
	require.NoError(t, err)
	return threadID
}

func createTestMessage(t *testing.T, message domain.MessageCreationData) domain.MsgId {
	t.Helper()
	msgID, err := storage.CreateMessage(message, false, nil)
	require.NoError(t, err)
	return msgID
}

func requireThreadOrder(t *testing.T, threads []domain.Thread, expectedTitles []string) {
	t.Helper()
	require.Len(t, threads, len(expectedTitles))
	for i, title := range expectedTitles {
		require.Equal(t, title, threads[i].Title, "Position %d", i)
	}
}

func requireMessageOrder(t *testing.T, messages []domain.Message, expectedTexts []string) {
	t.Helper()
	require.Len(t, messages, len(expectedTexts))
	for i, text := range expectedTexts {
		require.Equal(t, text, messages[i].Text, "Position %d", i)
	}
}
