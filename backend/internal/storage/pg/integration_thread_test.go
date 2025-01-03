package pg

import (
	"testing"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func TestCreateThread(t *testing.T) {
	// Create a board for testing
	boardName := "testboard"
	boardShortName := "tb"
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")

	// Test creating a thread
	title := "Test Thread"
	msg := &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP", Attachments: &domain.Attachments{"file1.jpg"}}
	threadID, err := storage.CreateThread(title, boardShortName, msg)
	require.NoError(t, err, "CreateThread should not return an error")
	assert.NotEqual(t, int64(-1), threadID, "Thread ID should not be -1")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "DeleteBoard should not return an error")
}

func TestGetThread(t *testing.T) {
	// Create a board and thread for testing
	boardName := "testboard"
	boardShortName := "tb"
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")
	title := "Test Thread"
	opMsg := &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP", Attachments: &domain.Attachments{"file1.jpg"}}
	threadID, err := storage.CreateThread(title, boardShortName, opMsg)
	require.NoError(t, err, "CreateThread should not return an error")

	// Add some messages to the thread
	replyMsgs := []struct {
		author *domain.User
		text   string
	}{
		{author: &domain.User{Id: 2}, text: "Reply 1"},
		{author: &domain.User{Id: 3}, text: "Reply 2"},
	}
	for _, m := range replyMsgs {
		_, err = storage.CreateMessage(boardShortName, m.author, m.text, nil, threadID)
		require.NoError(t, err, "CreateMessage should not return an error")
	}

	// Test getting the thread
	thread, err := storage.GetThread(threadID)
	require.NoError(t, err, "GetThread should not return an error")
	assert.Equal(t, title, thread.Title, "Thread title mismatch")
	assert.Equal(t, boardShortName, thread.Board, "Thread board mismatch")
	assert.Equal(t, uint(2), thread.NumReplies, "Number of replies mismatch")
	assert.Len(t, thread.Messages, 3, "Expected 3 messages (OP + 2 replies)")
	assert.Equal(t, opMsg.Text, thread.Messages[0].Text, "OP message text mismatch")
	assert.Equal(t, *opMsg.Attachments, *thread.Messages[0].Attachments, "OP message attachments mismatch")
	assert.Equal(t, replyMsgs[0].text, thread.Messages[1].Text, "Reply 1 text mismatch")
	assert.Equal(t, replyMsgs[1].text, thread.Messages[2].Text, "Reply 2 text mismatch")

	// Test getting a non-existent thread
	_, err = storage.GetThread(-1)
	require.Error(t, err, "GetThread should return an error for non-existent thread")
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "DeleteBoard should not return an error")
}

func TestDeleteThread(t *testing.T) {
	// Create a board and thread for testing
	boardName := "testboard"
	boardShortName := "tb"
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")
	title := "Test Thread"
	msg := &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP"}
	threadID, err := storage.CreateThread(title, boardShortName, msg)
	require.NoError(t, err, "CreateThread should not return an error")

	// Test deleting the thread
	err = storage.DeleteThread(boardShortName, threadID)
	require.NoError(t, err, "DeleteThread should not return an error")

	// Test getting the deleted thread
	_, err = storage.GetThread(threadID)
	require.Error(t, err, "GetThread should return an error for deleted thread")
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Test deleting a non-existent thread
	err = storage.DeleteThread(boardShortName, -1)
	require.Error(t, err, "DeleteThread should return an error for non-existent thread")
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "DeleteBoard should not return an error")
}
