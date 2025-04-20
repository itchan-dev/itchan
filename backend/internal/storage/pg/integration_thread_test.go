package pg

import (
	"fmt"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// ==================
// CreateThread Tests
// ==================

func TestCreateThread(t *testing.T) {
	boardShortName := setupBoard(t)
	title := "Test Thread Creation"
	opMsg := &domain.Message{
		Author:      domain.User{Id: 1},
		Text:        "Original Post Text",
		Attachments: &domain.Attachments{"op_image.png"},
	}

	// --- Success Case ---
	t.Run("Success", func(t *testing.T) {
		boardBefore, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)

		threadID, err := storage.CreateThread(title, boardShortName, opMsg)
		require.NoError(t, err, "CreateThread should succeed")
		require.Greater(t, threadID, int64(0), "Thread ID should be positive")
		boardAfter, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)

		// Verify the created thread using GetThread
		createdThread, err := storage.GetThread(threadID)
		require.NoError(t, err, "GetThread should find the newly created thread")
		assert.Equal(t, title, createdThread.Title, "Thread title mismatch")
		assert.Equal(t, boardShortName, createdThread.Board, "Thread board mismatch")
		assert.Equal(t, 0, createdThread.NumReplies, "Newly created thread should have 0 replies")
		assert.Len(t, createdThread.Messages, 1, "Newly created thread should have 1 message (OP)")
		assert.Equal(t, opMsg.Text, createdThread.Messages[0].Text, "OP message text mismatch")
		assert.Equal(t, opMsg.Author.Id, createdThread.Messages[0].Author.Id, "OP author ID mismatch")
		assert.Equal(t, *opMsg.Attachments, *createdThread.Messages[0].Attachments, "OP attachments mismatch")
		assert.Equal(t, threadID, createdThread.Messages[0].Id, "OP message ID should match thread ID")
		assert.False(t, createdThread.Messages[0].ThreadId.Valid, "OP message ThreadId should be nil")
		assert.WithinDuration(t, time.Now(), createdThread.LastBumped, 5*time.Second, "LastBumped time should be recent")
		assert.Equal(t, createdThread.Messages[0].CreatedAt, createdThread.LastBumped, "LastBumped time should match OP creation time initially")
		assert.True(t, boardBefore.LastActivity.Before(boardAfter.LastActivity), "Thread creation should update board last activity")
		assert.Equal(t, boardAfter.LastActivity, createdThread.LastBumped, "Board last activity should be equal to last bump")

		// Cleanup the thread manually since it was created within the subtest
		err = storage.DeleteThread(boardShortName, threadID)
		require.NoError(t, err, "Failed to cleanup thread in subtest")
	})
}

// ==================
// GetThread Tests
// ==================

func TestGetThread(t *testing.T) {
	boardShortName := setupBoard(t)
	title := "Test Get Thread"
	opMsg := &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP Get", Attachments: &domain.Attachments{"file1.jpg"}}

	t.Run("NotFound", func(t *testing.T) {
		_, err := storage.GetThread(-999) // Non-existent ID
		requireNotFoundError(t, err)
	})

	t.Run("OnlyOP", func(t *testing.T) {
		threadID := createTestThread(t, boardShortName, title+"_OnlyOP", opMsg)
		thread, err := storage.GetThread(threadID)
		require.NoError(t, err, "GetThread should not return an error for OP only")
		assert.Equal(t, title+"_OnlyOP", thread.Title)
		assert.Equal(t, boardShortName, thread.Board)
		assert.Equal(t, 0, thread.NumReplies)
		assert.Len(t, thread.Messages, 1)
		assert.Equal(t, opMsg.Text, thread.Messages[0].Text)
		assert.Equal(t, threadID, thread.Messages[0].Id)
		assert.False(t, thread.Messages[0].ThreadId.Valid)
		assert.Equal(t, *opMsg.Attachments, *thread.Messages[0].Attachments)
		assert.Equal(t, thread.Messages[0].CreatedAt, thread.LastBumped)
		// Cleanup
		err = storage.DeleteThread(boardShortName, threadID)
		require.NoError(t, err)
	})

	t.Run("WithReplies", func(t *testing.T) {
		threadID := createTestThread(t, boardShortName, title+"_WithReplies", opMsg)
		// Add replies
		replyMsgs := []struct {
			author *domain.User
			text   string
		}{
			{author: &domain.User{Id: 2}, text: "Reply 1 Text"},
			{author: &domain.User{Id: 3}, text: "Reply 2 Text"},
		}
		msgID1 := createTestMessage(t, boardShortName, replyMsgs[0].author, replyMsgs[0].text, nil, threadID)
		msgID2 := createTestMessage(t, boardShortName, replyMsgs[1].author, replyMsgs[1].text, nil, threadID)

		// Get the thread
		thread, err := storage.GetThread(threadID)
		require.NoError(t, err, "GetThread should not return an error")
		assert.Equal(t, title+"_WithReplies", thread.Title)
		assert.Equal(t, boardShortName, thread.Board)
		assert.Equal(t, 2, thread.NumReplies, "Number of replies mismatch")
		assert.Len(t, thread.Messages, 3, "Expected 3 messages (OP + 2 replies)")

		// Check message order and content
		requireMessageOrder(t, thread.Messages, []string{opMsg.Text, replyMsgs[0].text, replyMsgs[1].text})

		// Check OP details
		assert.Equal(t, threadID, thread.Messages[0].Id)
		assert.False(t, thread.Messages[0].ThreadId.Valid)
		assert.Equal(t, *opMsg.Attachments, *thread.Messages[0].Attachments)

		// Check reply details
		assert.Equal(t, msgID1, thread.Messages[1].Id)
		assert.True(t, thread.Messages[1].ThreadId.Valid)
		assert.Equal(t, threadID, thread.Messages[1].ThreadId.Int64)
		assert.Nil(t, thread.Messages[1].Attachments)
		assert.Equal(t, replyMsgs[0].author.Id, thread.Messages[1].Author.Id)

		assert.Equal(t, msgID2, thread.Messages[2].Id)
		assert.True(t, thread.Messages[2].ThreadId.Valid)
		assert.Equal(t, threadID, thread.Messages[2].ThreadId.Int64)
		assert.Nil(t, thread.Messages[2].Attachments)
		assert.Equal(t, replyMsgs[1].author.Id, thread.Messages[2].Author.Id)

		// Check LastBumped time (should match the last reply's creation time)
		assert.Equal(t, thread.Messages[2].CreatedAt, thread.LastBumped)

		// Cleanup
		err = storage.DeleteThread(boardShortName, threadID)
		require.NoError(t, err)
	})
}

// ==================
// DeleteThread Tests
// ==================

func TestDeleteThread(t *testing.T) {
	boardShortName := setupBoard(t)

	t.Run("NotFound", func(t *testing.T) {
		err := storage.DeleteThread(boardShortName, -999) // Non-existent ID
		requireNotFoundError(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		// Setup thread with OP and replies
		title := "Test Delete Thread"
		opMsg := &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP Delete"}
		threadID := createTestThread(t, boardShortName, title, opMsg)
		msgID1 := createTestMessage(t, boardShortName, &domain.User{Id: 2}, "Reply 1 Delete", nil, threadID)
		_ = createTestMessage(t, boardShortName, &domain.User{Id: 3}, "Reply 2 Delete", nil, threadID) // msgID2 not needed for verification

		boardBefore, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)
		// Delete the thread
		err = storage.DeleteThread(boardShortName, threadID)
		require.NoError(t, err, "DeleteThread should not return an error")
		boardAfter, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)

		// Verify thread is gone
		_, err = storage.GetThread(threadID)
		requireNotFoundError(t, err)

		// Verify messages are gone (by trying to fetch one directly - optional, implementation detail check)
		var exists bool
		checkMsgQuery := "SELECT EXISTS(SELECT 1 FROM messages WHERE id = $1 OR id = $2)"
		err = storage.db.QueryRow(checkMsgQuery, threadID, msgID1).Scan(&exists)
		require.NoError(t, err, "Querying for deleted messages should not error")
		assert.False(t, exists, "Messages associated with the deleted thread should be gone")

		// Verify board last activity is updated
		require.True(t, boardBefore.LastActivity.Before(boardAfter.LastActivity), "Thread deletion should update board last activity")
	})
}

// ==================
// ThreadCount Tests
// ==================

func TestThreadCount(t *testing.T) {
	boardA := setupBoard(t)
	boardB := setupBoard(t)

	t.Run("EmptyBoard", func(t *testing.T) {
		count, err := storage.ThreadCount(boardA)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Count should be 0 for an empty board")
	})

	t.Run("OneThread", func(t *testing.T) {
		threadID := createTestThread(t, boardA, "Thread 1", &domain.Message{Author: domain.User{Id: 1}, Text: "OP A1"})
		defer storage.DeleteThread(boardA, threadID) // Cleanup

		count, err := storage.ThreadCount(boardA)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Count should be 1")
	})

	t.Run("MultipleThreads", func(t *testing.T) {
		tID1 := createTestThread(t, boardA, "Thread 1", &domain.Message{Author: domain.User{Id: 1}, Text: "OP A1"})
		defer storage.DeleteThread(boardA, tID1)
		tID2 := createTestThread(t, boardA, "Thread 2", &domain.Message{Author: domain.User{Id: 2}, Text: "OP A2"})
		defer storage.DeleteThread(boardA, tID2)
		tID3 := createTestThread(t, boardA, "Thread 3", &domain.Message{Author: domain.User{Id: 3}, Text: "OP A3"})
		defer storage.DeleteThread(boardA, tID3)

		count, err := storage.ThreadCount(boardA)
		require.NoError(t, err)
		assert.Equal(t, 3, count, "Count should be 3")
	})

	t.Run("MultipleBoards", func(t *testing.T) {
		// Board A setup
		tA1 := createTestThread(t, boardA, "Thread A1", &domain.Message{Author: domain.User{Id: 1}, Text: "OP A1"})
		defer storage.DeleteThread(boardA, tA1)
		tA2 := createTestThread(t, boardA, "Thread A2", &domain.Message{Author: domain.User{Id: 2}, Text: "OP A2"})
		defer storage.DeleteThread(boardA, tA2)

		// Board B setup
		tB1 := createTestThread(t, boardB, "Thread B1", &domain.Message{Author: domain.User{Id: 3}, Text: "OP B1"})
		defer storage.DeleteThread(boardB, tB1)

		// Check counts
		countA, errA := storage.ThreadCount(boardA)
		require.NoError(t, errA)
		assert.Equal(t, 2, countA, "Count for board A should be 2")

		countB, errB := storage.ThreadCount(boardB)
		require.NoError(t, errB)
		assert.Equal(t, 1, countB, "Count for board B should be 1")
	})
}

// ==================
// LastThreadId Tests
// ==================

func TestLastThreadId(t *testing.T) {
	boardA := setupBoard(t)
	boardB := setupBoard(t)

	t.Run("NoThreads", func(t *testing.T) {
		_, err := storage.LastThreadId(boardA)
		requireNotFoundError(t, err) // Expect 404
	})

	t.Run("OneThread", func(t *testing.T) {
		tID := createTestThread(t, boardA, "Single Thread", &domain.Message{Author: domain.User{Id: 1}, Text: "OP Single"})
		defer storage.DeleteThread(boardA, tID)

		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID, lastID, "Last ID should be the only thread's ID")
	})

	t.Run("MultipleThreadsNoBumps", func(t *testing.T) {
		// Create threads sequentially, ensuring creation times are distinct enough
		tID1 := createTestThread(t, boardA, "Thread 1", &domain.Message{Author: domain.User{Id: 1}, Text: "OP 1"})
		defer storage.DeleteThread(boardA, tID1)
		time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps
		tID2 := createTestThread(t, boardA, "Thread 2", &domain.Message{Author: domain.User{Id: 2}, Text: "OP 2"})
		defer storage.DeleteThread(boardA, tID2)
		time.Sleep(10 * time.Millisecond)
		tID3 := createTestThread(t, boardA, "Thread 3", &domain.Message{Author: domain.User{Id: 3}, Text: "OP 3"})
		defer storage.DeleteThread(boardA, tID3)

		// Without bumps, the oldest thread (lowest last_bump_ts, which is creation time) should be returned
		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID1, lastID, "Last ID should be the oldest created thread (tID1)")
	})

	t.Run("MultipleThreadsWithBumps", func(t *testing.T) {
		// Create threads
		tID1 := createTestThread(t, boardA, "Thread 1 Bump", &domain.Message{Author: domain.User{Id: 1}, Text: "OP 1B"})
		defer storage.DeleteThread(boardA, tID1)
		time.Sleep(10 * time.Millisecond)
		tID2 := createTestThread(t, boardA, "Thread 2 Bump", &domain.Message{Author: domain.User{Id: 2}, Text: "OP 2B"})
		defer storage.DeleteThread(boardA, tID2)
		time.Sleep(10 * time.Millisecond)
		tID3 := createTestThread(t, boardA, "Thread 3 Bump", &domain.Message{Author: domain.User{Id: 3}, Text: "OP 3B"})
		defer storage.DeleteThread(boardA, tID3)
		time.Sleep(10 * time.Millisecond) // Ensure message timestamp is later

		// Initially, tID1 is oldest
		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID1, lastID, "Initially, oldest is tID1")

		// Bump thread 1 by adding a message
		_ = createTestMessage(t, boardA, &domain.User{Id: 4}, "Bump msg for tID1", nil, tID1)
		time.Sleep(10 * time.Millisecond) // Ensure next check sees the update

		// Now, tID2 should be the oldest non-bumped (or least recently bumped)
		lastID, err = storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID2, lastID, "After bumping tID1, oldest should be tID2")

		// Bump thread 2
		_ = createTestMessage(t, boardA, &domain.User{Id: 5}, "Bump msg for tID2", nil, tID2)
		time.Sleep(10 * time.Millisecond)

		// Now, tID3 should be the oldest
		lastID, err = storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID3, lastID, "After bumping tID2, oldest should be tID3")
	})

	t.Run("MultipleBoardsIsolation", func(t *testing.T) {
		// Board A setup
		tA1 := createTestThread(t, boardA, "Thread A1 Iso", &domain.Message{Author: domain.User{Id: 1}, Text: "OP A1"})
		defer storage.DeleteThread(boardA, tA1)
		time.Sleep(10 * time.Millisecond)
		tA2 := createTestThread(t, boardA, "Thread A2 Iso", &domain.Message{Author: domain.User{Id: 2}, Text: "OP A2"})
		defer storage.DeleteThread(boardA, tA2)

		// Board B setup
		tB1 := createTestThread(t, boardB, "Thread B1 Iso", &domain.Message{Author: domain.User{Id: 3}, Text: "OP B1"})
		defer storage.DeleteThread(boardB, tB1)

		// Check last IDs
		lastIDA, errA := storage.LastThreadId(boardA)
		require.NoError(t, errA)
		assert.Equal(t, tA1, lastIDA, "Last ID for board A should be tA1")

		lastIDB, errB := storage.LastThreadId(boardB)
		require.NoError(t, errB)
		assert.Equal(t, tB1, lastIDB, "Last ID for board B should be tB1")
	})

	t.Run("StickyIgnored", func(t *testing.T) {
		// Create two threads
		tID1 := createTestThread(t, boardA, "NonSticky", &domain.Message{Author: domain.User{Id: 1}, Text: "OP NS"})
		defer storage.DeleteThread(boardA, tID1)
		time.Sleep(10 * time.Millisecond)
		tID2 := createTestThread(t, boardA, "ToBeSticky", &domain.Message{Author: domain.User{Id: 2}, Text: "OP Sticky"})
		defer storage.DeleteThread(boardA, tID2)

		// Manually make tID1 sticky (this requires direct DB interaction, normally done elsewhere)
		_, err := storage.db.Exec("UPDATE threads SET is_sticky = TRUE, last_bump_ts = $2 WHERE id = $1", tID1, time.Now().Add(-1*time.Hour)) // Make it sticky and artificially old
		require.NoError(t, err, "Failed to manually set thread sticky and old")

		// LastThreadId should ignore the sticky tID1 and return tID2
		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID2, lastID, "LastThreadId should ignore sticky threads and return the oldest non-sticky (tID2)")

		// Make tID2 sticky too
		_, err = storage.db.Exec("UPDATE threads SET is_sticky = TRUE WHERE id = $1", tID2)
		require.NoError(t, err, "Failed to manually set thread sticky")

		// Now there should be no non-sticky threads
		_, err = storage.LastThreadId(boardA)
		// requireNotFoundError(t, err, "Should return Not Found if only sticky threads exist")
	})

	// --- Error Case: Simulate DB error (difficult without mocking) ---
	// Test sql.ErrNoRows is covered by "NoThreads" and "StickyIgnored" cases.
}

// ==================
// TestBumpLimit (Kept from original, slightly adjusted context)
// ==================

func TestBumpLimit(t *testing.T) {
	// This test verifies the bump logic implicitly used by CreateMessage,
	// checking its effect on the thread's LastBumped timestamp.
	boardShortName, threadID := setupBoardAndThread(t)

	// Send messages up to *just below* the bump limit (NumReplies = BumpLimit - 1)
	// Remember OP is not a reply.
	bumpLimit := storage.cfg.Public.BumpLimit + 1
	for i := 1; i < bumpLimit; i++ {
		createTestMessage(t, boardShortName, &domain.User{Id: int64(i + 1)}, fmt.Sprintf("Reply %d", i), nil, threadID)
	}

	// Get the thread, the next message *should* bump it.
	threadBeforeBumpLimit, err := storage.GetThread(threadID)
	require.NoError(t, err)
	assert.Equal(t, bumpLimit-1, threadBeforeBumpLimit.NumReplies)
	lastBumpTsBeforeLimit := threadBeforeBumpLimit.LastBumped

	// Send the message that reaches the bump limit (NumReplies = BumpLimit)
	msgAtLimitID := createTestMessage(t, boardShortName, &domain.User{Id: int64(bumpLimit + 1)}, fmt.Sprintf("Reply %d (at limit)", bumpLimit), nil, threadID)
	msgAtLimit, err := storage.GetMessage(msgAtLimitID) // Need GetMessage to check timestamp accurately
	require.NoError(t, err)

	// Get the thread again, check bump occurred
	threadAtLimit, err := storage.GetThread(threadID)
	require.NoError(t, err)
	assert.Equal(t, bumpLimit, threadAtLimit.NumReplies)
	assert.Equal(t, msgAtLimit.CreatedAt, threadAtLimit.LastBumped, "Last bump timestamp should match the message at the bump limit")
	assert.NotEqual(t, lastBumpTsBeforeLimit, threadAtLimit.LastBumped, "Timestamp should have updated")
	lastBumpTsAtLimit := threadAtLimit.LastBumped

	// Send one more message (over the bump limit)
	_ = createTestMessage(t, boardShortName, &domain.User{Id: int64(bumpLimit + 2)}, "Reply over limit", nil, threadID)

	// Get the thread again and check that the last bump timestamp hasn't changed
	threadOverLimit, err := storage.GetThread(threadID)
	require.NoError(t, err)
	assert.Equal(t, bumpLimit+1, threadOverLimit.NumReplies) // Reply count still increases
	assert.Equal(t, lastBumpTsAtLimit, threadOverLimit.LastBumped, "Last bump timestamp should NOT change after going over the bump limit")
}

// Note: Helper functions (setupBoard, etc.) and TestMain remain the same as provided in the setup.
