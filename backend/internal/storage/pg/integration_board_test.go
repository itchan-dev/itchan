package pg

import (
	"testing"
	"time"

	"math/rand"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/require"
)

func TestCreateBoard(t *testing.T) {
	boardName := "testboard1"
	boardShortName := "tb1"

	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")

	// Try to create a board with the same short name
	err = storage.CreateBoard(boardName, boardShortName)
	require.Error(t, err, "Expected error when creating duplicate board")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "Failed to clean up after test")
}

func TestGetBoard(t *testing.T) {
	boardName := "testboard2"
	boardShortName := "tb2"

	// Create a board
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")

	// Get the board
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err, "GetBoard should not return an error")
	require.Equal(t, boardName, board.Name, "Expected board name %s, got %s", boardName, board.Name)
	require.Equal(t, boardShortName, board.ShortName, "Expected board short name %s, got %s", boardShortName, board.ShortName)

	// Try to get a non-existent board
	_, err = storage.GetBoard("nonexistent", 1)
	require.Error(t, err, "Expected error when getting non-existent board")
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	require.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "Failed to clean up after test")
}

func TestDeleteBoard(t *testing.T) {
	boardName := "testboard3"
	boardShortName := "tb3"

	// Create a board
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")

	// Create thread and message
	thread_id, err := storage.CreateThread("test_title", boardShortName, &domain.Message{Author: domain.User{Id: 229}, Text: "test text"})
	require.NoError(t, err, "CreateThread should not return an error")

	msg_id, err := storage.CreateMessage(boardShortName, &domain.User{Id: 229}, "test text 2", nil, thread_id)
	require.NoError(t, err, "CreateMessage should not return an error")

	// Delete the board
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "DeleteBoard should not return an error")

	// Check that thread also deleted
	_, err = storage.GetThread(thread_id)
	require.Error(t, err, "Expected error when getting thread from non-existent board")
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	require.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Check that messages deleted
	msg, err := storage.GetMessage(thread_id)
	require.Error(t, err, "Expected error when getting message from non-existent board")
	require.Nil(t, msg, "Expected message to be nil")
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	require.Equal(t, 404, e.StatusCode, "Expected status code 404")

	_, err = storage.GetMessage(msg_id)
	require.Error(t, err, "Expected error when getting message from non-existent board")
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	require.Equal(t, 404, e.StatusCode, "Expected status code 404")

	// Try to delete a non-existent board
	err = storage.DeleteBoard("nonexistent")
	require.Error(t, err, "Expected error when deleting non-existent board")
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	require.Equal(t, 404, e.StatusCode, "Expected status code 404")
}

func TestBoardWorkflow(t *testing.T) {
	boardName := "testboard4"
	boardShortName := "tb4"

	// Create a board
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")

	// Get the board
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err, "GetBoard should not return an error")
	require.Equal(t, boardName, board.Name, "Expected board name %s, got %s", boardName, board.Name)
	require.Equal(t, boardShortName, board.ShortName, "Expected board short name %s, got %s", boardShortName, board.ShortName)

	// Create several threads
	_, err = storage.CreateThread("thread4", boardShortName, &domain.Message{Author: domain.User{Id: 229}, Text: "text 10"})
	require.NoError(t, err, "CreateThread should not return an error")
	_, err = storage.CreateThread("thread1", boardShortName, &domain.Message{Author: domain.User{Id: 228}, Text: "text 1"})
	require.NoError(t, err, "CreateThread should not return an error")
	thread2, err := storage.CreateThread("thread2", boardShortName, &domain.Message{Author: domain.User{Id: 229}, Text: "text 2"})
	require.NoError(t, err, "CreateThread should not return an error")
	thread3, err := storage.CreateThread("thread3", boardShortName, &domain.Message{Author: domain.User{Id: 229}, Text: "text 6"})
	require.NoError(t, err, "CreateThread should not return an error")

	// Fill threads with messages
	messagesSample := []struct {
		text        string
		author      *domain.User
		attachments domain.Attachments
		threadId    int64
	}{
		{
			text:        "text 4",
			author:      &domain.User{Id: 228},
			attachments: nil,
			threadId:    thread2,
		},
		{
			text:        "text 5",
			author:      &domain.User{Id: 300},
			attachments: domain.Attachments{"file.txt", "file2.png"},
			threadId:    thread2,
		},
		{
			text:        "text 7",
			author:      &domain.User{Id: 301},
			attachments: nil,
			threadId:    thread3,
		},
		{
			text:        "text 8",
			author:      &domain.User{Id: 303},
			attachments: domain.Attachments{"file3.txt", "file4.png"},
			threadId:    thread2,
		},
	}
	for _, msg := range messagesSample {
		_, err = storage.CreateMessage(boardShortName, msg.author, msg.text, &msg.attachments, msg.threadId)
		require.NoError(t, err, "CreateMessage should not return an error")
	}

	board, err = storage.GetBoard(boardShortName, 1)
	require.NoError(t, err, "GetBoard should not return an error")

	// check board metadata
	require.Equal(t, "testboard4", board.Name, "Expected board.Name testboard4, got %s", board.Name)
	require.Equal(t, "tb4", board.ShortName, "Expected board.ShortName tb4, got %s", board.ShortName)

	// check thread order
	threads := board.Threads
	require.Len(t, threads, 3, "Expected 3 threads")
	require.Equal(t, "thread2", threads[0].Title, "Expected thread2 to be first")
	require.Equal(t, "thread3", threads[1].Title, "Expected thread3 to be second")
	require.Equal(t, "thread1", threads[2].Title, "Expected thread1 to be third")

	// check message order
	messages := threads[0].Messages
	require.Len(t, messages, 4, "Expected 4 messages in thread2")
	require.Equal(t, "text 2", messages[0].Text, "Expected 'text 2' to be the first message in thread2")
	require.Equal(t, "text 4", messages[1].Text, "Expected 'text 4' to be the second message in thread2")
	require.Equal(t, "text 5", messages[2].Text, "Expected 'text 5' to be the third message in thread2")
	require.Equal(t, "text 8", messages[3].Text, "Expected 'text 8' to be the fourth message in thread2")

	messages = threads[1].Messages
	require.Len(t, messages, 2, "Expected 2 messages in thread3")
	require.Equal(t, "text 6", messages[0].Text, "Expected 'text 6' to be the first message in thread3")
	require.Equal(t, "text 7", messages[1].Text, "Expected 'text 7' to be the second message in thread3")

	messages = threads[2].Messages
	require.Len(t, messages, 1, "Expected 1 message in thread1")
	require.Equal(t, "text 1", messages[0].Text, "Expected 'text 1' to be the first message in thread1")

	// check pagination
	board, err = storage.GetBoard(boardShortName, 2)
	require.NoError(t, err, "GetBoard2 should not return an error")
	require.Len(t, board.Threads, 1, "Expected 1 thread on page 2")
	require.Equal(t, "thread4", board.Threads[0].Title, "Expected thread4 on page 2")
	require.Len(t, board.Threads[0].Messages, 1, "Expected 1 message in thread4 on page 2")
	require.Equal(t, "text 10", board.Threads[0].Messages[0].Text, "Expected 'text 10' to be the first message in thread4 on page 2")

	// check bump limit
	// spam thread to bump limit
	for i := 0; i < storage.cfg.Public.BumpLimit+10; i++ {
		_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 1}, "test", nil, thread2)
		require.NoError(t, err, "CreateMessage should not return an error")
	}
	// bump another thread
	_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 1}, "test", nil, thread3)
	require.NoError(t, err, "CreateMessage should not return an error")
	// bump thread in bump limit
	_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 1}, "test", nil, thread2)
	require.NoError(t, err, "CreateMessage should not return an error")

	// check that thread in bump limit didnt get bumped
	board, err = storage.GetBoard(boardShortName, 1)
	require.NoError(t, err, "GetBoard should not return an error")
	require.Equal(t, "thread3", board.Threads[0].Title, "Expected thread3 to be first")
	require.Equal(t, "thread2", board.Threads[1].Title, "Expected thread2 to be second")

	// Delete the board
	err = storage.DeleteBoard(boardShortName)
	require.NoError(t, err, "DeleteBoard should not return an error")

	// Try to get the deleted board
	_, err = storage.GetBoard(boardShortName, 1)
	require.Error(t, err, "Expected error when getting deleted board")
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	require.Equal(t, 404, e.StatusCode, "Expected status code 404")
}

// Send new messages to several threads and check for invariants to be correct
func TestBoardInvariants(t *testing.T) {
	boardName := "testboard5"
	boardShortName := "tb5"

	// Create a board
	err := storage.CreateBoard(boardName, boardShortName)
	require.NoError(t, err, "CreateBoard should not return an error")

	messages := []domain.Message{{Author: domain.User{Id: 1}, Text: "test message1"}, {Author: domain.User{Id: 2}, Text: "test message2"}, {Author: domain.User{Id: 3}, Text: "test message3"}}
	// Create several threads
	thread1, err := storage.CreateThread("thread1", boardShortName, &messages[rand.Intn(len(messages))])
	require.NoError(t, err, "CreateThread1 should not return an error")
	thread2, err := storage.CreateThread("thread2", boardShortName, &messages[rand.Intn(len(messages))])
	require.NoError(t, err, "CreateThread2 should not return an error")
	thread3, err := storage.CreateThread("thread3", boardShortName, &messages[rand.Intn(len(messages))])
	require.NoError(t, err, "CreateThread3 should not return an error")
	thread4, err := storage.CreateThread("thread4", boardShortName, &messages[rand.Intn(len(messages))])
	require.NoError(t, err, "CreateThread4 should not return an error")

	threads := []int64{thread1, thread2, thread3, thread4}
	// Fill threads with messages
	n_messages := (storage.cfg.Public.BumpLimit * len(threads)) + 10 // atleast 1 thread will go into bump limit
	for i := 0; i < n_messages; i++ {
		th := threads[rand.Intn(len(threads))]
		m := messages[rand.Intn(len(messages))]
		_, err := storage.CreateMessage(boardShortName, &m.Author, m.Text, m.Attachments, th)
		require.NoError(t, err, "Message creation should not return an error")

		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err, "GetBoard should not return an error")

		// Check for threads per page
		require.LessOrEqual(t, len(board.Threads), storage.cfg.Public.ThreadsPerPage, "Num Threads should not exceed %d", storage.cfg.Public.ThreadsPerPage)

		// Check lastBumped correct order (desc)
		lastBumped := time.Now().Add(time.Hour)
		for _, thread := range board.Threads {
			require.False(t, thread.LastBumped.After(lastBumped), "Wrong thread order. LastBumped: %v, Title: %s, Threads: %v", lastBumped, thread.Title, board.Threads)
			lastBumped = thread.LastBumped

			// Check NLastMsg (op msg doesnt count)
			require.LessOrEqual(t, len(thread.Messages), (storage.cfg.Public.NLastMsg + 1), "Wrong msg count. Messages: %v", thread.Messages)

			// Check message correct order (asc)
			msgCreated := thread.Messages[0].CreatedAt.Add(-time.Hour)
			for _, msg := range thread.Messages {
				require.False(t, msg.CreatedAt.Before(msgCreated), "Wrong message order. Messages: %v", thread.Messages)
				msgCreated = msg.CreatedAt
			}
		}
	}
}
