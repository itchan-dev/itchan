//go:build !polluting

package pg

import (
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessageOperations verifies message CRUD operations with attachments and reply cascades.
func TestMessageOperations(t *testing.T) {
	t.Run("CreateMessage", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName("btest")
		createTestBoard(t, tx, boardShortName)
		userID := createTestUser(t, tx, "author@example.com")
		threadID, _ := createTestThread(t, tx, domain.ThreadCreationData{
			Title: "Test Thread",
			Board: boardShortName,
			OpMessage: domain.MessageCreationData{
				Author: domain.User{Id: userID},
				Text:   "OP Message",
			},
		})

		t.Run("with attachments and replies", func(t *testing.T) {
			targetMsgID := createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Target Message",
				ThreadId: threadID,
			})

			attachments := getRandomAttachments(t)
			creationData := domain.MessageCreationData{
				Board:       boardShortName,
				Author:      domain.User{Id: userID},
				Text:        "A new message",
				Attachments: &attachments,
				ThreadId:    threadID,
				ReplyTo: &domain.Replies{
					{To: targetMsgID, ToThreadId: threadID},
				},
			}

			msgID, err := storage.createMessage(tx, creationData)
			require.NoError(t, err)
			require.Greater(t, msgID, int64(0))

			createdMsg, err := storage.getMessage(tx, boardShortName, msgID)
			require.NoError(t, err)
			assert.Equal(t, creationData.Text, createdMsg.Text)
			require.Len(t, createdMsg.Attachments, 2)
			assert.Equal(t, attachments[0].File.FilePath, createdMsg.Attachments[0].File.FilePath)

			replies, err := storage.getMessageRepliesFrom(tx, boardShortName, msgID)
			require.NoError(t, err)
			require.Len(t, replies, 1)
			assert.Equal(t, targetMsgID, replies[0].To)
		})

		t.Run("updates thread metadata", func(t *testing.T) {
			threadBefore, err := storage.getThread(tx, boardShortName, threadID)
			require.NoError(t, err)

			createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "A new reply",
				ThreadId: threadID,
			})

			threadAfter, err := storage.getThread(tx, boardShortName, threadID)
			require.NoError(t, err)
			assert.Equal(t, threadBefore.MessageCount+1, threadAfter.MessageCount)
			assert.True(t, threadAfter.LastBumped.After(threadBefore.LastBumped))
		})

		t.Run("fails on invalid board or thread", func(t *testing.T) {
			_, err := storage.createMessage(tx, domain.MessageCreationData{
				Board:    "nonexistent",
				Author:   domain.User{Id: userID},
				ThreadId: threadID,
			})
			require.Error(t, err)

			_, err = storage.createMessage(tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				ThreadId: -999,
			})
			requireNotFoundError(t, err)
		})
	})

	t.Run("GetMessage", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName("bget")
		createTestBoard(t, tx, boardShortName)
		userID := createTestUser(t, tx, "getter@example.com")
		threadID, _ := createTestThread(t, tx, domain.ThreadCreationData{
			Title: "Get Test", Board: boardShortName,
			OpMessage: domain.MessageCreationData{Author: domain.User{Id: userID}, Text: "OP"},
		})

		attachments := getRandomAttachments(t)
		msgID := createTestMessage(t, tx, domain.MessageCreationData{
			Board: boardShortName, Author: domain.User{Id: userID},
			Text: "Message to get", ThreadId: threadID, Attachments: &attachments,
		})

		createTestMessage(t, tx, domain.MessageCreationData{
			Board: boardShortName, Author: domain.User{Id: userID},
			Text: "A reply", ThreadId: threadID,
			ReplyTo: &domain.Replies{{To: msgID, ToThreadId: threadID}},
		})

		msg, err := storage.getMessage(tx, boardShortName, msgID)
		require.NoError(t, err)
		assert.Equal(t, msgID, msg.Id)
		assert.Equal(t, "Message to get", msg.Text)
		require.Len(t, msg.Attachments, 2)
		require.Len(t, msg.Replies, 1)
		assert.Equal(t, msgID, msg.Replies[0].To)

		_, err = storage.getMessage(tx, boardShortName, -999)
		requireNotFoundError(t, err)
	})

	t.Run("DeleteMessage", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName("bdel")
		createTestBoard(t, tx, boardShortName)
		userID := createTestUser(t, tx, "deleter@example.com")
		threadID, _ := createTestThread(t, tx, domain.ThreadCreationData{
			Title: "Delete Test", Board: boardShortName,
			OpMessage: domain.MessageCreationData{Author: domain.User{Id: userID}, Text: "OP"},
		})

		msgBeingRepliedTo := createTestMessage(t, tx, domain.MessageCreationData{
			Board: boardShortName, Author: domain.User{Id: userID},
			Text: "I will be replied to", ThreadId: threadID,
		})

		attachments := getRandomAttachments(t)
		msgToDelete := createTestMessage(t, tx, domain.MessageCreationData{
			Board:       boardShortName,
			Author:      domain.User{Id: userID},
			Text:        "I will be deleted",
			ThreadId:    threadID,
			Attachments: &attachments,
			ReplyTo:     &domain.Replies{{To: msgBeingRepliedTo, ToThreadId: threadID}},
		})

		msgReplyingToDeleted := createTestMessage(t, tx, domain.MessageCreationData{
			Board: boardShortName, Author: domain.User{Id: userID},
			Text: "My target will be deleted", ThreadId: threadID,
			ReplyTo: &domain.Replies{{To: msgToDelete, ToThreadId: threadID}},
		})

		_, err := storage.getMessage(tx, boardShortName, msgToDelete)
		require.NoError(t, err)
		attachmentsBefore, err := storage.getMessageAttachments(tx, boardShortName, msgToDelete)
		require.NoError(t, err)
		require.NotEmpty(t, attachmentsBefore)
		repliesToBefore, err := storage.getMessageRepliesTo(tx, boardShortName, msgToDelete)
		require.NoError(t, err)
		require.NotEmpty(t, repliesToBefore)
		repliesFromBefore, err := storage.getMessageRepliesFrom(tx, boardShortName, msgToDelete)
		require.NoError(t, err)
		require.NotEmpty(t, repliesFromBefore)

		err = storage.deleteMessage(tx, boardShortName, msgToDelete)
		require.NoError(t, err)

		_, err = storage.getMessage(tx, boardShortName, msgToDelete)
		requireNotFoundError(t, err)

		attachmentsAfter, err := storage.getMessageAttachments(tx, boardShortName, msgToDelete)
		require.NoError(t, err)
		assert.Empty(t, attachmentsAfter)

		repliesToAfter, err := storage.getMessageRepliesTo(tx, boardShortName, msgToDelete)
		require.NoError(t, err)
		assert.Empty(t, repliesToAfter)

		repliesFromAfter, err := storage.getMessageRepliesFrom(tx, boardShortName, msgToDelete)
		require.NoError(t, err)
		assert.Empty(t, repliesFromAfter)

		_, err = storage.getMessage(tx, boardShortName, msgBeingRepliedTo)
		require.NoError(t, err)

		_, err = storage.getMessage(tx, boardShortName, msgReplyingToDeleted)
		require.NoError(t, err)

		t.Run("fails on invalid board or message", func(t *testing.T) {
			err := storage.deleteMessage(tx, "nonexistent", msgBeingRepliedTo)
			requireNotFoundError(t, err)

			err = storage.deleteMessage(tx, boardShortName, -1)
			requireNotFoundError(t, err)
		})
	})
}
