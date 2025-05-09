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

// TestRefreshMaterializedViewConcurrentUpdatesView verifies that refreshing the view
// correctly updates the messages shown according to NLastMsg configuration.
func TestRefreshMaterializedViewConcurrentUpdatesView(t *testing.T) {
	boardShortName := setupBoard(t)
	user := domain.User{Id: 1}
	opMsg := domain.MessageCreationData{Board: boardShortName, Author: user, Text: "OP"}
	threadID := createTestThread(t, domain.ThreadCreationData{Title: "Test Refresh Thread", Board: boardShortName, OpMessage: opMsg})

	// Add initial replies (NLastMsg = 3, so add 3)
	replies := make([]string, 3)
	for i := 1; i <= storage.cfg.Public.NLastMsg; i++ {
		replies[i-1] = fmt.Sprintf("Reply %d", i)
		msg := domain.MessageCreationData{Board: boardShortName, Author: user, Text: replies[i-1], ThreadId: threadID}
		createTestMessage(t, msg)
	}

	// Initial refresh to populate the view
	// Use a reasonable timeout for the refresh operation itself
	refreshTimeout := 2 * time.Second
	err := storage.refreshMaterializedViewConcurrent(boardShortName, refreshTimeout)
	require.NoError(t, err)

	// Verify initial messages in the view (OP + 3 replies)
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1, "Should have 1 thread")
	thread := board.Threads[0]
	// Expect OP + NLastMsg (3) = 4 messages max initially
	require.Len(t, thread.Messages, 4, "Initial view should contain OP + 3 replies")
	require.Equal(t, "OP", thread.Messages[0].Text)
	requireMessageOrder(t, thread.Messages[1:], replies) // Check only replies

	// Add a 4th reply, which should eventually replace the oldest reply (Reply 1)
	newReplyText := "Reply 4"
	createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: user, Text: newReplyText, ThreadId: threadID})

	// Verify view hasn't updated yet
	board, err = storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	thread = board.Threads[0]
	require.Len(t, thread.Messages, 4, "View should not have updated yet")
	// Check that the new reply is NOT yet visible
	foundNewReply := false
	for _, msg := range thread.Messages {
		if msg.Text == newReplyText {
			foundNewReply = true
			break
		}
	}
	require.False(t, foundNewReply, "Reply 4 should not be in the view before refresh")

	// Refresh the view again
	err = storage.refreshMaterializedViewConcurrent(boardShortName, refreshTimeout)
	require.NoError(t, err)

	// Verify updated messages (OP + last 3 replies: Reply 2, Reply 3, Reply 4)
	board, err = storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	thread = board.Threads[0]
	// Still expect OP + NLastMsg (3) = 4 messages max
	require.Len(t, thread.Messages, 4, "Updated view should contain OP + last 3 replies")
	require.Equal(t, "OP", thread.Messages[0].Text)
	// Expected replies: Reply 2, Reply 3, Reply 4
	expectedRepliesAfterUpdate := []string{"Reply 2", "Reply 3", "Reply 4"}
	requireMessageOrder(t, thread.Messages[1:], expectedRepliesAfterUpdate)
}

// TestRefreshMaterializedViewConcurrentFailsForNonexistentBoard verifies that
// attempting to refresh a view for a board that doesn't exist returns an error.
func TestRefreshMaterializedViewConcurrentFailsForNonexistentBoard(t *testing.T) {
	nonExistentBoard := generateString(t)
	// Use a short timeout as it should fail quickly
	err := storage.refreshMaterializedViewConcurrent(nonExistentBoard, 500*time.Millisecond)
	require.Error(t, err)
	// The error comes from the DB driver when the view doesn't exist
	require.Contains(t, err.Error(), "concurrent refresh failed", "Error should indicate refresh failure")
	require.Contains(t, err.Error(), "does not exist", "Error message should mention the view/relation doesn't exist")
}

// TestStartPeriodicViewRefreshRefreshesActiveBoards verifies that the periodic refresh
// mechanism updates the view for boards that have had recent activity.
func TestStartPeriodicViewRefreshRefreshesActiveBoards(t *testing.T) {
	// Setup a new context for this test to control the periodic refresh lifetime
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Shorten the refresh interval for faster testing
	originalInterval := storage.cfg.Public.BoardPreviewRefreshInterval
	testInterval := 500 * time.Millisecond
	storage.cfg.Public.BoardPreviewRefreshInterval = testInterval
	defer func() { storage.cfg.Public.BoardPreviewRefreshInterval = originalInterval }()

	// Setup board and thread with an initial message
	boardShortName, threadID := setupBoardAndThread(t) // uses "Test OP"
	user := domain.User{Id: 1}
	initialReply := "Reply 1"
	createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: user, Text: initialReply, ThreadId: threadID})

	// Ensure initial refresh to populate the view before starting periodic checks
	err := storage.refreshMaterializedViewConcurrent(boardShortName, 2*time.Second)
	require.NoError(t, err)

	// Verify initial state
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	require.Len(t, board.Threads[0].Messages, 2) // OP + Reply 1
	require.Equal(t, initialReply, board.Threads[0].Messages[1].Text)

	// Start the periodic refresh *after* initial setup and refresh
	storage.StartPeriodicViewRefresh(ctx, testInterval)

	// Add a new message to make the board "active" within the refresh interval
	time.Sleep(testInterval / 2) // Ensure the new message time is clearly after the start
	secondReply := "Reply 2"
	createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: user, Text: secondReply, ThreadId: threadID})

	// Wait long enough for at least one, likely several, refresh cycles to occur
	// Wait slightly longer than 2 intervals to increase certainty.
	time.Sleep(testInterval*2 + 100*time.Millisecond)

	// Verify the view was refreshed with the new message by the periodic task
	board, err = storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	thread := board.Threads[0]
	// Expect OP + Reply 1 + Reply 2 (since NLastMsg=3)
	require.Len(t, thread.Messages, 3, "View should now contain OP + 2 replies")

	found := false
	for _, msg := range thread.Messages {
		if msg.Text == secondReply {
			found = true
			break
		}
	}
	require.True(t, found, "%q should be present in the refreshed view", secondReply)
	require.Equal(t, "Test OP", thread.Messages[0].Text)
	require.Equal(t, initialReply, thread.Messages[1].Text)
	require.Equal(t, secondReply, thread.Messages[2].Text)
}

// TestStartPeriodicViewRefreshIgnoresInactiveBoards verifies that boards without
// recent activity (relative to the interval) are not refreshed unnecessarily.
func TestStartPeriodicViewRefreshIgnoresInactiveBoards(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testInterval := 100 * time.Millisecond
	originalInterval := storage.cfg.Public.BoardPreviewRefreshInterval
	storage.cfg.Public.BoardPreviewRefreshInterval = testInterval
	defer func() { storage.cfg.Public.BoardPreviewRefreshInterval = originalInterval }()

	// Setup two boards
	boardActive := setupBoard(t)
	threadActiveID := createTestThread(t, domain.ThreadCreationData{Title: "Active Thread", Board: boardActive, OpMessage: domain.MessageCreationData{Text: "OP Active"}})
	boardInactive := setupBoard(t)
	threadInactiveID := createTestThread(t, domain.ThreadCreationData{Title: "Inactive Thread", Board: boardInactive, OpMessage: domain.MessageCreationData{Text: "OP Inactive"}})
	user := domain.User{Id: 5}

	// Initial refresh for both
	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardActive, 2*time.Second))
	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardInactive, 2*time.Second))

	// Make Inactive board 'old' by adding a message, then waiting longer than the interval
	createTestMessage(t, domain.MessageCreationData{Board: boardInactive, Author: user, Text: "Old Message", ThreadId: threadInactiveID})
	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardInactive, 2*time.Second)) // Refresh to capture "Old Message"
	time.Sleep(testInterval * 2)                                                                // Wait long enough for it to become inactive

	// Start periodic refresh
	storage.StartPeriodicViewRefresh(ctx, testInterval)

	// Now, add a message to the ACTIVE board
	time.Sleep(testInterval / 2) // Ensure activity timestamp is recent
	activeMsg := "Recent Active Message"
	createTestMessage(t, domain.MessageCreationData{Board: boardActive, Author: user, Text: activeMsg, ThreadId: threadActiveID})

	inactiveMsg := "Recent Inactive Message"
	createTestMessage(t, domain.MessageCreationData{Board: boardInactive, Author: user, Text: inactiveMsg, ThreadId: threadInactiveID})
	_, err := storage.db.Exec("update boards set last_activity = now() - interval '1d' where short_name = $1", boardInactive)
	require.NoError(t, err, "Failed to set old activity time")
	// Wait for refresh cycle(s)
	time.Sleep(testInterval*2 + 100*time.Millisecond)

	// Verify Active board WAS updated
	board, err := storage.GetBoard(boardActive, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	foundActive := false
	for _, msg := range board.Threads[0].Messages {
		if msg.Text == activeMsg {
			foundActive = true
			break
		}
	}
	assert.True(t, foundActive, "Active board should contain the recent message %q", activeMsg)

	// Verify Inactive board WAS NOT updated with its most recent message
	board, err = storage.GetBoard(boardInactive, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	foundInactiveRecent := false
	foundInactiveOld := false
	for _, msg := range board.Threads[0].Messages {
		if msg.Text == inactiveMsg {
			foundInactiveRecent = true
		}
		if msg.Text == "Old Message" {
			foundInactiveOld = true
		}
	}
	assert.True(t, foundInactiveOld, "Inactive board should still contain 'Old Message'")
	assert.False(t, foundInactiveRecent, "Inactive board should NOT contain the recent message %q as it wasn't refreshed", inactiveMsg)
}

// TestStartPeriodicViewRefreshHandlesMultipleActiveBoards verifies that if multiple
// boards become active, the periodic refresh updates all of them.
func TestStartPeriodicViewRefreshHandlesMultipleActiveBoards(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testInterval := 300 * time.Millisecond // Slightly longer to avoid flaky tests
	originalInterval := storage.cfg.Public.BoardPreviewRefreshInterval
	storage.cfg.Public.BoardPreviewRefreshInterval = testInterval
	defer func() { storage.cfg.Public.BoardPreviewRefreshInterval = originalInterval }()

	// Setup boards
	boardA := setupBoard(t)
	threadA := createTestThread(t, domain.ThreadCreationData{Board: boardA, Title: "Thread A", OpMessage: domain.MessageCreationData{Text: "OP A"}})
	boardB := setupBoard(t)
	threadB := createTestThread(t, domain.ThreadCreationData{Board: boardB, Title: "Thread B", OpMessage: domain.MessageCreationData{Text: "OP B"}})
	user := domain.User{Id: 6}

	// Initial refresh
	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardA, 2*time.Second))
	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardB, 2*time.Second))

	// Start periodic refresh
	storage.StartPeriodicViewRefresh(ctx, testInterval)

	// Make both boards active
	time.Sleep(testInterval / 3)
	msgA := "New Msg A"
	createTestMessage(t, domain.MessageCreationData{Board: boardA, Author: user, Text: msgA, ThreadId: threadA})
	time.Sleep(testInterval / 3) // Stagger activity slightly
	msgB := "New Msg B"
	createTestMessage(t, domain.MessageCreationData{Board: boardB, Author: user, Text: msgB, ThreadId: threadB})

	// Wait for refresh cycle(s)
	time.Sleep(testInterval*2 + 100*time.Millisecond)

	// Verify Board A was updated
	board, err := storage.GetBoard(boardA, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	foundA := false
	for _, m := range board.Threads[0].Messages {
		if m.Text == msgA {
			foundA = true
			break
		}
	}
	assert.True(t, foundA, "Board A should contain its new message %q", msgA)

	// Verify Board B was updated
	board, err = storage.GetBoard(boardB, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	foundB := false
	for _, m := range board.Threads[0].Messages {
		if m.Text == msgB {
			foundB = true
			break
		}
	}
	assert.True(t, foundB, "Board B should contain its new message %q", msgB)
}

// TestStartPeriodicViewRefreshContextCancelStopsRefresh verifies that cancelling
// the context stops the periodic refresh goroutine.
func TestStartPeriodicViewRefreshContextCancelStopsRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// Don't defer cancel() immediately, we need to call it during the test

	testInterval := 300 * time.Millisecond // Short interval
	originalInterval := storage.cfg.Public.BoardPreviewRefreshInterval
	storage.cfg.Public.BoardPreviewRefreshInterval = testInterval
	defer func() { storage.cfg.Public.BoardPreviewRefreshInterval = originalInterval }()

	boardShortName, threadID := setupBoardAndThread(t) // OP: "Test OP"
	user := domain.User{Id: 7}

	// Initial refresh
	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, 2*time.Second))

	// Start periodic refresh
	storage.StartPeriodicViewRefresh(ctx, testInterval)

	// Wait for a cycle just to be sure it started (optional but good practice)
	time.Sleep(testInterval + 50*time.Millisecond)

	// Cancel the context
	cancel()

	// Wait a moment to allow the goroutine to potentially exit
	time.Sleep(100 * time.Millisecond)

	// Add a new message *after* cancellation
	newMessage := "Message After Cancel"
	createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: user, Text: newMessage, ThreadId: threadID})

	// Wait for longer than several refresh intervals would have taken
	time.Sleep(testInterval*3 + 100*time.Millisecond)

	// Verify the view WAS NOT updated, because the refresh task should have stopped
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)
	require.Len(t, board.Threads, 1)
	found := false
	for _, msg := range board.Threads[0].Messages {
		if msg.Text == newMessage {
			found = true
			break
		}
	}
	require.False(t, found, "View should not contain message %q posted after context cancellation", newMessage)
	// Check that only the OP message is present
	require.Len(t, board.Threads[0].Messages, 1, "Only the initial OP message should be present")
	require.Equal(t, "Test OP", board.Threads[0].Messages[0].Text)
}

// TestPeriodicRefreshRaceCondition attempts to trigger race conditions by
// having multiple concurrent refreshes triggered by the periodic mechanism.
// This test relies on timing and might be flaky, but useful for basic checks.
func TestPeriodicRefreshRaceCondition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testInterval := 200 * time.Millisecond // Very short interval to increase concurrency chance
	originalInterval := storage.cfg.Public.BoardPreviewRefreshInterval
	storage.cfg.Public.BoardPreviewRefreshInterval = testInterval
	defer func() { storage.cfg.Public.BoardPreviewRefreshInterval = originalInterval }()

	numBoards := 5
	boardNames := make([]string, numBoards)
	threadIDs := make([]int64, numBoards)
	user := domain.User{Id: 8}

	// Setup boards and initial refresh
	for i := 0; i < numBoards; i++ {
		boardNames[i] = setupBoard(t)
		threadIDs[i] = createTestThread(t, domain.ThreadCreationData{Board: boardNames[i], Title: fmt.Sprintf("Race Board %d", i), OpMessage: domain.MessageCreationData{Text: "OP"}})
		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardNames[i], 2*time.Second))
	}

	// Start periodic refresh
	storage.StartPeriodicViewRefresh(ctx, testInterval)

	// Hammer the boards with messages concurrently to keep them active
	var wg sync.WaitGroup
	stopSignal := make(chan struct{})
	hammerDuration := time.Second * 2

	for i := 0; i < numBoards; i++ {
		wg.Add(1)
		go func(boardIdx int) {
			defer wg.Done()
			ticker := time.NewTicker(50 * time.Millisecond) // Post frequently
			defer ticker.Stop()
			msgCounter := 0
			for {
				select {
				case <-ticker.C:
					msgText := fmt.Sprintf("Msg %d Board %d", msgCounter, boardIdx)
					// Use createTestMessage which includes require.NoError
					createTestMessage(t, domain.MessageCreationData{Board: boardNames[boardIdx], Author: user, Text: msgText, ThreadId: threadIDs[boardIdx]})
					msgCounter++
				case <-stopSignal:
					return
				}
			}
		}(i)
	}

	// Let the hammering run alongside periodic refresh
	time.Sleep(hammerDuration)
	close(stopSignal) // Signal hammering goroutines to stop
	wg.Wait()         // Wait for hammering to finish

	// Wait a bit longer for final refreshes to potentially complete
	time.Sleep(testInterval*2 + 100*time.Millisecond)

	// Basic check: Verify no panics occurred and boards are accessible.
	// A more robust check would involve verifying the *final state* of messages,
	// but that's complex given the concurrent non-deterministic nature.
	// The main goal here is ensuring REFRESH CONCURRENTLY doesn't deadlock or panic.
	t.Logf("Checking board accessibility after concurrent refresh stress")
	for i := 0; i < numBoards; i++ {
		_, err := storage.GetBoard(boardNames[i], 1)
		assert.NoError(t, err, "Board %s should still be accessible after stress test", boardNames[i])
	}
	t.Logf("Concurrent refresh stress test completed without obvious failures.")
}
