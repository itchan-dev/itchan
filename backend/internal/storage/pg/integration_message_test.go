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
	// --- Shared Setup for all CreateMessage subtests ---
	boardShortName, threadID := setupBoardAndThread(t) // Assume this creates the board

	// Get initial board state once
	initialBoard, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err, "Setup: Failed to get initial board state for: %s", boardShortName)

	author := &domain.User{Id: 2}
	text := "Test message for create"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}

	// --- Subtest: Successful Creation and Verification ---
	t.Run("Successful Creation and Verification", func(t *testing.T) {
		// Add a small delay to ensure the new timestamp is distinct
		time.Sleep(50 * time.Millisecond)
		timeBeforeCreate := time.Now()

		// Perform creation
		var createErr error
		msgID, createErr := storage.CreateMessage(boardShortName, author, text, attachments, threadID)
		require.NoError(t, createErr, "CreateMessage should not return an error")
		require.Greater(t, msgID, int64(0), "Message ID should be greater than 0")

		// Verify created message details
		createdMsg, err := storage.GetMessage(msgID)
		require.NoError(t, err, "Failed to get created message")
		assert.Equal(t, text, createdMsg.Text, "Message text mismatch")
		assert.Equal(t, *attachments, *createdMsg.Attachments, "Attachments mismatch")
		assert.Equal(t, author.Id, createdMsg.Author.Id, "Author ID mismatch")
		assert.Equal(t, threadID, createdMsg.ThreadId.Int64, "Thread ID mismatch")
		assert.False(t, createdMsg.CreatedAt.IsZero(), "CreatedAt should not be zero")
		assert.False(t, createdMsg.ModifiedAt.IsZero(), "ModifiedAt should not be zero")
		assert.True(t, initialBoard.LastActivity.Before(createdMsg.CreatedAt), "LastActivity should be before CreatedAt")
		assert.True(t, createdMsg.CreatedAt.Equal(createdMsg.ModifiedAt), "CreatedAt should equal ModifiedAt on creation")
		assert.True(t, timeBeforeCreate.Before(createdMsg.CreatedAt))

		// Verify board's last_activity update
		boardAfterCreate, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after create for: %s", boardShortName)

		assert.True(t, boardAfterCreate.LastActivity.After(initialBoard.LastActivity), "Board last_activity [%v] should be after initial activity [%v]", boardAfterCreate.LastActivity, initialBoard.LastActivity)
		assert.Equal(t, boardAfterCreate.LastActivity, createdMsg.CreatedAt)
	})

	// --- Subtest: Failure Case - Non-existent Thread ---
	t.Run("Failure - Non-existent Thread", func(t *testing.T) {
		// Get board state *after* the successful create (if it ran)
		boardBeforeFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state before non-existent thread test")

		// Attempt creation
		_, err = storage.CreateMessage(boardShortName, author, text, attachments, -1) // Non-existent thread ID
		requireNotFoundError(t, err)                                                  // Check it's the expected 404 error

		// Verify board activity did *not* change
		boardAfterFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after failed create (non-existent thread)")

		assert.Equal(t, boardBeforeFail.LastActivity, boardAfterFail.LastActivity, "Board last_activity [%v] should not change on failed CreateMessage (non-existent thread), expected [%v]", boardAfterFail.LastActivity, boardBeforeFail.LastActivity)
	})

	// --- Subtest: Failure Case - Non-existent Board ---
	t.Run("Failure - Non-existent Board", func(t *testing.T) {
		_, err := storage.CreateMessage("nonexistentboard", author, text, attachments, threadID)
		requireNotFoundError(t, err)
	})
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
	// --- Shared Setup for all DeleteMessage subtests ---
	boardShortName, _, msgID := setupBoardAndThreadAndMessage(t)

	// Get initial states *after* setup (which includes message creation)
	originalMsg, err := storage.GetMessage(msgID)
	require.NoError(t, err, "Setup: Failed to get original message right after creation")

	boardAfterCreate, err := storage.GetBoard(boardShortName, 1) // Get state set by CreateMessage in setup
	require.NoError(t, err, "Setup: Failed to get board state after setup create for: %s", boardShortName)

	// --- Subtest: Successful Deletion and State Verification ---
	t.Run("Successful Deletion and State Verification", func(t *testing.T) {
		// Add a delay to ensure the delete timestamp is noticeably different
		time.Sleep(50 * time.Millisecond)
		timeBeforeDelete := time.Now().UTC() // Record time just before delete

		// Perform delete
		deleteErr := storage.DeleteMessage(boardShortName, msgID)
		require.NoError(t, deleteErr, "DeleteMessage should not return an error")

		// Verify message state after deletion
		updatedMsg, err := storage.GetMessage(msgID)
		require.NoError(t, err, "GetMessage should not return an error after deletion")
		assert.Equal(t, "msg deleted", updatedMsg.Text, "Message text was not updated")
		assert.Nil(t, updatedMsg.Attachments, "Attachments were not set to nil")
		require.False(t, updatedMsg.ModifiedAt.IsZero(), "Modified timestamp should not be the zero time after update")
		assert.True(t, updatedMsg.ModifiedAt.After(originalMsg.ModifiedAt),
			"Updated Modified time [%v] should be after the *original* Modified time [%v]", updatedMsg.ModifiedAt, originalMsg.ModifiedAt)
		assert.True(t, originalMsg.CreatedAt.Equal(updatedMsg.CreatedAt), "CreatedAt timestamp [%v] should not change on delete, expected [%v]", updatedMsg.CreatedAt, originalMsg.CreatedAt)
		assert.WithinDuration(t, timeBeforeDelete, updatedMsg.ModifiedAt, 5*time.Second,
			"Updated Modified timestamp [%v] is not recent compared to delete time [%v]", updatedMsg.ModifiedAt, timeBeforeDelete)

		// Verify board state after deletion
		boardAfterDelete, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after delete for: %s", boardShortName)

		assert.True(t, boardAfterDelete.LastActivity.After(boardAfterCreate.LastActivity),
			"Board last_activity after delete [%v] should be after activity after create [%v]", boardAfterDelete.LastActivity, boardAfterCreate.LastActivity)
		assert.Equal(t, boardAfterDelete.LastActivity, updatedMsg.ModifiedAt, "boardAfterDelete.LastActivity should be equal to modified time in deleted msg")
	})

	// --- Subtest: Failure Case - Non-existent Message ---
	t.Run("Failure - Non-existent Message", func(t *testing.T) {
		// Get board state *after* the successful delete
		boardBeforeFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state before non-existent message test")

		// Attempt deletion of a non-existent message ID
		err = storage.DeleteMessage(boardShortName, -1)
		requireNotFoundError(t, err)

		// Verify board activity did *not* change
		boardAfterFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after failed delete (non-existent message)")

		assert.True(t, boardBeforeFail.LastActivity.Equal(boardAfterFail.LastActivity), "Board last_activity [%v] should not change on failed DeleteMessage (non-existent message), expected [%v]", boardBeforeFail.LastActivity, boardAfterFail.LastActivity)
	})

	// --- Subtest: Failure Case - Non-existent Board ---
	t.Run("Failure - Non-existent Board", func(t *testing.T) {
		// Attempt deletion on a non-existent board
		err = storage.DeleteMessage("nonexistentboard", msgID) // Use the original msgID, but wrong board
		requireNotFoundError(t, err)
		// No need to check activity on a non-existent board
	})
}
