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
	opMsg := domain.MessageCreationData{
		Board:       boardShortName,
		Author:      domain.User{Id: 1},
		Text:        "Original Post Text",
		Attachments: &domain.Attachments{"op_image.png"},
	}
	creationData := domain.ThreadCreationData{
		Title:     title,
		Board:     boardShortName,
		OpMessage: opMsg,
	}

	t.Run("Success", func(t *testing.T) {
		boardBefore, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		// Allow some time for potential clock skew or fast operations
		time.Sleep(50 * time.Millisecond)
		creationTimeStart := time.Now()

		threadID, err := storage.CreateThread(creationData)
		require.NoError(t, err, "CreateThread should succeed")
		require.Greater(t, threadID, int64(0), "Thread ID should be positive")

		boardAfter, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)

		// Verify the created thread using GetThread
		createdThread, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err, "GetThread should find the newly created thread")

		assert.Equal(t, title, createdThread.Title, "Thread title mismatch")
		assert.Equal(t, boardShortName, createdThread.Board, "Thread board mismatch")
		assert.Equal(t, 0, createdThread.NumReplies, "Newly created thread should have 0 replies")
		require.Len(t, createdThread.Messages, 1, "Newly created thread should have 1 message (OP)")

		// Verify OP Message details
		op := createdThread.Messages[0]
		assert.Equal(t, opMsg.Text, op.Text, "OP message text mismatch")
		assert.Equal(t, opMsg.Author.Id, op.Author.Id, "OP author ID mismatch")
		require.NotNil(t, op.Attachments, "OP attachments should not be nil")
		assert.Equal(t, *opMsg.Attachments, *op.Attachments, "OP attachments mismatch")
		assert.Equal(t, threadID, op.ThreadId, "OP message ThreadId should be equal to threadId")

		// Verify Timestamps
		assert.WithinDuration(t, creationTimeStart, createdThread.LastBumped, 5*time.Second, "LastBumped time should be recent")
		assert.Equal(t, op.CreatedAt, createdThread.LastBumped, "LastBumped time should match OP creation time initially")
		assert.True(t, boardBefore.LastActivity.Before(boardAfter.LastActivity), "Thread creation should update board last activity")
		// Board activity might be slightly different due to transaction commit time vs message creation time
		assert.WithinDuration(t, boardAfter.LastActivity, createdThread.LastBumped, 100*time.Millisecond, "Board last activity should be very close to last bump")

		// Cleanup the thread manually since it was created within the subtest
		err = storage.DeleteThread(boardShortName, threadID)
		require.NoError(t, err, "Failed to cleanup thread in subtest")
	})

	t.Run("BoardNotFound", func(t *testing.T) {
		opMsgNonexistingBoard := opMsg
		opMsgNonexistingBoard.Board = "nonexistentboard"
		invalidCreationData := domain.ThreadCreationData{
			Title:     "Test Invalid Board",
			Board:     "nonexistentboard",
			OpMessage: opMsgNonexistingBoard,
		}
		_, err := storage.CreateThread(invalidCreationData)
		requireNotFoundError(t, err) // Expect 404 as board does not exist
	})

	t.Run("Create Sticky Thread", func(t *testing.T) {
		creationData := domain.ThreadCreationData{
			Title:     "Sticky Thread",
			Board:     boardShortName,
			IsSticky:  true,
			OpMessage: opMsg,
		}
		threadID, err := storage.CreateThread(creationData)
		require.NoError(t, err)

		thread, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err)
		assert.True(t, thread.IsSticky, "Thread should be marked as sticky")
	})
}

// ==================
// GetThread Tests
// ==================

func TestGetThread(t *testing.T) {
	boardShortName := setupBoard(t)
	title := "Test Get Thread"
	opMsg := domain.MessageCreationData{
		Board:       boardShortName,
		Author:      domain.User{Id: 1},
		Text:        "Test OP Get",
		Attachments: &domain.Attachments{"file1.jpg"},
	}

	t.Run("NotFound", func(t *testing.T) {
		_, err := storage.GetThread(boardShortName, -999) // Non-existent ID
		requireNotFoundError(t, err)
	})

	t.Run("OnlyOP", func(t *testing.T) {
		// Use updated helper
		threadID := createTestThread(t, domain.ThreadCreationData{Title: title + "_OnlyOP", Board: boardShortName, OpMessage: opMsg})
		t.Cleanup(func() { require.NoError(t, storage.DeleteThread(boardShortName, threadID)) })

		thread, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err, "GetThread should not return an error for OP only")

		assert.Equal(t, title+"_OnlyOP", thread.Title)
		assert.Equal(t, boardShortName, thread.Board)
		assert.Equal(t, 0, thread.NumReplies)
		require.Len(t, thread.Messages, 1)

		op := thread.Messages[0]
		assert.Equal(t, opMsg.Text, op.Text)
		assert.Equal(t, threadID, threadID)
		require.NotNil(t, op.Attachments)
		assert.Equal(t, *opMsg.Attachments, *op.Attachments)
		assert.Equal(t, op.CreatedAt, thread.LastBumped)
	})

	t.Run("WithReplies", func(t *testing.T) {
		threadID := createTestThread(t, domain.ThreadCreationData{Title: title + "_WithReplies", Board: boardShortName, OpMessage: opMsg})
		t.Cleanup(func() { require.NoError(t, storage.DeleteThread(boardShortName, threadID)) })

		// Add replies
		replyMsgs := []struct {
			author domain.User
			text   string
		}{
			{author: domain.User{Id: 2}, text: "Reply 1 Text"},
			{author: domain.User{Id: 3}, text: "Reply 2 Text"},
		}
		msgID1 := createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: replyMsgs[0].author, Text: replyMsgs[0].text, ThreadId: threadID})
		time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps for last bump check
		msgID2 := createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: replyMsgs[1].author, Text: replyMsgs[1].text, ThreadId: threadID})

		// Get the thread
		thread, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err, "GetThread should not return an error")
		assert.Equal(t, title+"_WithReplies", thread.Title)
		assert.Equal(t, boardShortName, thread.Board)
		assert.Equal(t, 2, thread.NumReplies, "Number of replies mismatch")
		require.Len(t, thread.Messages, 3, "Expected 3 messages (OP + 2 replies)")

		// Check message order and content
		requireMessageOrder(t, thread.Messages, []string{opMsg.Text, replyMsgs[0].text, replyMsgs[1].text})

		// Check OP details
		op := thread.Messages[0]
		assert.Equal(t, threadID, op.ThreadId)
		require.NotNil(t, op.Attachments)
		assert.Equal(t, *opMsg.Attachments, *op.Attachments)

		// Check reply 1 details
		reply1 := thread.Messages[1]
		assert.Equal(t, msgID1, reply1.Id)
		require.NotNil(t, reply1.ThreadId)
		assert.Equal(t, threadID, reply1.ThreadId)
		assert.Nil(t, reply1.Attachments) // Attachments were nil when creating
		assert.Equal(t, replyMsgs[0].author.Id, reply1.Author.Id)

		// Check reply 2 details
		reply2 := thread.Messages[2]
		assert.Equal(t, msgID2, reply2.Id)
		require.NotNil(t, reply2.ThreadId)
		assert.Equal(t, threadID, reply2.ThreadId)
		assert.Nil(t, reply2.Attachments)
		assert.Equal(t, replyMsgs[1].author.Id, reply2.Author.Id)

		// Check LastBumped time (should match the last reply's creation time)
		// Assuming CreateMessage updates LastBumped correctly (verified in TestBumpLimit)
		assert.Equal(t, reply2.CreatedAt, thread.LastBumped)
	})
}

// ==================
// DeleteThread Tests
// ==================

func TestDeleteThread(t *testing.T) {
	boardShortName := setupBoard(t)
	opMsg := domain.MessageCreationData{
		Board:       boardShortName,
		Author:      domain.User{Id: 1},
		Text:        "Test OP Get",
		Attachments: &domain.Attachments{"file1.jpg"},
	}

	t.Run("NotFoundThread", func(t *testing.T) {
		err := storage.DeleteThread(boardShortName, -999) // Non-existent thread ID
		requireNotFoundError(t, err)
	})

	t.Run("NotFoundBoard", func(t *testing.T) {
		// Create a real thread first to ensure we test board not found, not thread not found
		threadID := createTestThread(t, domain.ThreadCreationData{Title: "Temp Thread", Board: boardShortName, OpMessage: opMsg})
		t.Cleanup(func() { _ = storage.DeleteThread(boardShortName, threadID) }) // Cleanup if test fails before deletion

		err := storage.DeleteThread("nonexistentboard", threadID) // Non-existent board
		requireNotFoundError(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		// Setup thread with OP and replies
		title := "Test Delete Thread"
		opMsgCopy := opMsg
		opMsgCopy.Text = "Test OP Delete"
		threadID := createTestThread(t, domain.ThreadCreationData{Title: title, Board: boardShortName, OpMessage: opMsgCopy})
		_ = createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 2}, Text: "Reply 1 Delete", ThreadId: threadID})
		_ = createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 3}, Text: "Reply 2 Delete", ThreadId: threadID})

		boardBefore, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)

		// Delete the thread
		err = storage.DeleteThread(boardShortName, threadID)
		require.NoError(t, err, "DeleteThread should not return an error")

		boardAfter, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)

		// Verify thread is gone
		_, err = storage.GetThread(boardShortName, threadID)
		requireNotFoundError(t, err)

		// Verify messages are gone (implicitly via cascade, checked by GetThread 404)
		// No need for direct DB query, GetThread failing is sufficient evidence

		// Verify board last activity is updated
		assert.True(t, boardBefore.LastActivity.Before(boardAfter.LastActivity), "Thread deletion should update board last activity")
		// Check it's recent
		assert.WithinDuration(t, time.Now(), boardAfter.LastActivity, 5*time.Second)
	})
}

// ==================
// TestBumpLimit
// ==================

func TestBumpLimit(t *testing.T) {
	// This test verifies the bump logic implicitly used by CreateMessage,
	// checking its effect on the thread's LastBumped timestamp and NumReplies.
	boardShortName, threadID := setupBoardAndThread(t) // Use updated helper
	t.Cleanup(func() { require.NoError(t, storage.DeleteThread(boardShortName, threadID)) })

	// Send messages up to *just below* the bump limit
	// NumReplies = BumpLimit - 1
	bumpLimit := storage.cfg.Public.BumpLimit
	require.Greater(t, bumpLimit, 0, "BumpLimit must be positive for this test")

	for i := 0; i < bumpLimit-1; i++ {
		createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: int64(i + 2)}, Text: fmt.Sprintf("Reply %d", i+1), ThreadId: threadID})
	}

	// Get the thread, the next message *should* bump it.
	threadBeforeLimit, err := storage.GetThread(boardShortName, threadID)
	require.NoError(t, err)
	require.Equal(t, bumpLimit-1, threadBeforeLimit.NumReplies)
	lastBumpTsBeforeLimit := threadBeforeLimit.LastBumped

	time.Sleep(10 * time.Millisecond) // Ensure next message has later timestamp

	// Send the message that reaches the bump limit (NumReplies = BumpLimit)
	msgAtLimitID := createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: int64(bumpLimit + 1)}, Text: fmt.Sprintf("Reply %d (at limit)", bumpLimit), ThreadId: threadID})
	time.Sleep(10 * time.Millisecond) // Allow time for potential DB updates

	msgAtLimit, err := storage.GetMessage(boardShortName, msgAtLimitID)
	require.NoError(t, err, "Failed to get message details for timestamp check")

	// Get the thread again, check bump occurred
	threadAtLimit, err := storage.GetThread(boardShortName, threadID)
	require.NoError(t, err)
	assert.Equal(t, bumpLimit, threadAtLimit.NumReplies)
	assert.Equal(t, msgAtLimit.CreatedAt, threadAtLimit.LastBumped, "Last bump timestamp should match the message at the bump limit")
	assert.True(t, threadAtLimit.LastBumped.After(lastBumpTsBeforeLimit), "Timestamp should have updated")
	lastBumpTsAtLimit := threadAtLimit.LastBumped

	time.Sleep(10 * time.Millisecond) // Ensure next message has later timestamp

	// Send one more message (over the bump limit)
	_ = createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: int64(bumpLimit + 2)}, Text: "Reply over limit", ThreadId: threadID})
	time.Sleep(10 * time.Millisecond) // Allow time for potential DB updates

	// Get the thread again and check that the last bump timestamp hasn't changed
	threadOverLimit, err := storage.GetThread(boardShortName, threadID)
	require.NoError(t, err)
	assert.Equal(t, bumpLimit+1, threadOverLimit.NumReplies) // Reply count still increases
	assert.Equal(t, lastBumpTsAtLimit.UTC(), threadOverLimit.LastBumped.UTC(), "Last bump timestamp should NOT change after going over the bump limit")
}
