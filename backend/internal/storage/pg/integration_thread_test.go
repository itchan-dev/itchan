package pg

import (
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func TestCreateThread(t *testing.T) {
	// Create a board for testing
	boardShortName := setupBoard(t)

	// Test creating a thread
	title := "Test Thread"
	msg := &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP", Attachments: &domain.Attachments{"file1.jpg"}}
	threadID := createTestThread(t, boardShortName, title, msg)

	assert.Greater(t, threadID, int64(0), "Thread ID should be greater 0")
}

func TestGetThread(t *testing.T) {
	boardShortName := setupBoard(t)

	title := "Test Thread"
	opMsg := &domain.Message{Author: domain.User{Id: 1}, Text: "Test OP", Attachments: &domain.Attachments{"file1.jpg"}}
	threadID := createTestThread(t, boardShortName, title, opMsg)

	// Add some messages to the thread
	replyMsgs := []struct {
		author *domain.User
		text   string
	}{
		{author: &domain.User{Id: 2}, text: "Reply 1"},
		{author: &domain.User{Id: 3}, text: "Reply 2"},
	}
	for _, m := range replyMsgs {
		createTestMessage(t, boardShortName, m.author, m.text, nil, threadID)
	}

	// Test getting the thread
	thread, err := storage.GetThread(threadID)
	require.NoError(t, err, "GetThread should not return an error")
	assert.Equal(t, title, thread.Title, "Thread title mismatch")
	assert.Equal(t, boardShortName, thread.Board, "Thread board mismatch")
	assert.Equal(t, 2, thread.NumReplies, "Number of replies mismatch")
	assert.Len(t, thread.Messages, 3, "Expected 3 messages (OP + 2 replies)")
	requireMessageOrder(t, thread.Messages, []string{opMsg.Text, replyMsgs[0].text, replyMsgs[1].text})
	assert.Equal(t, *opMsg.Attachments, *thread.Messages[0].Attachments, "OP message attachments mismatch")

	// Test getting a non-existent thread
	_, err = storage.GetThread(-1)
	requireNotFoundError(t, err)
}

func TestDeleteThread(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t)

	// Test deleting the thread
	err := storage.DeleteThread(boardShortName, threadID)
	require.NoError(t, err, "DeleteThread should not return an error")

	// Test getting the deleted thread
	_, err = storage.GetThread(threadID)
	requireNotFoundError(t, err)

	// Test deleting a non-existent thread
	err = storage.DeleteThread(boardShortName, -1)
	requireNotFoundError(t, err)
}

func TestBumpLimit(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t)

	// Send messages up to the bump limit
	for i := 0; i <= storage.cfg.Public.BumpLimit; i++ {
		createTestMessage(t, boardShortName, &domain.User{Id: 2}, "Test message", nil, threadID)
	}

	// Get the thread and check the last bump timestamp
	thread, err := storage.GetThread(threadID)
	require.NoError(t, err)
	lastBumpTsBefore := thread.LastBumped

	// Send one more message (over the bump limit)
	createTestMessage(t, boardShortName, &domain.User{Id: 2}, "Test message over bump limit", nil, threadID)

	// Get the thread again and check that the last bump timestamp hasn't changed
	thread, err = storage.GetThread(threadID)
	require.NoError(t, err)
	lastBumpTsAfter := thread.LastBumped

	assert.Equal(t, lastBumpTsBefore, lastBumpTsAfter, "Last bump timestamp should not change after going over the bump limit")
}
