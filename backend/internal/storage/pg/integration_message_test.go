package pg

import (
	"testing"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func TestCreateMessage(t *testing.T) {
	// Create a board and thread for testing
	boardName := "testboard"
	boardShortName := "tb"
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")
	threadID, err := storage.CreateThread("Test Thread", boardShortName, &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP"})
	require.NoError(t, err, "CreateThread should not return an error")

	// Test creating a message
	author := &domain.User{Id: 2}
	text := "Test message"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID, err := storage.CreateMessage(boardShortName, author, text, attachments, threadID)
	require.NoError(t, err, "CreateMessage should not return an error")
	assert.NotEqual(t, int64(-1), msgID, "Message ID should not be -1")

	// Test creating a message in a non-existent thread
	_, err = storage.CreateMessage(boardShortName, author, text, attachments, -1)
	require.Error(t, err, "CreateMessage should return an error for non-existent thread")
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "DeleteBoard should not return an error")
}

func TestGetMessage(t *testing.T) {
	// Create a board, thread, and message for testing
	boardName := "testboard"
	boardShortName := "tb"
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")
	threadID, err := storage.CreateThread("Test Thread", boardShortName, &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP"})
	require.NoError(t, err, "CreateThread should not return an error")
	author := &domain.User{Id: 2}
	text := "Test message"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID, err := storage.CreateMessage(boardShortName, author, text, attachments, threadID)
	require.NoError(t, err, "CreateMessage should not return an error")

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
	require.Error(t, err, "GetMessage should return an error for non-existent message")
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "DeleteBoard should not return an error")
}

func TestDeleteMessage(t *testing.T) {
	// Create a board, thread, and message for testing
	boardName := "testboard"
	boardShortName := "tb"
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")
	threadID, err := storage.CreateThread("Test Thread", boardShortName, &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP"})
	require.NoError(t, err, "CreateThread should not return an error")
	author := &domain.User{Id: 2}
	text := "Test message"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID, err := storage.CreateMessage(boardShortName, author, text, attachments, threadID)
	require.NoError(t, err, "CreateMessage should not return an error")

	// Test deleting the message
	err = storage.DeleteMessage(boardShortName, msgID)
	require.NoError(t, err, "DeleteMessage should not return an error")

	// Verify that the message text and attachments are updated
	updatedMsg, err := storage.GetMessage(msgID)
	require.NoError(t, err, "GetMessage should not return an error after deletion")
	assert.Equal(t, "msg deleted", updatedMsg.Text, "Message text was not updated")
	assert.Nil(t, updatedMsg.Attachments, "Attachments were not deleted")

	// Test deleting a non-existent message
	err = storage.DeleteMessage(boardShortName, -1)
	require.Error(t, err, "DeleteMessage should return an error for non-existent message")
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "DeleteBoard should not return an error")
}

func TestBumpLimit(t *testing.T) {
	// Create a board and thread
	boardName := "testboard"
	boardShortName := "tb"
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err)
	threadID, err := storage.CreateThread("Test Thread", boardShortName, &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP"})
	require.NoError(t, err)

	// Send messages up to the bump limit
	for i := 0; i <= storage.cfg.BumpLimit; i++ {
		_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 2}, "Test message", nil, threadID)
		require.NoError(t, err)
	}

	// Get the thread and check the last bump timestamp
	thread, err := storage.GetThread(threadID)
	require.NoError(t, err)
	lastBumpTsBefore := thread.LastBumped

	// Send one more message (over the bump limit)
	_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 2}, "Test message over bump limit", nil, threadID)
	require.NoError(t, err)

	// Get the thread again and check that the last bump timestamp hasn't changed
	thread, err = storage.GetThread(threadID)
	require.NoError(t, err)
	lastBumpTsAfter := thread.LastBumped

	assert.Equal(t, lastBumpTsBefore, lastBumpTsAfter, "Last bump timestamp should not change after going over the bump limit")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err)
}
