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

		// Allow some time for distinct timestamps
		time.Sleep(50 * time.Millisecond)
		creationTimeStart := time.Now()

		threadID, err := storage.CreateThread(creationData)
		require.NoError(t, err, "CreateThread should succeed")
		require.Greater(t, threadID, int64(0), "Thread ID should be positive")

		boardAfter, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)

		createdThread, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err, "GetThread should find the newly created thread")

		assert.Equal(t, title, createdThread.Title, "Thread title mismatch")
		assert.Equal(t, boardShortName, createdThread.Board, "Thread board mismatch")
		assert.Equal(t, 0, createdThread.NumReplies, "New thread should have 0 replies")
		require.Len(t, createdThread.Messages, 1, "Thread should contain only the OP message")

		// Verify OP message details
		op := createdThread.Messages[0]
		assert.Equal(t, opMsg.Text, op.Text, "OP message text mismatch")
		assert.Equal(t, opMsg.Author.Id, op.Author.Id, "OP author ID mismatch")
		require.NotNil(t, op.Attachments, "OP attachments should not be nil")
		assert.Equal(t, *opMsg.Attachments, *op.Attachments, "OP attachments mismatch")
		assert.Equal(t, threadID, op.ThreadId, "OP message ThreadId should match thread ID")

		// Verify timestamps and board last_activity update
		assert.WithinDuration(t, creationTimeStart, createdThread.LastBumped, 5*time.Second, "LastBumped should be recent")
		assert.Equal(t, op.CreatedAt, createdThread.LastBumped, "LastBumped should equal OP CreatedAt")
		assert.True(t, boardBefore.LastActivity.Before(boardAfter.LastActivity), "Board last_activity should update on thread creation")
		assert.WithinDuration(t, boardAfter.LastActivity, createdThread.LastBumped, 100*time.Millisecond, "Board last_activity should be very close to LastBumped")

		// Cleanup
		err = storage.DeleteThread(boardShortName, threadID)
		require.NoError(t, err, "Failed to cleanup thread in subtest")
	})

	t.Run("BoardNotFound", func(t *testing.T) {
		// Change board for OP msg to an invalid value.
		opMsgInvalid := opMsg
		opMsgInvalid.Board = "nonexistentboard"
		invalidData := domain.ThreadCreationData{
			Title:     "Invalid Board Thread",
			Board:     "nonexistentboard",
			OpMessage: opMsgInvalid,
		}
		_, err := storage.CreateThread(invalidData)
		requireNotFoundError(t, err)
	})

	t.Run("CreateStickyThread", func(t *testing.T) {
		stickyData := domain.ThreadCreationData{
			Title:     "Sticky Thread",
			Board:     boardShortName,
			IsSticky:  true,
			OpMessage: opMsg,
		}
		threadID, err := storage.CreateThread(stickyData)
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

	t.Run("WithReplies", func(t *testing.T) {
		threadID := createTestThread(t, domain.ThreadCreationData{
			Title:     title + "_WithReplies",
			Board:     boardShortName,
			OpMessage: opMsg,
		})
		t.Cleanup(func() { require.NoError(t, storage.DeleteThread(boardShortName, threadID)) })

		// Create two replies
		replyMsgs := []struct {
			author domain.User
			text   string
		}{
			{author: domain.User{Id: 2}, text: "Reply 1 Text"},
			{author: domain.User{Id: 3}, text: "Reply 2 Text"},
		}
		msgID1 := createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   replyMsgs[0].author,
			Text:     replyMsgs[0].text,
			ThreadId: threadID,
		})
		time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps
		msgID2 := createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   replyMsgs[1].author,
			Text:     replyMsgs[1].text,
			ThreadId: threadID,
		})

		thread, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err, "GetThread should not return an error")
		assert.Equal(t, title+"_WithReplies", thread.Title)
		assert.Equal(t, boardShortName, thread.Board)
		assert.Equal(t, 2, thread.NumReplies, "Mismatch in reply count")
		require.Len(t, thread.Messages, 3, "Expected 3 messages (OP + 2 replies)")

		// Verify ordering using helper
		requireMessageOrder(t, thread.Messages, []string{opMsg.Text, replyMsgs[0].text, replyMsgs[1].text})

		// Check OP message details
		op := thread.Messages[0]
		assert.Equal(t, threadID, op.ThreadId)
		require.NotNil(t, op.Attachments)
		assert.Equal(t, *opMsg.Attachments, *op.Attachments)

		// Verify reply 1
		reply1 := thread.Messages[1]
		assert.Equal(t, msgID1, reply1.Id)
		require.NotNil(t, reply1.ThreadId)
		assert.Equal(t, threadID, reply1.ThreadId)
		assert.Nil(t, reply1.Attachments)
		assert.Equal(t, replyMsgs[0].author.Id, reply1.Author.Id)
		assert.Len(t, reply1.Replies, 0, "Reply 1 should have no nested replies")

		// Verify reply 2
		reply2 := thread.Messages[2]
		assert.Equal(t, msgID2, reply2.Id)
		require.NotNil(t, reply2.ThreadId)
		assert.Equal(t, threadID, reply2.ThreadId)
		assert.Nil(t, reply2.Attachments)
		assert.Equal(t, replyMsgs[1].author.Id, reply2.Author.Id)
		assert.Len(t, reply2.Replies, 0, "Reply 2 should have no nested replies")

		// LastBumped should equal the creation time of the last reply
		assert.Equal(t, reply2.CreatedAt, thread.LastBumped, "LastBumped should equal last reply's CreatedAt")
	})

	t.Run("RepliesToMessages", func(t *testing.T) {
		// Test nested replies (reply to a reply)
		boardForNested := setupBoard(t)
		title := "Thread With Message Replies"
		opMsg := domain.MessageCreationData{
			Board:       boardForNested,
			Author:      domain.User{Id: 1},
			Text:        "OP for replies test",
			Attachments: &domain.Attachments{"op.png"},
		}
		threadID := createTestThread(t, domain.ThreadCreationData{
			Title:     title,
			Board:     boardForNested,
			OpMessage: opMsg,
		})
		t.Cleanup(func() { require.NoError(t, storage.DeleteThread(boardForNested, threadID)) })

		// Create first reply to the OP
		msgID1 := createTestMessage(t, domain.MessageCreationData{
			Board:    boardForNested,
			Author:   domain.User{Id: 2},
			Text:     "First reply",
			ThreadId: threadID,
		})
		// Create second reply replying to the first reply (nested reply)
		msgID2 := createTestMessage(t, domain.MessageCreationData{
			Board:    boardForNested,
			Author:   domain.User{Id: 3},
			Text:     "Reply to first reply",
			ThreadId: threadID,
			ReplyTo: &domain.Replies{
				{
					FromThreadId: threadID,
					To:           msgID1,
					ToThreadId:   threadID,
					CreatedAt:    time.Now().UTC(),
				},
			},
		})

		thread, err := storage.GetThread(boardForNested, threadID)
		require.NoError(t, err, "GetThread should not return an error")
		require.Len(t, thread.Messages, 3, "Expected 3 messages (OP, first reply, nested reply)")

		// Find the first reply among messages
		var firstReply *domain.Message
		for i := range thread.Messages {
			if thread.Messages[i].Id == msgID1 {
				firstReply = thread.Messages[i]
				break
			}
		}

		require.NotNil(t, firstReply, "First reply must be found")
		require.Len(t, firstReply.Replies, 1, "First reply should have one nested reply")
		assert.Equal(t, msgID2, firstReply.Replies[0].From, "Nested reply From mismatch")
		assert.Equal(t, msgID1, firstReply.Replies[0].To, "Nested reply To should match first reply ID")
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := storage.GetThread(boardShortName, -999) // Non-existent thread ID
		requireNotFoundError(t, err)
	})

	t.Run("OnlyOP", func(t *testing.T) {
		threadID := createTestThread(t, domain.ThreadCreationData{
			Title:     title + "_OnlyOP",
			Board:     boardShortName,
			OpMessage: opMsg,
		})
		t.Cleanup(func() { require.NoError(t, storage.DeleteThread(boardShortName, threadID)) })

		thread, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err, "GetThread should succeed for OP only")
		assert.Equal(t, title+"_OnlyOP", thread.Title)
		assert.Equal(t, boardShortName, thread.Board)
		assert.Equal(t, 0, thread.NumReplies)
		require.Len(t, thread.Messages, 1, "Only OP message expected")

		op := thread.Messages[0]
		assert.Equal(t, opMsg.Text, op.Text)
		require.NotNil(t, op.Attachments)
		assert.Equal(t, *opMsg.Attachments, *op.Attachments)
		assert.Equal(t, op.CreatedAt, thread.LastBumped, "LastBumped should equal OP CreatedAt")
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
		Text:        "Test OP for delete",
		Attachments: &domain.Attachments{"file1.jpg"},
	}

	t.Run("NotFoundThread", func(t *testing.T) {
		err := storage.DeleteThread(boardShortName, -999)
		requireNotFoundError(t, err)
	})

	t.Run("NotFoundBoard", func(t *testing.T) {
		// Create a thread to ensure thread exists but board is invalid
		threadID := createTestThread(t, domain.ThreadCreationData{
			Title:     "Temp Thread",
			Board:     boardShortName,
			OpMessage: opMsg,
		})
		t.Cleanup(func() { _ = storage.DeleteThread(boardShortName, threadID) })
		err := storage.DeleteThread("nonexistentboard", threadID)
		requireNotFoundError(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		title := "Test Delete Thread"
		opMsgCopy := opMsg
		opMsgCopy.Text = "Test OP Delete"
		threadID := createTestThread(t, domain.ThreadCreationData{
			Title:     title,
			Board:     boardShortName,
			OpMessage: opMsgCopy,
		})
		// Create two replies for the thread
		_ = createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: 2},
			Text:     "Reply 1 Delete",
			ThreadId: threadID,
		})
		_ = createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: 3},
			Text:     "Reply 2 Delete",
			ThreadId: threadID,
		})

		boardBefore, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)

		err = storage.DeleteThread(boardShortName, threadID)
		require.NoError(t, err, "DeleteThread should succeed")

		_, err = storage.GetThread(boardShortName, threadID)
		requireNotFoundError(t, err)

		boardAfter, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		assert.True(t, boardBefore.LastActivity.Before(boardAfter.LastActivity), "Board last_activity should update after deletion")
		assert.WithinDuration(t, time.Now(), boardAfter.LastActivity, 5*time.Second, "Board last_activity should be recent")
	})
}

// ==================
// TestBumpLimit
// ==================

func TestBumpLimit(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t)
	t.Cleanup(func() { require.NoError(t, storage.DeleteThread(boardShortName, threadID)) })

	bumpLimit := storage.cfg.Public.BumpLimit
	require.Greater(t, bumpLimit, 0, "BumpLimit must be > 0")

	// Send messages to reach just below the bump limit
	for i := 0; i < bumpLimit-1; i++ {
		createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: int64(i + 2)},
			Text:     fmt.Sprintf("Reply %d", i+1),
			ThreadId: threadID,
		})
	}

	threadBefore, err := storage.GetThread(boardShortName, threadID)
	require.NoError(t, err)
	require.Equal(t, bumpLimit-1, threadBefore.NumReplies)
	lastBumpBefore := threadBefore.LastBumped

	time.Sleep(10 * time.Millisecond)
	msgAtLimitID := createTestMessage(t, domain.MessageCreationData{
		Board:    boardShortName,
		Author:   domain.User{Id: int64(bumpLimit + 1)},
		Text:     fmt.Sprintf("Reply %d (at limit)", bumpLimit),
		ThreadId: threadID,
	})
	time.Sleep(10 * time.Millisecond)

	msgAtLimit, err := storage.GetMessage(boardShortName, msgAtLimitID)
	require.NoError(t, err)

	threadAtLimit, err := storage.GetThread(boardShortName, threadID)
	require.NoError(t, err)
	assert.Equal(t, bumpLimit, threadAtLimit.NumReplies)
	assert.Equal(t, msgAtLimit.CreatedAt, threadAtLimit.LastBumped, "LastBumped should match message at limit")
	assert.True(t, threadAtLimit.LastBumped.After(lastBumpBefore), "LastBumped should update")
	lastBumpAtLimit := threadAtLimit.LastBumped

	time.Sleep(10 * time.Millisecond)
	_ = createTestMessage(t, domain.MessageCreationData{
		Board:    boardShortName,
		Author:   domain.User{Id: int64(bumpLimit + 2)},
		Text:     "Reply over limit",
		ThreadId: threadID,
	})
	time.Sleep(10 * time.Millisecond)

	threadOverLimit, err := storage.GetThread(boardShortName, threadID)
	require.NoError(t, err)
	// The reply count still increases but LastBumped remains unchanged
	assert.Equal(t, bumpLimit+1, threadOverLimit.NumReplies)
	assert.Equal(t, lastBumpAtLimit.UTC(), threadOverLimit.LastBumped.UTC(), "LastBumped should not change after exceeding bump limit")
}
