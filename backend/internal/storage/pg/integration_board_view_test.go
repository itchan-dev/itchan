package pg

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/require"
)

func TestRefreshMaterializedViewConcurrentUpdatesView(t *testing.T) {
	boardShortName := setupBoard(t)
	user := &domain.User{Id: 1}
	opMsg := &domain.Message{Author: *user, Text: "OP"}
	threadID := createTestThread(t, boardShortName, "Test Thread", opMsg)

	// Add 3 replies
	for i := 1; i <= 3; i++ {
		createTestMessage(t, boardShortName, user, fmt.Sprintf("Reply %d", i), nil, threadID)
	}

	// Initial refresh to populate the view
	err := storage.refreshMaterializedViewConcurrent(boardShortName, time.Second)
	require.NoError(t, err)

	// Verify initial messages in the view
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	thread := board.Threads[0]
	require.Len(t, thread.Messages, 4) // OP + 3 replies
	require.Equal(t, "OP", thread.Messages[0].Text)
	require.Equal(t, "Reply 1", thread.Messages[1].Text)
	require.Equal(t, "Reply 2", thread.Messages[2].Text)
	require.Equal(t, "Reply 3", thread.Messages[3].Text)

	// Add 4th reply
	createTestMessage(t, boardShortName, user, "Reply 4", nil, threadID)

	// Verify view hasn't updated yet
	board, err = storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	thread = board.Threads[0]
	require.Len(t, thread.Messages, 4) // Still OP + 3 replies

	// Refresh the view
	err = storage.refreshMaterializedViewConcurrent(boardShortName, time.Second)
	require.NoError(t, err)

	// Verify updated messages
	board, err = storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	thread = board.Threads[0]
	require.Len(t, thread.Messages, 4)
	require.Equal(t, "OP", thread.Messages[0].Text)
	require.Equal(t, "Reply 2", thread.Messages[1].Text)
	require.Equal(t, "Reply 3", thread.Messages[2].Text)
	require.Equal(t, "Reply 4", thread.Messages[3].Text)
}

func TestRefreshMaterializedViewConcurrentFailsForNonexistentBoard(t *testing.T) {
	err := storage.refreshMaterializedViewConcurrent("nonexistent_board", time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "concurrent refresh failed")
}

func TestStartPeriodicViewRefreshRefreshesActiveBoards(t *testing.T) {
	// Setup a new context for this test to control the periodic refresh
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Shorten the refresh interval for testing
	originalInterval := storage.cfg.Public.BoardPreviewRefreshInterval
	storage.cfg.Public.BoardPreviewRefreshInterval = 500 * time.Millisecond
	defer func() { storage.cfg.Public.BoardPreviewRefreshInterval = originalInterval }()

	// Start the periodic refresh
	storage.StartPeriodicViewRefresh(ctx, storage.cfg.Public.BoardPreviewRefreshInterval)

	// Setup board and thread with a recent message
	boardShortName, threadID := setupBoardAndThread(t)
	user := &domain.User{Id: 1}
	createTestMessage(t, boardShortName, user, "Reply 1", nil, threadID)

	// Ensure initial refresh to populate the view
	err := storage.refreshMaterializedViewConcurrent(boardShortName, time.Second)
	require.NoError(t, err)

	// Add a new message to make the board active
	createTestMessage(t, boardShortName, user, "Reply 2", nil, threadID)

	// Wait for the periodic refresh to trigger
	time.Sleep(2000 * time.Millisecond)

	// Verify the view was refreshed with the new message
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	thread := board.Threads[0]
	found := false
	for _, msg := range thread.Messages {
		if msg.Text == "Reply 2" {
			found = true
			break
		}
	}
	require.True(t, found, "Reply 2 should be present in the refreshed view")
}
