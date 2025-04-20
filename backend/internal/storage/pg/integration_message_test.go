package pg

import (
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.False(t, msg.CreatedAt.IsZero(), "CreatedAt should not be zero")
	assert.False(t, msg.ModifiedAt.IsZero(), "Modified should not be zero")
	assert.Equal(t, msg.ModifiedAt, msg.CreatedAt, "ModifiedAt != CreatedAt")

	// Test getting a non-existent message
	_, err = storage.GetMessage(-1)
	requireNotFoundError(t, err)
}

func TestDeleteMessage(t *testing.T) {
	boardShortName, _, msgID := setupBoardAndThreadAndMessage(t) // Assuming this helper creates a valid message

	// 1. Get the state *immediately after* creation
	originalMsg, err := storage.GetMessage(msgID)
	require.NoError(t, err, "Failed to get original message right after creation")
	originalModifiedTime := originalMsg.ModifiedAt
	time.Sleep(50 * time.Millisecond)

	// 2. Perform the delete operation
	err = storage.DeleteMessage(boardShortName, msgID)
	require.NoError(t, err, "DeleteMessage should not return an error")

	// Record time just before fetching the updated record
	fetchTime := time.Now().UTC()

	// 3. Get the state *after* deletion
	updatedMsg, err := storage.GetMessage(msgID)
	require.NoError(t, err, "GetMessage should not return an error after deletion")

	assert.Equal(t, "msg deleted", updatedMsg.Text, "Message text was not updated")
	assert.Nil(t, updatedMsg.Attachments, "Attachments were not set to nil") // Or assert empty slice if appropriate

	// Assert modified timestamp behavior
	require.False(t, updatedMsg.ModifiedAt.IsZero(), "Modified timestamp should not be the zero time after update")

	// The new modified time MUST be after the original creation time.
	assert.True(t, updatedMsg.ModifiedAt.After(originalMsg.CreatedAt),
		"Updated Modified time [%v] should be after CreatedAt time [%v]", updatedMsg.ModifiedAt, originalMsg.CreatedAt)

	// The new modified time MUST be after the original modified time.
	assert.True(t, updatedMsg.ModifiedAt.After(originalModifiedTime),
		"Updated Modified time [%v] should be after the *original* Modified time [%v]", updatedMsg.ModifiedAt, originalModifiedTime)

	// Check if the modified time is reasonably close to the time the delete operation likely occurred.
	// Allow for some small delay (e.g., 5 seconds) between DB update and test check.
	assert.WithinDuration(t, fetchTime, updatedMsg.ModifiedAt, 5*time.Second,
		"Updated Modified timestamp [%v] is not recent compared to fetch time [%v]", updatedMsg.ModifiedAt, fetchTime)

	// Assert that CreatedAt timestamp was NOT changed by the delete operation
	assert.Equal(t, originalMsg.CreatedAt, updatedMsg.CreatedAt,
		"CreatedAt timestamp should not change on delete")

	// 4. Test deleting a non-existent message
	err = storage.DeleteMessage(boardShortName, -1) // Use a non-existent ID
	requireNotFoundError(t, err)                    // Assumes requireNotFoundError checks for the specific error type/status code
}
