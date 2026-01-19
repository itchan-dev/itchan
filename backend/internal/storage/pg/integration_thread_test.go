//go:build !polluting

package pg

import (
	"fmt"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

// TestThreadOperations verifies thread CRUD operations and bump limit enforcement.
func TestThreadOperations(t *testing.T) {
	t.Run("CreateThread", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName(generateString(t))
		createTestBoard(t, tx, boardShortName)
		userID := createTestUser(t, tx, generateString(t)+"@example.com")

		t.Run("Success", func(t *testing.T) {
			opMsg := domain.MessageCreationData{
				Board:  boardShortName,
				Author: domain.User{Id: userID},
				Text:   "Original Post Text",
			}

			time.Sleep(50 * time.Millisecond)
			creationTimeStart := time.Now()

			threadID, createdAt, err := storage.createThread(tx, domain.ThreadCreationData{
				Title:     "Test Thread Creation",
				Board:     boardShortName,
				OpMessage: opMsg,
			})
			require.NoError(t, err)
			require.Greater(t, threadID, int64(0))

			opMsg.ThreadId = threadID
			opMsg.CreatedAt = &createdAt
			opMsg.Board = boardShortName
			_, _, err = storage.createMessage(tx, opMsg)
			require.NoError(t, err)

			createdThread, err := storage.getThread(tx, boardShortName, threadID, 1)
			require.NoError(t, err)

			assert.Equal(t, "Test Thread Creation", createdThread.Title)
			assert.Equal(t, boardShortName, createdThread.Board)
			assert.Equal(t, 1, createdThread.MessageCount)
			require.Len(t, createdThread.Messages, 1)

			op := createdThread.Messages[0]
			assert.Equal(t, opMsg.Text, op.Text)
			assert.Equal(t, opMsg.Author.Id, op.Author.Id)
			assert.Empty(t, op.Attachments) // No attachments added
			assert.Equal(t, threadID, op.ThreadId)

			assert.WithinDuration(t, creationTimeStart, createdThread.LastBumped, 5*time.Second)
			assert.Equal(t, createdAt, createdThread.LastBumped)
		})

		t.Run("BoardNotFound", func(t *testing.T) {
			opMsg := domain.MessageCreationData{
				Board:  "nonexistentboard",
				Author: domain.User{Id: userID},
				Text:   "Original Post Text",
			}
			_, _, err := storage.createThread(tx, domain.ThreadCreationData{
				Title:     "Invalid Board Thread",
				Board:     "nonexistentboard",
				OpMessage: opMsg,
			})
			requireNotFoundError(t, err)
		})

		t.Run("CreatePinnedThread", func(t *testing.T) {
			opMsg := domain.MessageCreationData{
				Board:  boardShortName,
				Author: domain.User{Id: userID},
				Text:   "Pinned Post",
			}
			threadID, createdAt, err := storage.createThread(tx, domain.ThreadCreationData{
				Title:    "Pinned Thread",
				Board:    boardShortName,
				IsPinned: true,
				OpMessage: opMsg,
			})
			require.NoError(t, err)

			opMsg.ThreadId = threadID
			opMsg.CreatedAt = &createdAt
			opMsg.Board = boardShortName
			_, _, err = storage.createMessage(tx, opMsg)
			require.NoError(t, err)

			thread, err := storage.getThread(tx, boardShortName, threadID, 1)
			require.NoError(t, err)
			assert.True(t, thread.IsPinned)
		})
	})

	t.Run("GetThread", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName(generateString(t))
		createTestBoard(t, tx, boardShortName)
		userID := createTestUser(t, tx, generateString(t)+"@example.com")

		t.Run("WithReplies", func(t *testing.T) {
			opMsg := domain.MessageCreationData{
				Board:  boardShortName,
				Author: domain.User{Id: userID},
				Text:   "Test OP Get",
			}

			threadID, createdAt, err := storage.createThread(tx, domain.ThreadCreationData{
				Title:     "Test Get Thread WithReplies",
				Board:     boardShortName,
				OpMessage: opMsg,
			})
			require.NoError(t, err)

			opMsg.ThreadId = threadID
			opMsg.CreatedAt = &createdAt
			opMsg.Board = boardShortName
			_, _, err = storage.createMessage(tx, opMsg)
			require.NoError(t, err)

			user2 := createTestUser(t, tx, generateString(t)+"@example.com")
			user3 := createTestUser(t, tx, generateString(t)+"@example.com")

			msgID1, _, err := storage.createMessage(tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: user2},
				Text:     "Reply 1 Text",
				ThreadId: threadID,
			})
			require.NoError(t, err)

			time.Sleep(10 * time.Millisecond)
			msgID2, _, err := storage.createMessage(tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: user3},
				Text:     "Reply 2 Text",
				ThreadId: threadID,
			})
			require.NoError(t, err)

			thread, err := storage.getThread(tx, boardShortName, threadID, 1)
			require.NoError(t, err)
			assert.Equal(t, "Test Get Thread WithReplies", thread.Title)
			assert.Equal(t, boardShortName, thread.Board)
			assert.Equal(t, 3, thread.MessageCount)
			require.Len(t, thread.Messages, 3)

			requireMessageOrder(t, thread.Messages, []string{"Test OP Get", "Reply 1 Text", "Reply 2 Text"})

			op := thread.Messages[0]
			assert.Equal(t, threadID, op.ThreadId)
			assert.Empty(t, op.Attachments) // No attachments added

			reply1 := thread.Messages[1]
			assert.Equal(t, msgID1, reply1.Id)
			require.NotNil(t, reply1.ThreadId)
			assert.Equal(t, threadID, reply1.ThreadId)
			assert.Empty(t, reply1.Attachments)
			assert.Equal(t, user2, reply1.Author.Id)
			assert.Len(t, reply1.Replies, 0)

			reply2 := thread.Messages[2]
			assert.Equal(t, msgID2, reply2.Id)
			require.NotNil(t, reply2.ThreadId)
			assert.Equal(t, threadID, reply2.ThreadId)
			assert.Nil(t, reply2.Attachments)
			assert.Equal(t, user3, reply2.Author.Id)
			assert.Len(t, reply2.Replies, 0)

			assert.Equal(t, reply2.CreatedAt, thread.LastBumped)
		})

		t.Run("RepliesToMessages", func(t *testing.T) {
			opMsg := domain.MessageCreationData{
				Board:  boardShortName,
				Author: domain.User{Id: userID},
				Text:   "OP for replies test",
			}

			threadID, createdAt, err := storage.createThread(tx, domain.ThreadCreationData{
				Title:     "Thread With Message Replies",
				Board:     boardShortName,
				OpMessage: opMsg,
			})
			require.NoError(t, err)

			opMsg.ThreadId = threadID
			opMsg.CreatedAt = &createdAt
			opMsg.Board = boardShortName
			_, _, err = storage.createMessage(tx, opMsg)
			require.NoError(t, err)

			user2 := createTestUser(t, tx, generateString(t)+"@example.com")
			user3 := createTestUser(t, tx, generateString(t)+"@example.com")

			msgID1, _, err := storage.createMessage(tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: user2},
				Text:     "First reply",
				ThreadId: threadID,
			})
			require.NoError(t, err)

			msgID2, _, err := storage.createMessage(tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: user3},
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
			require.NoError(t, err)

			thread, err := storage.getThread(tx, boardShortName, threadID, 1)
			require.NoError(t, err)
			require.Len(t, thread.Messages, 3)

			var firstReply *domain.Message
			for i := range thread.Messages {
				if thread.Messages[i].Id == msgID1 {
					firstReply = thread.Messages[i]
					break
				}
			}

			require.NotNil(t, firstReply)
			require.Len(t, firstReply.Replies, 1)
			assert.Equal(t, msgID2, firstReply.Replies[0].From)
			assert.Equal(t, msgID1, firstReply.Replies[0].To)
		})

		t.Run("OnlyOP", func(t *testing.T) {
			opMsg := domain.MessageCreationData{
				Board:  boardShortName,
				Author: domain.User{Id: userID},
				Text:   "Lonely OP",
			}

			threadID, createdAt, err := storage.createThread(tx, domain.ThreadCreationData{
				Title:     "Only OP Thread",
				Board:     boardShortName,
				OpMessage: opMsg,
			})
			require.NoError(t, err)

			opMsg.ThreadId = threadID
			opMsg.CreatedAt = &createdAt
			opMsg.Board = boardShortName
			_, _, err = storage.createMessage(tx, opMsg)
			require.NoError(t, err)

			thread, err := storage.getThread(tx, boardShortName, threadID, 1)
			require.NoError(t, err)
			assert.Equal(t, "Only OP Thread", thread.Title)
			assert.Equal(t, boardShortName, thread.Board)
			assert.Equal(t, 1, thread.MessageCount)
			require.Len(t, thread.Messages, 1)

			op := thread.Messages[0]
			assert.Equal(t, "Lonely OP", op.Text)
			assert.Equal(t, createdAt, thread.LastBumped)
		})

		t.Run("NotFound", func(t *testing.T) {
			_, err := storage.getThread(tx, boardShortName, -999, 1)
			requireNotFoundError(t, err)
		})
	})

	t.Run("DeleteThread", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName(generateString(t))
		createTestBoard(t, tx, boardShortName)
		userID := createTestUser(t, tx, generateString(t)+"@example.com")

		t.Run("NotFoundThread", func(t *testing.T) {
			err := storage.deleteThread(tx, boardShortName, -999)
			requireNotFoundError(t, err)
		})

		t.Run("NotFoundBoard", func(t *testing.T) {
			opMsg := domain.MessageCreationData{
				Board:  boardShortName,
				Author: domain.User{Id: userID},
				Text:   "Temp OP",
			}
			threadID, createdAt, err := storage.createThread(tx, domain.ThreadCreationData{
				Title:     "Temp Thread",
				Board:     boardShortName,
				OpMessage: opMsg,
			})
			require.NoError(t, err)

			opMsg.ThreadId = threadID
			opMsg.CreatedAt = &createdAt
			opMsg.Board = boardShortName
			_, _, err = storage.createMessage(tx, opMsg)
			require.NoError(t, err)

			err = storage.deleteThread(tx, "nonexistentboard", threadID)
			requireNotFoundError(t, err)
		})

		t.Run("Success", func(t *testing.T) {
			opMsg := domain.MessageCreationData{
				Board:  boardShortName,
				Author: domain.User{Id: userID},
				Text:   "Test OP Delete",
			}

			threadID, createdAt, err := storage.createThread(tx, domain.ThreadCreationData{
				Title:     "Test Delete Thread",
				Board:     boardShortName,
				OpMessage: opMsg,
			})
			require.NoError(t, err)

			opMsg.ThreadId = threadID
			opMsg.CreatedAt = &createdAt
			opMsg.Board = boardShortName
			opMsgID, _, err := storage.createMessage(tx, opMsg)
			require.NoError(t, err)

			reply1 := createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Reply 1 Delete",
				ThreadId: threadID,
			})
			reply2 := createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Reply 2 Delete",
				ThreadId: threadID,
				ReplyTo: &domain.Replies{
					{
						FromThreadId: threadID,
						To:           opMsgID,
						ToThreadId:   threadID,
					},
				},
			})

			err = storage.deleteThread(tx, boardShortName, threadID)
			require.NoError(t, err)

			_, err = storage.getThread(tx, boardShortName, threadID, 1)
			requireNotFoundError(t, err)

			_, err = storage.getMessage(tx, boardShortName, opMsgID)
			requireNotFoundError(t, err)
			_, err = storage.getMessage(tx, boardShortName, reply1)
			requireNotFoundError(t, err)
			_, err = storage.getMessage(tx, boardShortName, reply2)
			requireNotFoundError(t, err)

			// Verify that cascading deletes removed related data
			replies, err := storage.getMessageRepliesFrom(tx, boardShortName, opMsgID)
			require.NoError(t, err)
			assert.Empty(t, replies, "Replies should be deleted via cascade")

			attachments, err := storage.getMessageAttachments(tx, boardShortName, opMsgID)
			require.NoError(t, err)
			assert.Empty(t, attachments, "Attachments should be deleted via cascade")
		})
	})

	t.Run("BumpLimit", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName(generateString(t))
		createTestBoard(t, tx, boardShortName)
		userID := createTestUser(t, tx, generateString(t)+"@example.com")

		opMsg := domain.MessageCreationData{
			Board:  boardShortName,
			Author: domain.User{Id: userID},
			Text:   "OP for bump test",
		}

		threadID, createdAt, err := storage.createThread(tx, domain.ThreadCreationData{
			Title:     "Bump Limit Test",
			Board:     boardShortName,
			OpMessage: opMsg,
		})
		require.NoError(t, err)

		opMsg.ThreadId = threadID
		opMsg.CreatedAt = &createdAt
		opMsg.Board = boardShortName
		_, _, err = storage.createMessage(tx, opMsg)
		require.NoError(t, err)

		bumpLimit := storage.cfg.Public.BumpLimit
		require.Greater(t, bumpLimit, 0)

		for i := 0; i < bumpLimit-1; i++ {
			_, _, err := storage.createMessage(tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     fmt.Sprintf("Reply %d", i+1),
				ThreadId: threadID,
			})
			require.NoError(t, err)
		}

		threadBefore, err := storage.getThread(tx, boardShortName, threadID, 1)
		require.NoError(t, err)
		require.Equal(t, bumpLimit, threadBefore.MessageCount)
		lastBumpBefore := threadBefore.LastBumped

		msgAtLimit, _, err := storage.createMessage(tx, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: userID},
			Text:     fmt.Sprintf("Reply %d (at limit)", bumpLimit),
			ThreadId: threadID,
		})
		require.NoError(t, err)

		createdMsgAtLimit, err := storage.getMessage(tx, boardShortName, msgAtLimit)
		require.NoError(t, err)

		threadAtLimit, err := storage.getThread(tx, boardShortName, threadID, 1)
		require.NoError(t, err)
		assert.Equal(t, bumpLimit+1, threadAtLimit.MessageCount)
		assert.Equal(t, createdMsgAtLimit.CreatedAt, threadAtLimit.LastBumped)
		assert.True(t, threadAtLimit.LastBumped.After(lastBumpBefore))
		lastBumpAtLimit := threadAtLimit.LastBumped

		_ = createTestMessage(t, tx, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: userID},
			Text:     "Reply over limit",
			ThreadId: threadID,
		})

		threadOverLimit, err := storage.getThread(tx, boardShortName, threadID, 1)
		require.NoError(t, err)
		assert.Equal(t, bumpLimit+2, threadOverLimit.MessageCount)
		assert.Equal(t, lastBumpAtLimit.UTC(), threadOverLimit.LastBumped.UTC())
	})
}
