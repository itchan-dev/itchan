package pg

import (
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	_ "github.com/lib/pq" // pg driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateMessage verifies message creation logic, including board activity updates.
func TestCreateMessage(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t) // Creates board & thread, handles cleanup

	// Get initial board state once *after* thread creation (as thread creation also updates activity)
	initialBoard, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err, "Setup: Failed to get initial board state for: %s", boardShortName)

	author := domain.User{Id: 2} // Use value type directly as required by MessageCreationData
	text := "Test message for create"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}

	// --- Subtest: Successful Creation and Verification ---
	t.Run("Successful Creation and Verification", func(t *testing.T) {
		// Add a small delay to ensure the new timestamp is distinct
		time.Sleep(50 * time.Millisecond)
		timeBeforeCreate := time.Now().UTC()

		// Prepare creation data
		creationData := domain.MessageCreationData{
			Board:       boardShortName,
			Author:      author,
			Text:        text,
			Attachments: attachments,
			ThreadId:    threadID,
		}

		// Perform creation
		msgID, createErr := storage.CreateMessage(creationData, false, nil)
		require.NoError(t, createErr, "CreateMessage should not return an error")
		require.Greater(t, msgID, int64(0), "Message ID should be greater than 0")

		// Verify created message details (GetMessage retrieves from main table)
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
		assert.True(t, createdMsg.CreatedAt.Equal(createdMsg.ModifiedAt), "CreatedAt [%v] should equal ModifiedAt [%v] on creation", createdMsg.CreatedAt, createdMsg.ModifiedAt)
		// Ensure timestamps are reasonably close and ordered correctly
		assert.True(t, timeBeforeCreate.Before(createdMsg.CreatedAt) || timeBeforeCreate.Equal(createdMsg.CreatedAt), "timeBeforeCreate [%v] should be before or equal to CreatedAt [%v]", timeBeforeCreate, createdMsg.CreatedAt)
		assert.WithinDuration(t, timeBeforeCreate, createdMsg.CreatedAt, 2*time.Second, "CreatedAt should be very close to timeBeforeCreate")

		// Verify board's last_activity update
		boardAfterCreate, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after create for: %s", boardShortName)

		// Note: initialBoard.LastActivity reflects thread creation time
		assert.True(t, boardAfterCreate.LastActivity.After(initialBoard.LastActivity), "Board last_activity [%v] should be after initial activity [%v]", boardAfterCreate.LastActivity, initialBoard.LastActivity)
		// The refactored code sets board last_activity = message createdTs
		assert.Equal(t, createdMsg.CreatedAt.UTC(), boardAfterCreate.LastActivity.UTC(), "Board last_activity [%v] should equal message CreatedAt [%v]", boardAfterCreate.LastActivity, createdMsg.CreatedAt)
	})

	t.Run("Successful Creation of op msg (num_replies remain the same)", func(t *testing.T) {
		// Get thread before new message
		threadBefore, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err)
		// Prepare creation data
		creationData := domain.MessageCreationData{
			Board:       boardShortName,
			Author:      author,
			Text:        "Test num_replies",
			Attachments: nil,
			ThreadId:    threadID,
		}
		// Perform creation
		_, createErr := storage.CreateMessage(creationData, true, nil)
		require.NoError(t, createErr, "CreateMessage should not return an error")
		// Get thread after message creation
		threadAfter, err := storage.GetThread(boardShortName, threadID)
		require.NoError(t, err)

		assert.Equal(t, threadBefore.NumReplies, threadAfter.NumReplies, "Op msg shouldnt increment num replies")
		// Test num_replies incrmenet if not op msg
		_, createErr = storage.CreateMessage(creationData, false, nil)
		require.NoError(t, createErr, "CreateMessage should not return an error")
		// Get thread after second message creation
		threadAfter, err = storage.GetThread(boardShortName, threadID)
		require.NoError(t, err)

		assert.Equal(t, threadBefore.NumReplies+1, threadAfter.NumReplies, "Non-op msg shouldnt increment num replies")
	})

	// --- Subtest: Failure Case - Non-existent Thread ---
	t.Run("Failure - Non-existent Thread", func(t *testing.T) {
		// Get board state *before* this specific test run
		boardBeforeFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state before non-existent thread test")

		// Prepare creation data with invalid thread ID
		creationData := domain.MessageCreationData{
			Board:       boardShortName,
			Author:      author,
			Text:        text,
			Attachments: attachments,
			ThreadId:    -1, // Non-existent thread ID
		}

		// Attempt creation
		_, err = storage.CreateMessage(creationData, false, nil)
		requireNotFoundError(t, err) // Check it's the expected 404 error

		// Verify board activity did *not* change (transaction rollback)
		boardAfterFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after failed create (non-existent thread)")

		assert.Equal(t, boardBeforeFail.LastActivity.UTC(), boardAfterFail.LastActivity.UTC(), "Board last_activity [%v] should not change on failed CreateMessage (non-existent thread), expected [%v]", boardAfterFail.LastActivity, boardBeforeFail.LastActivity)
	})

	// --- Subtest: Failure Case - Non-existent Board ---
	t.Run("Failure - Non-existent Board", func(t *testing.T) {
		// Prepare creation data with invalid board name
		creationData := domain.MessageCreationData{
			Board:       "nonexistentboard", // Non-existent board
			Author:      author,
			Text:        text,
			Attachments: attachments,
			ThreadId:    threadID, // Valid thread ID, but board doesn't exist
		}

		_, err := storage.CreateMessage(creationData, false, nil)
		requireNotFoundError(t, err)
		// No need to check activity on the *valid* board, as the operation targets a non-existent one.
	})

	// Test for custom CreatedAt timestamp
	t.Run("Successful Creation with Custom Timestamp", func(t *testing.T) {
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

// TestGetMessage verifies retrieving a specific message.
func TestGetMessage(t *testing.T) {
	boardShortName, threadID := setupBoardAndThread(t)

	author := domain.User{Id: 2}
	text := "Test message for get"
	attachments := &domain.Attachments{"file1.jpg", "file2.png"}
	msgID := createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: author, Text: text, Attachments: attachments, ThreadId: threadID})

	t.Run("Get existing message", func(t *testing.T) {
		msg, err := storage.GetMessage(boardShortName, msgID)
		require.NoError(t, err, "GetMessage should not return an error")
		assert.Equal(t, msgID, msg.Id, "Message ID mismatch")
		assert.Equal(t, author.Id, msg.Author.Id, "Author ID mismatch")
		assert.Equal(t, text, msg.Text, "Message text mismatch")
		require.NotNil(t, msg.Attachments, "Attachments should not be nil")
		assert.Equal(t, *attachments, *msg.Attachments, "Attachments mismatch")
		require.NotNil(t, msg.ThreadId, "Thread ID should not be nil")
		assert.Equal(t, threadID, msg.ThreadId, "Thread ID mismatch") // Dereference pointer
		assert.False(t, msg.CreatedAt.IsZero(), "CreatedAt should not be zero")
		assert.False(t, msg.ModifiedAt.IsZero(), "Modified should not be zero")
		assert.Equal(t, msg.CreatedAt.UTC(), msg.ModifiedAt.UTC(), "ModifiedAt [%v] should equal CreatedAt [%v] initially", msg.ModifiedAt, msg.CreatedAt)
	})

	t.Run("Get non-existent message", func(t *testing.T) {
		_, err := storage.GetMessage(boardShortName, -1)
		requireNotFoundError(t, err)
	})
}

// TestDeleteMessage verifies message deletion logic (soft delete) and board activity updates.
func TestDeleteMessage(t *testing.T) {
	// --- Shared Setup ---
	// Use helper that creates board, thread, and a message
	boardShortName, _, msgID := setupBoardAndThreadAndMessage(t)

	// Get initial states *after* setup (which includes message creation)
	originalMsg, err := storage.GetMessage(boardShortName, msgID)
	require.NoError(t, err, "Setup: Failed to get original message right after creation")

	// Get board state *after* the message creation in setup
	boardAfterCreate, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err, "Setup: Failed to get board state after setup create for: %s", boardShortName)

	// --- Subtest: Successful Deletion and State Verification ---
	t.Run("Successful Deletion and State Verification", func(t *testing.T) {
		// Add a delay to ensure the delete timestamp is noticeably different
		time.Sleep(50 * time.Millisecond)
		timeBeforeDelete := time.Now().UTC() // Record time just before delete

		// Perform delete (signature matches refactored code)
		deleteErr := storage.DeleteMessage(boardShortName, msgID)
		require.NoError(t, deleteErr, "DeleteMessage should not return an error")

		// Verify message state after deletion (soft delete)
		updatedMsg, err := storage.GetMessage(boardShortName, msgID)
		require.NoError(t, err, "GetMessage should still retrieve the message after soft delete")
		assert.Equal(t, "deleted", updatedMsg.Text, "Message text should be updated to 'deleted'")
		assert.Nil(t, updatedMsg.Attachments, "Attachments should be set to nil") // Check explicitly for nil
		require.False(t, updatedMsg.ModifiedAt.IsZero(), "ModifiedAt timestamp should not be the zero time after update")
		assert.True(t, updatedMsg.ModifiedAt.After(originalMsg.ModifiedAt),
			"Updated ModifiedAt [%v] should be after the original ModifiedAt [%v]", updatedMsg.ModifiedAt, originalMsg.ModifiedAt)
		// CreatedAt should remain unchanged
		assert.Equal(t, originalMsg.CreatedAt.UTC(), updatedMsg.CreatedAt.UTC(), "CreatedAt timestamp [%v] should not change on delete, expected [%v]", updatedMsg.CreatedAt, originalMsg.CreatedAt)
		// ModifiedAt should be close to the time the delete happened
		assert.WithinDuration(t, timeBeforeDelete, updatedMsg.ModifiedAt, 2*time.Second,
			"Updated ModifiedAt timestamp [%v] is not recent compared to delete time [%v]", updatedMsg.ModifiedAt, timeBeforeDelete)

		// Verify board state after deletion
		boardAfterDelete, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after delete for: %s", boardShortName)

		assert.True(t, boardAfterDelete.LastActivity.After(boardAfterCreate.LastActivity),
			"Board last_activity after delete [%v] should be after activity after create [%v]", boardAfterDelete.LastActivity, boardAfterCreate.LastActivity)
		// The refactored code sets board last_activity = deletion timestamp
		assert.Equal(t, updatedMsg.ModifiedAt.UTC(), boardAfterDelete.LastActivity.UTC(), "Board LastActivity [%v] should be equal to the message's updated ModifiedAt [%v]", boardAfterDelete.LastActivity, updatedMsg.ModifiedAt)
	})

	// --- Subtest: Failure Case - Non-existent Message ---
	t.Run("Failure - Non-existent Message", func(t *testing.T) {
		// Get board state *before* this specific test run (it might have been updated by the successful delete test)
		boardBeforeFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state before non-existent message test")

		// Attempt deletion of a non-existent message ID
		err = storage.DeleteMessage(boardShortName, -1)
		requireNotFoundError(t, err) // Expect 404 because message update affects 0 rows

		// Verify board activity did *not* change (transaction rollback)
		boardAfterFail, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "Failed to get board state after failed delete (non-existent message)")

		// Use Equal for timestamp comparison
		assert.Equal(t, boardBeforeFail.LastActivity.UTC(), boardAfterFail.LastActivity.UTC(), "Board last_activity [%v] should not change on failed DeleteMessage (non-existent message), expected [%v]", boardAfterFail.LastActivity, boardBeforeFail.LastActivity)
	})

	// --- Subtest: Failure Case - Non-existent Board ---
	t.Run("Failure - Non-existent Board", func(t *testing.T) {
		// Attempt deletion using a valid message ID but a non-existent board short name
		err = storage.DeleteMessage("nonexistentboard", msgID)
		requireNotFoundError(t, err) // Expect 404 because board update affects 0 rows
		// No need to check activity on the *valid* board, as the operation targets a non-existent one.
	})
}
