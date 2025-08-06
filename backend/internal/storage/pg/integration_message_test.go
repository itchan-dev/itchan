package pg

import (
	"fmt"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	_ "github.com/lib/pq" // pg driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// verifyBoardActivityUnchanged asserts that the board's last_activity has not changed.
func verifyBoardActivityUnchanged(t *testing.T, before, after domain.Board) {
	t.Helper()
	assert.Equal(t, before.LastActivity.UTC(), after.LastActivity.UTC(), "Board last_activity should remain unchanged")
}

func TestCreateMessage(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t) // Creates board & thread, handles cleanup
	author := domain.User{Id: 2}
	text := "Test message for create"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}

	t.Run("Create Message with Reply", func(t *testing.T) {
		// Create the base message.
		baseMsgID := createTestMessage(t, domain.MessageCreationData{
			Board:       boardShortName,
			Author:      author,
			Text:        text,
			Attachments: attachments,
			ThreadId:    threadID,
		})

		// Create a reply referencing the base message.
		replyText := "Reply to first message"
		replyMsgID := createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   author,
			Text:     replyText,
			ThreadId: threadID,
			ReplyTo: &domain.Replies{
				{
					FromThreadId: threadID,
					To:           baseMsgID,
					ToThreadId:   threadID,
				},
			},
		})

		// Fetch the base message and verify its reply.
		msg, err := storage.GetMessage(boardShortName, baseMsgID)
		require.NoError(t, err, "GetMessage should not return an error")
		require.NotNil(t, msg.Replies, "Replies should not be nil")
		require.Len(t, msg.Replies, 1, "Base message should have one reply")
		assert.Equal(t, replyMsgID, msg.Replies[0].From, "Reply From should match reply message ID")
		assert.Equal(t, baseMsgID, msg.Replies[0].To, "Reply To should match base message ID")
	})

	t.Run("Multiple Replies Ordering", func(t *testing.T) {
		// Create a base message that will receive multiple replies.
		baseMsgID := createTestMessage(t, domain.MessageCreationData{
			Board:       boardShortName,
			Author:      author,
			Text:        "Base for multiple replies",
			Attachments: attachments,
			ThreadId:    threadID,
		})

		// Create several replies with a slight delay to ensure distinct timestamps.
		replyIDs := []domain.MsgId{}
		replyTexts := []string{"First reply", "Second reply", "Third reply"}
		for _, rText := range replyTexts {
			time.Sleep(10 * time.Millisecond)
			replyID := createTestMessage(t, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   author,
				Text:     rText,
				ThreadId: threadID,
				ReplyTo: &domain.Replies{
					{
						FromThreadId: threadID,
						To:           baseMsgID,
						ToThreadId:   threadID,
					},
				},
			})
			replyIDs = append(replyIDs, replyID)
		}

		// Verify that the base message now has three replies in order.
		msg, err := storage.GetMessage(boardShortName, baseMsgID)
		require.NoError(t, err)
		require.Len(t, msg.Replies, 3, "Expected 3 replies")
		for i, rep := range msg.Replies {
			assert.Equal(t, replyIDs[i], rep.From, fmt.Sprintf("Reply order mismatch at position %d", i))
		}
	})

	t.Run("Successful Creation and Verification", func(t *testing.T) {
		// Delay to ensure the new timestamp is distinct.
		time.Sleep(50 * time.Millisecond)
		timeBeforeCreate := time.Now().UTC()

		creationData := domain.MessageCreationData{
			Board:       boardShortName,
			Author:      author,
			Text:        text,
			Attachments: attachments,
			ThreadId:    threadID,
		}

		msgID, createErr := storage.CreateMessage(creationData, false, nil)
		require.NoError(t, createErr, "CreateMessage should not return an error")
		require.Greater(t, msgID, int64(0), "Message ID should be greater than 0")

		createdMsg, err := storage.GetMessage(creationData.Board, msgID)
		require.NoError(t, err, "Failed to get created message")
		assert.Equal(t, text, createdMsg.Text, "Message text mismatch")
		require.NotNil(t, createdMsg.Attachments, "Attachments should not be nil")
		assert.Equal(t, *attachments, *createdMsg.Attachments, "Attachments mismatch")
		assert.Equal(t, author.Id, createdMsg.Author.Id, "Author ID mismatch")
		require.NotNil(t, createdMsg.ThreadId, "Thread ID should not be nil")
		assert.Equal(t, threadID, createdMsg.ThreadId, "Thread ID mismatch")
		assert.False(t, createdMsg.CreatedAt.IsZero(), "CreatedAt should not be zero")
		assert.False(t, createdMsg.ModifiedAt.IsZero(), "ModifiedAt should not be zero")
		assert.True(t, createdMsg.CreatedAt.Equal(createdMsg.ModifiedAt),
			"CreatedAt [%v] should equal ModifiedAt [%v] on creation", createdMsg.CreatedAt, createdMsg.ModifiedAt)
		assert.True(t, timeBeforeCreate.Before(createdMsg.CreatedAt) || timeBeforeCreate.Equal(createdMsg.CreatedAt),
			"Time before creation should be before or equal to CreatedAt")
		assert.WithinDuration(t, timeBeforeCreate, createdMsg.CreatedAt, 2*time.Second,
			"CreatedAt should be very close to time before creation")

		boardAfterCreate, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after create for: %s", boardShortName)
		// Board's last_activity should update to message's CreatedAt.
		assert.True(t, boardAfterCreate.LastActivity.After(timeBeforeCreate),
			"Board last_activity should be updated after message creation")
		assert.Equal(t, createdMsg.CreatedAt.UTC(), boardAfterCreate.LastActivity.UTC(),
			"Board last_activity should equal message CreatedAt")
	})

	t.Run("Op Message does not Increment num_replies", func(t *testing.T) {
		threadBefore, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err)

		creationData := domain.MessageCreationData{
			Board:       boardShortName,
			Author:      author,
			Text:        "Test num_replies",
			Attachments: nil,
			ThreadId:    threadID,
		}
		// Creating an op message should not change num_replies.
		_, err = storage.CreateMessage(creationData, true, nil)
		require.NoError(t, err, "Op message should be created successfully")
		threadAfter, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err)
		assert.Equal(t, threadBefore.NumReplies, threadAfter.NumReplies, "Op message should not increment num_replies")

		// Creating a non-op message should increment num_replies.
		_, err = storage.CreateMessage(creationData, false, nil)
		require.NoError(t, err, "Non-op message should be created successfully")
		threadAfter, err = storage.GetThread(boardShortName, threadID)
		require.NoError(t, err)
		assert.Equal(t, threadBefore.NumReplies+1, threadAfter.NumReplies, "Non-op message should increment num_replies")
	})

	t.Run("Failure - Non-existent Thread", func(t *testing.T) {
		boardBefore, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "GetBoard should succeed before failing creation")

		creationData := domain.MessageCreationData{
			Board:       boardShortName,
			Author:      author,
			Text:        text,
			Attachments: attachments,
			ThreadId:    -1, // Invalid thread ID
		}
		_, err = storage.CreateMessage(creationData, false, nil)
		requireNotFoundError(t, err)
		boardAfter, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "GetBoard should succeed after failed creation")
		verifyBoardActivityUnchanged(t, boardBefore, boardAfter)
	})

	t.Run("Failure - Non-existent Board", func(t *testing.T) {
		creationData := domain.MessageCreationData{
			Board:       "nonexistentboard", // Invalid board
			Author:      author,
			Text:        text,
			Attachments: attachments,
			ThreadId:    threadID,
		}
		_, err := storage.CreateMessage(creationData, false, nil)
		requireNotFoundError(t, err)
	})

	t.Run("Creation with Custom Timestamp", func(t *testing.T) {
		customTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		creationData := domain.MessageCreationData{
			Board:     boardShortName,
			Author:    author,
			Text:      "Custom timestamp test",
			ThreadId:  threadID,
			CreatedAt: &customTime,
		}
		msgID, err := storage.CreateMessage(creationData, false, nil)
		require.NoError(t, err)
		msg, err := storage.GetMessage(boardShortName, msgID)
		require.NoError(t, err)
		assert.True(t, msg.CreatedAt.Equal(customTime), "CreatedAt should match custom timestamp")
	})
}

func TestGetMessage(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t)
	author := domain.User{Id: 2}
	text := "Test message for get"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID := createTestMessage(t, domain.MessageCreationData{
		Board:       boardShortName,
		Author:      author,
		Text:        text,
		Attachments: attachments,
		ThreadId:    threadID,
	})

	t.Run("Get Existing Message with Replies", func(t *testing.T) {
		// Create a reply to the message.
		replyText := "Reply for get"
		replyMsgID := createTestMessage(t, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   author,
			Text:     replyText,
			ThreadId: threadID,
			ReplyTo: &domain.Replies{
				{
					FromThreadId: threadID,
					To:           msgID,
					ToThreadId:   threadID,
				},
			},
		})
		msg, err := storage.GetMessage(boardShortName, msgID)
		require.NoError(t, err, "GetMessage should succeed")
		require.NotNil(t, msg.Replies, "Replies should not be nil")
		require.Len(t, msg.Replies, 1, "Expected one reply")
		assert.Equal(t, replyMsgID, msg.Replies[0].From, "Reply From should match reply message ID")
		assert.Equal(t, msgID, msg.Replies[0].To, "Reply To should match original message ID")
	})

	t.Run("Get Existing Message without Replies", func(t *testing.T) {
		msg, err := storage.GetMessage(boardShortName, msgID)
		require.NoError(t, err, "GetMessage should succeed")
		assert.Equal(t, msgID, msg.Id, "Message ID mismatch")
		assert.Equal(t, author.Id, msg.Author.Id, "Author ID mismatch")
		assert.Equal(t, text, msg.Text, "Message text mismatch")
		require.NotNil(t, msg.Attachments, "Attachments should not be nil")
		assert.Equal(t, *attachments, *msg.Attachments, "Attachments mismatch")
		require.NotNil(t, msg.ThreadId, "Thread ID should not be nil")
		assert.Equal(t, threadID, msg.ThreadId, "Thread ID mismatch")
		assert.False(t, msg.CreatedAt.IsZero(), "CreatedAt should not be zero")
		assert.False(t, msg.ModifiedAt.IsZero(), "ModifiedAt should not be zero")
		assert.Equal(t, msg.CreatedAt.UTC(), msg.ModifiedAt.UTC(), "ModifiedAt should equal CreatedAt initially")
	})

	t.Run("Get Non-existent Message", func(t *testing.T) {
		_, err := storage.GetMessage(boardShortName, -1)
		requireNotFoundError(t, err)
	})
}

func TestDeleteMessage(t *testing.T) {
	// Setup: create board, thread and a message.
	boardShortName, _, msgID := setupBoardAndThreadAndMessage(t)
	originalMsg, err := storage.GetMessage(boardShortName, msgID)
	require.NoError(t, err, "Failed to get original message")

	boardAfterCreate, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err, "Failed to get board state after message creation")

	t.Run("Successful Deletion and Board Update", func(t *testing.T) {
		time.Sleep(50 * time.Millisecond) // Ensure timestamp gap.
		timeBeforeDelete := time.Now().UTC()

		deleteErr := storage.DeleteMessage(boardShortName, msgID)
		require.NoError(t, deleteErr, "DeleteMessage should succeed")
		updatedMsg, err := storage.GetMessage(boardShortName, msgID)
		require.NoError(t, err, "Message should be retrievable after soft delete")
		assert.Equal(t, "deleted", updatedMsg.Text, "Message text should be updated to 'deleted'")
		assert.Nil(t, updatedMsg.Attachments, "Attachments should be nil after deletion")
		require.False(t, updatedMsg.ModifiedAt.IsZero(), "ModifiedAt should not be zero")
		assert.True(t, updatedMsg.ModifiedAt.After(originalMsg.ModifiedAt),
			"ModifiedAt should be updated after deletion")
		assert.Equal(t, originalMsg.CreatedAt.UTC(), updatedMsg.CreatedAt.UTC(), "CreatedAt should remain unchanged")
		assert.WithinDuration(t, timeBeforeDelete, updatedMsg.ModifiedAt, 2*time.Second,
			"ModifiedAt should be close to deletion time")

		boardAfterDelete, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after delete")
		assert.True(t, boardAfterDelete.LastActivity.After(boardAfterCreate.LastActivity),
			"Board last_activity should be updated after deletion")
		assert.Equal(t, updatedMsg.ModifiedAt.UTC(), boardAfterDelete.LastActivity.UTC(),
			"Board last_activity should equal message ModifiedAt after deletion")
	})

	t.Run("Failure - Non-existent Message", func(t *testing.T) {
		boardBeforeFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "GetBoard should succeed before deletion failure")
		err = storage.DeleteMessage(boardShortName, -1)
		requireNotFoundError(t, err)
		boardAfterFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "GetBoard should succeed after deletion failure")
		verifyBoardActivityUnchanged(t, boardBeforeFail, boardAfterFail)
	})

	t.Run("Failure - Non-existent Board", func(t *testing.T) {
		err := storage.DeleteMessage("nonexistentboard", msgID)
		requireNotFoundError(t, err)
	})
}
