package pg

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper functions for board view testing

func refreshBoardView(t *testing.T, boardShortName domain.BoardShortName) {
	t.Helper()
	err := storage.refreshMaterializedViewConcurrent(boardShortName, 2*time.Second)
	require.NoError(t, err)
}

func getBoardAfterRefresh(t *testing.T, boardShortName domain.BoardShortName) domain.Board {
	t.Helper()
	refreshBoardView(t, boardShortName)
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	return board
}

func setBoardInactive(t *testing.T, boardShortName domain.BoardShortName) {
	t.Helper()
	_, err := storage.db.Exec("UPDATE boards SET last_activity = now() - interval '1 day' WHERE short_name = $1", boardShortName)
	require.NoError(t, err)
}

// TestRefreshMaterializedViewConcurrent tests the basic materialized view refresh functionality.
func TestRefreshMaterializedViewConcurrent(t *testing.T) {
	t.Run("success with existing board", func(t *testing.T) {
		boardShortName, _ := setupBoardAndThread(t)

		err := storage.refreshMaterializedViewConcurrent(boardShortName, 2*time.Second)
		require.NoError(t, err)

		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		assert.Equal(t, boardShortName, board.ShortName)
	})

	t.Run("fails for nonexistent board", func(t *testing.T) {
		nonExistentBoard := generateString(t)
		err := storage.refreshMaterializedViewConcurrent(nonExistentBoard, 500*time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "concurrent refresh failed")
	})

	t.Run("respects NLastMsg limit", func(t *testing.T) {
		boardShortName, threadID := setupBoardAndThread(t)

		// Add messages up to NLastMsg limit
		for i := 1; i <= storage.cfg.Public.NLastMsg; i++ {
			createTestMessage(t, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: 1},
				Text:     fmt.Sprintf("Reply %d", i),
				ThreadId: threadID,
			})
		}

		board := getBoardAfterRefresh(t, boardShortName)
		require.Len(t, board.Threads, 1)
		require.Len(t, board.Threads[0].Messages, storage.cfg.Public.NLastMsg+1) // OP + NLastMsg replies

		expectedTexts := []string{"Test OP", "Reply 1", "Reply 2", "Reply 3"}
		requireMessageOrder(t, board.Threads[0].Messages, expectedTexts)
	})

	t.Run("maintains limit when adding new messages", func(t *testing.T) {
		boardShortName, threadID := setupBoardAndThread(t)

		// Add messages up to limit
		for i := 1; i <= storage.cfg.Public.NLastMsg; i++ {
			createTestMessage(t, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: 1},
				Text:     fmt.Sprintf("Reply %d", i),
				ThreadId: threadID,
			})
		}
		board := getBoardAfterRefresh(t, boardShortName)

		// Add one more message
		createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: 1},
			Text:     "Reply 4",
			ThreadId: threadID,
		})
		board = getBoardAfterRefresh(t, boardShortName)

		// Should show OP + last NLastMsg replies
		expectedTexts := []string{"Test OP", "Reply 2", "Reply 3", "Reply 4"}
		requireMessageOrder(t, board.Threads[0].Messages, expectedTexts)
	})
}

// TestStartPeriodicViewRefresh tests the periodic refresh mechanism.
func TestStartPeriodicViewRefresh(t *testing.T) {
	t.Run("starts and stops without errors", func(t *testing.T) {
		boardShortName := setupBoard(t)

		ctx, cancel := context.WithCancel(context.Background())
		testInterval := 15 * time.Millisecond

		storage.StartPeriodicViewRefresh(ctx, testInterval)
		time.Sleep(testInterval)
		threadData := domain.ThreadCreationData{
			Title: "First thread",
			Board: boardShortName,
			OpMessage: domain.MessageCreationData{
				Author: domain.User{Id: int64(1)},
				Text:   "op",
			},
		}
		_ = createTestThread(t, threadData)
		time.Sleep(3 * testInterval) // Brief pause to ensure it started
		cancel()
		time.Sleep(50 * time.Millisecond) // Allow goroutine to exit gracefully

		// If we get here without panics/deadlocks, the test passes
		assert.True(t, true, "Periodic refresh started and stopped successfully")

		// Verify board is still accessible
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		assert.NotEmpty(t, board.Threads)
	})
}

// TestPeriodicRefreshIntegration tests the integration of periodic refresh with active boards.
func TestPeriodicRefreshIntegration(t *testing.T) {
	t.Run("ignores inactive boards", func(t *testing.T) {
		boardShortName, _ := setupBoardAndThread(t)

		// Make board inactive
		setBoardInactive(t, boardShortName)

		// Verify board is inactive
		activeBoards, err := storage.GetActiveBoards(time.Hour)
		require.NoError(t, err)

		found := false
		for _, board := range activeBoards {
			if board.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.False(t, found, "Inactive board should not be in active boards list")
	})

	t.Run("refreshes active boards automatically", func(t *testing.T) {
		boardShortName, threadID := setupBoardAndThread(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		testInterval := 100 * time.Millisecond
		storage.StartPeriodicViewRefresh(ctx, testInterval)

		// Add a message after starting periodic refresh
		time.Sleep(testInterval)
		createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: 1},
			Text:     "Periodic Message",
			ThreadId: threadID,
		})

		// Wait for at least one refresh cycle
		time.Sleep(testInterval * 2)

		// Verify message appears
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		require.Len(t, board.Threads, 1)

		found := false
		for _, msg := range board.Threads[0].Messages {
			if msg.Text == "Periodic Message" {
				found = true
				break
			}
		}
		assert.True(t, found, "Message should appear after periodic refresh")
	})
}

// TestConcurrentRefresh tests concurrent access to refresh functionality.
func TestConcurrentRefresh(t *testing.T) {
	t.Run("handles concurrent refresh calls", func(t *testing.T) {
		boardShortName, threadID := setupBoardAndThread(t)

		// Add some initial messages
		createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: 1},
			Text:     "Initial Message",
			ThreadId: threadID,
		})

		var wg sync.WaitGroup
		numConcurrent := 3

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				refreshBoardView(t, boardShortName)
			}()
		}

		wg.Wait()

		// Verify board remains accessible after concurrent refreshes
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		assert.NotEmpty(t, board.Threads)
		assert.GreaterOrEqual(t, len(board.Threads[0].Messages), 2)
	})
}
