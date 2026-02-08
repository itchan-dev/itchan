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
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "A new message",
				ThreadId: threadID,
				ReplyTo: &domain.Replies{
					{To: targetMsgID, ToThreadId: threadID},
				},
			}

			msgID, err := storage.createMessage(tx, creationData)
			require.NoError(t, err)
			require.Greater(t, msgID, int64(0))

			// Add attachments separately
			err = storage.addAttachments(tx, boardShortName, threadID, msgID, attachments)
			require.NoError(t, err)

			createdMsg, err := storage.getMessage(tx, boardShortName, threadID, msgID)
			require.NoError(t, err)
			assert.Equal(t, creationData.Text, createdMsg.Text)
			require.Len(t, createdMsg.Attachments, 2)
			assert.Equal(t, attachments[0].File.FilePath, createdMsg.Attachments[0].File.FilePath)

			replies, err := storage.getMessageRepliesFrom(tx, boardShortName, threadID, msgID)
			require.NoError(t, err)
			require.Len(t, replies, 1)
			assert.Equal(t, targetMsgID, replies[0].To)
		})

		t.Run("updates thread metadata", func(t *testing.T) {
			threadBefore, err := storage.getThread(tx, boardShortName, threadID, 1)
			require.NoError(t, err)

			createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "A new reply",
				ThreadId: threadID,
			})

			threadAfter, err := storage.getThread(tx, boardShortName, threadID, 1)
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

		t.Run("stores and retrieves show_email_domain", func(t *testing.T) {
			// Create message with ShowEmailDomain = true
			msgWithDomain := createTestMessage(t, tx, domain.MessageCreationData{
				Board:           boardShortName,
				Author:          domain.User{Id: userID},
				Text:            "Company post",
				ShowEmailDomain: true,
				ThreadId:        threadID,
			})

			retrieved, err := storage.getMessage(tx, boardShortName, threadID, msgWithDomain)
			require.NoError(t, err)
			assert.True(t, retrieved.ShowEmailDomain, "ShowEmailDomain should be true")
			assert.Equal(t, "example.com", retrieved.Author.EmailDomain)

			// Create message with ShowEmailDomain = false (default)
			msgWithoutDomain := createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Anonymous post",
				ThreadId: threadID,
			})

			retrieved2, err := storage.getMessage(tx, boardShortName, threadID, msgWithoutDomain)
			require.NoError(t, err)
			assert.False(t, retrieved2.ShowEmailDomain, "ShowEmailDomain should default to false")
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

		targetMsgID := createTestMessage(t, tx, domain.MessageCreationData{
			Board: boardShortName, Author: domain.User{Id: userID},
			Text: "Target message", ThreadId: threadID,
		})

		msgID := createTestMessage(t, tx, domain.MessageCreationData{
			Board: boardShortName, Author: domain.User{Id: userID},
			Text: "Message to get", ThreadId: threadID,
			ReplyTo: &domain.Replies{{To: targetMsgID, ToThreadId: threadID}},
		})

		// Add attachments
		attachments := getRandomAttachments(t)
		err := storage.addAttachments(tx, boardShortName, threadID, msgID, attachments)
		require.NoError(t, err)

		// Create a reply TO msgID
		_ = createTestMessage(t, tx, domain.MessageCreationData{
			Board: boardShortName, Author: domain.User{Id: userID},
			Text: "Reply to msgID", ThreadId: threadID,
			ReplyTo: &domain.Replies{{To: msgID, ToThreadId: threadID}},
		})

		msg, err := storage.getMessage(tx, boardShortName, threadID, msgID)
		require.NoError(t, err)
		assert.Equal(t, msgID, msg.Id)
		assert.Equal(t, "Message to get", msg.Text)
		require.Len(t, msg.Attachments, 2)
		require.Len(t, msg.Replies, 1)
		assert.Equal(t, msgID, msg.Replies[0].To)

		_, err = storage.getMessage(tx, boardShortName, threadID, -999)
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

		msgToDelete := createTestMessage(t, tx, domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: userID},
			Text:     "I will be deleted",
			ThreadId: threadID,
			ReplyTo:  &domain.Replies{{To: msgBeingRepliedTo, ToThreadId: threadID}},
		})

		// Add attachments to message that will be deleted
		attachments := getRandomAttachments(t)
		err := storage.addAttachments(tx, boardShortName, threadID, msgToDelete, attachments)
		require.NoError(t, err)

		msgReplyingToDeleted := createTestMessage(t, tx, domain.MessageCreationData{
			Board: boardShortName, Author: domain.User{Id: userID},
			Text: "My target will be deleted", ThreadId: threadID,
			ReplyTo: &domain.Replies{{To: msgToDelete, ToThreadId: threadID}},
		})

		_, err = storage.getMessage(tx, boardShortName, threadID, msgToDelete)
		require.NoError(t, err)
		attachmentsBefore, err := storage.getMessageAttachments(tx, boardShortName, threadID, msgToDelete)
		require.NoError(t, err)
		require.NotEmpty(t, attachmentsBefore)
		repliesToBefore, err := storage.getMessageRepliesTo(tx, boardShortName, threadID, msgToDelete)
		require.NoError(t, err)
		require.NotEmpty(t, repliesToBefore)
		repliesFromBefore, err := storage.getMessageRepliesFrom(tx, boardShortName, threadID, msgToDelete)
		require.NoError(t, err)
		require.NotEmpty(t, repliesFromBefore)

		err = storage.deleteMessage(tx, boardShortName, threadID, msgToDelete)
		require.NoError(t, err)

		_, err = storage.getMessage(tx, boardShortName, threadID, msgToDelete)
		requireNotFoundError(t, err)

		attachmentsAfter, err := storage.getMessageAttachments(tx, boardShortName, threadID, msgToDelete)
		require.NoError(t, err)
		assert.Empty(t, attachmentsAfter)

		// Verify files are deleted from files table
		for _, att := range attachmentsBefore {
			var count int
			err = tx.QueryRow("SELECT COUNT(*) FROM files WHERE id = $1", att.FileId).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 0, count, "File %d should be deleted from files table", att.FileId)
		}

		repliesToAfter, err := storage.getMessageRepliesTo(tx, boardShortName, threadID, msgToDelete)
		require.NoError(t, err)
		assert.Empty(t, repliesToAfter)

		repliesFromAfter, err := storage.getMessageRepliesFrom(tx, boardShortName, threadID, msgToDelete)
		require.NoError(t, err)
		assert.Empty(t, repliesFromAfter)

		_, err = storage.getMessage(tx, boardShortName, threadID, msgBeingRepliedTo)
		require.NoError(t, err)

		_, err = storage.getMessage(tx, boardShortName, threadID, msgReplyingToDeleted)
		require.NoError(t, err)

		t.Run("fails on invalid board or message", func(t *testing.T) {
			err := storage.deleteMessage(tx, "nonexistent", threadID, msgBeingRepliedTo)
			requireNotFoundError(t, err)

			err = storage.deleteMessage(tx, boardShortName, threadID, -1)
			requireNotFoundError(t, err)
		})
	})

	t.Run("AddAttachments", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName("badd")
		createTestBoard(t, tx, boardShortName)
		userID := createTestUser(t, tx, "adder@example.com")
		threadID, _ := createTestThread(t, tx, domain.ThreadCreationData{
			Title: "Add Attachments Test",
			Board: boardShortName,
			OpMessage: domain.MessageCreationData{
				Author: domain.User{Id: userID},
				Text:   "OP",
			},
		})

		t.Run("adds attachments to existing message", func(t *testing.T) {
			// Create message without attachments
			msgID := createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Message without attachments",
				ThreadId: threadID,
			})

			// Verify no attachments initially
			attachmentsBefore, err := storage.getMessageAttachments(tx, boardShortName, threadID, msgID)
			require.NoError(t, err)
			assert.Empty(t, attachmentsBefore)

			// Add attachments
			attachments := domain.Attachments{
				&domain.Attachment{
					Board:     boardShortName,
					ThreadId:  threadID,
					MessageId: msgID,
					File: &domain.File{
						FileCommonMetadata: domain.FileCommonMetadata{
							Filename:    "image1.jpg",
							SizeBytes:   1024,
							MimeType:    "image/jpeg",
							ImageWidth:  intPtr(800),
							ImageHeight: intPtr(600),
						},
						FilePath:         "tech/1/image1.jpg",
						OriginalFilename: "image1.jpg",
					},
				},
				&domain.Attachment{
					Board:     boardShortName,
					ThreadId:  threadID,
					MessageId: msgID,
					File: &domain.File{
						FileCommonMetadata: domain.FileCommonMetadata{
							Filename:    "image2.png",
							SizeBytes:   2048,
							MimeType:    "image/png",
							ImageWidth:  intPtr(1024),
							ImageHeight: intPtr(768),
						},
						FilePath:         "tech/1/image2.png",
						OriginalFilename: "image2.png",
					},
				},
			}

			err = storage.addAttachments(tx, boardShortName, threadID, msgID, attachments)
			require.NoError(t, err)

			// Verify attachments were added
			attachmentsAfter, err := storage.getMessageAttachments(tx, boardShortName, threadID, msgID)
			require.NoError(t, err)
			assert.Len(t, attachmentsAfter, 2)

			// Verify first attachment
			assert.Equal(t, "tech/1/image1.jpg", attachmentsAfter[0].File.FilePath)
			assert.Equal(t, "image1.jpg", attachmentsAfter[0].File.Filename)
			assert.Equal(t, "image1.jpg", attachmentsAfter[0].File.OriginalFilename)
			assert.Equal(t, int64(1024), attachmentsAfter[0].File.SizeBytes)
			assert.Equal(t, "image/jpeg", attachmentsAfter[0].File.MimeType)
			assert.NotNil(t, attachmentsAfter[0].File.ImageWidth)
			assert.Equal(t, 800, *attachmentsAfter[0].File.ImageWidth)

			// Verify second attachment
			assert.Equal(t, "tech/1/image2.png", attachmentsAfter[1].File.FilePath)
		})

		t.Run("can add more attachments to message that already has some", func(t *testing.T) {
			// Create message with attachments
			initialAttachments := getRandomAttachments(t)
			msgID := createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Message with initial attachments",
				ThreadId: threadID,
			})

			// Add initial attachments
			err := storage.addAttachments(tx, boardShortName, threadID, msgID, initialAttachments)
			require.NoError(t, err)
			initialCount := len(initialAttachments)

			// Create new attachments to add
			newAttachments := getRandomAttachments(t)
			err = storage.addAttachments(tx, boardShortName, threadID, msgID, newAttachments)
			require.NoError(t, err)

			// Verify total attachments
			attachmentsAfter, err := storage.getMessageAttachments(tx, boardShortName, threadID, msgID)
			require.NoError(t, err)
			assert.Len(t, attachmentsAfter, initialCount+len(newAttachments))
		})

		t.Run("handles video files without dimensions", func(t *testing.T) {
			msgID := createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Message for video",
				ThreadId: threadID,
			})

			attachments := domain.Attachments{
				&domain.Attachment{
					Board:     boardShortName,
					ThreadId:  threadID,
					MessageId: msgID,
					File: &domain.File{
						FileCommonMetadata: domain.FileCommonMetadata{
							Filename:    "video.mp4",
							SizeBytes:   10000,
							MimeType:    "video/mp4",
							ImageWidth:  nil,
							ImageHeight: nil,
						},
						FilePath:         "tech/1/video.mp4",
						OriginalFilename: "video.mp4",
					},
				},
			}

			err := storage.addAttachments(tx, boardShortName, threadID, msgID, attachments)
			require.NoError(t, err)

			retrieved, err := storage.getMessageAttachments(tx, boardShortName, threadID, msgID)
			require.NoError(t, err)
			assert.Len(t, retrieved, 1)
			assert.Nil(t, retrieved[0].File.ImageWidth)
			assert.Nil(t, retrieved[0].File.ImageHeight)
		})

		t.Run("fails with invalid message ID", func(t *testing.T) {
			attachments := domain.Attachments{
				&domain.Attachment{
					Board:     boardShortName,
					ThreadId:  threadID,
					MessageId: -999,
					File: &domain.File{
						FileCommonMetadata: domain.FileCommonMetadata{
							Filename:  "file.jpg",
							SizeBytes: 1000,
							MimeType:  "image/jpeg",
						},
						FilePath:         "tech/1/file.jpg",
						OriginalFilename: "file.jpg",
					},
				},
			}

			err := storage.addAttachments(tx, boardShortName, threadID, -999, attachments)
			require.Error(t, err)
		})
	})
}

func intPtr(i int) *int {
	return &i
}
