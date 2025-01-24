package pg

import (
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func TestCreateMessage(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t)

	// Test creating a message
	author := &domain.User{Id: 2}
	text := "Test message"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID, err := storage.CreateMessage(boardShortName, author, text, attachments, threadID)
	require.NoError(t, err, "CreateMessage should not return an error")
	assert.Greater(t, msgID, int64(0), "Message ID should not be greater than 0")

	// Test creating a message in a non-existent thread
	_, err = storage.CreateMessage(boardShortName, author, text, attachments, -1)
	requireNotFoundError(t, err)
}

func TestGetMessage(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t)

	author := &domain.User{Id: 2}
	text := "Test message"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID := createTestMessage(t, boardShortName, author, text, attachments, threadID)

	// Test getting the message
	msg, err := storage.GetMessage(msgID)
	require.NoError(t, err, "GetMessage should not return an error")
	assert.Equal(t, msgID, msg.Id, "Message ID mismatch")
	assert.Equal(t, author.Id, msg.Author.Id, "Author ID mismatch")
	assert.Equal(t, text, msg.Text, "Message text mismatch")
	assert.Equal(t, *attachments, *msg.Attachments, "Attachments mismatch")
	assert.Equal(t, threadID, msg.ThreadId.Int64, "Thread ID mismatch")

	// Test getting a non-existent message
	_, err = storage.GetMessage(-1)
	requireNotFoundError(t, err)
}

func TestDeleteMessage(t *testing.T) {
	boardShortName, _, msgID := setupBoardAndThreadAndMessage(t)

	// Test deleting the message
	err := storage.DeleteMessage(boardShortName, msgID)
	require.NoError(t, err, "DeleteMessage should not return an error")

	// Verify that the message text and attachments are updated
	updatedMsg, err := storage.GetMessage(msgID)
	require.NoError(t, err, "GetMessage should not return an error after deletion")
	assert.Equal(t, "msg deleted", updatedMsg.Text, "Message text was not updated")
	assert.Nil(t, updatedMsg.Attachments, "Attachments were not deleted")

	// Test deleting a non-existent message
	err = storage.DeleteMessage(boardShortName, -1)
	requireNotFoundError(t, err)
}
