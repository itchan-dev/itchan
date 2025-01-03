package pg

import (
	"testing"
	"time"

	"math/rand"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
)

func TestCreateBoard(t *testing.T) {
	boardName := "testboard1"
	boardShortName := "tb1"

	err := storage.CreateBoard(boardName, boardShortName)
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	// Try to create a board with the same short name
	err = storage.CreateBoard(boardName, boardShortName)
	if err == nil {
		t.Fatal("Expected error when creating duplicate board, got nil")
	}

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	if err != nil {
		t.Errorf("Failed to clean up after test: %v", err) // Use Errorf for cleanup errors, as the test already failed.
	}
}

func TestGetBoard(t *testing.T) {
	boardName := "testboard2"
	boardShortName := "tb2"

	// Create a board
	err := storage.CreateBoard(boardName, boardShortName)
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	// Get the board
	board, err := storage.GetBoard(boardShortName, 1)
	if err != nil {
		t.Fatalf("GetBoard failed: %v", err)
	}
	if board.Name != boardName {
		t.Errorf("Expected board name %s, got %s", boardName, board.Name)
	}
	if board.ShortName != boardShortName {
		t.Errorf("Expected board short name %s, got %s", boardShortName, board.ShortName)
	}

	// Try to get a non-existent board
	_, err = storage.GetBoard("nonexistent", 1)
	if err == nil {
		t.Fatal("Expected error when getting non-existent board, got nil")
	}
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %v", err)
	}

	// Clean up
	err = storage.DeleteBoard(boardShortName)
	if err != nil {
		t.Errorf("Failed to clean up after test: %v", err)
	}
}

func TestDeleteBoard(t *testing.T) {
	boardName := "testboard3"
	boardShortName := "tb3"

	// Create a board
	err := storage.CreateBoard(boardName, boardShortName)
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}
	// Create thread and message
	thread_id, err := storage.CreateThread("test_title", boardShortName, &domain.Message{Author: domain.User{Id: 229}, Text: "test text"})
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	msg_id, err := storage.CreateMessage("tb4", &domain.User{Id: 229}, "test text 2", nil, thread_id)
	// Delete the board
	err = storage.DeleteBoard(boardShortName)
	if err != nil {
		t.Fatalf("DeleteBoard failed: %v", err)
	}
	// Check that thread also deleted
	_, err = storage.GetThread(thread_id)
	if err == nil {
		t.Fatal("Expected error when getting thread from non-existent board, got nil")
	}
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %v", err)
	}
	// Check that messages deleted
	msg, err := storage.GetMessage(thread_id)
	if err == nil {
		t.Fatalf("Expected error when getting message from non-existent board, got nil, msg: %v", msg)
	}
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %v", err)
	}
	_, err = storage.GetMessage(msg_id)
	if err == nil {
		t.Fatal("Expected error when getting message from non-existent board, got nil")
	}
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %v", err)
	}

	// Try to delete a non-existent board
	err = storage.DeleteBoard("nonexistent")
	if err == nil {
		t.Fatal("Expected error when deleting non-existent board, got nil")
	}
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %v", err)
	}
}

func TestBoardWorkflow(t *testing.T) {
	boardName := "testboard4"
	boardShortName := "tb4"

	// Create a board
	err := storage.CreateBoard(boardName, boardShortName)
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	// Get the board
	board, err := storage.GetBoard(boardShortName, 1)
	if err != nil {
		t.Fatalf("GetBoard failed: %v", err)
	}
	if board.Name != boardName {
		t.Errorf("Expected board name %s, got %s", boardName, board.Name)
	}
	if board.ShortName != boardShortName {
		t.Errorf("Expected board short name %s, got %s", boardShortName, board.ShortName)
	}
	// Create several threads
	_, err = storage.CreateThread("thread4", boardShortName, &domain.Message{Author: domain.User{Id: 229}, Text: "text 10"})
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	_, err = storage.CreateThread("thread1", boardShortName, &domain.Message{Author: domain.User{Id: 228}, Text: "text 1"})
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	thread2, err := storage.CreateThread("thread2", boardShortName, &domain.Message{Author: domain.User{Id: 229}, Text: "text 2"})
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	thread3, err := storage.CreateThread("thread3", boardShortName, &domain.Message{Author: domain.User{Id: 229}, Text: "text 6"})
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
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
		_, err = storage.CreateMessage("tb4", msg.author, msg.text, &msg.attachments, msg.threadId)
		if err != nil {
			t.Fatalf("CreateMessage failed: %v", err)
		}
	}
	board, err = storage.GetBoard(boardShortName, 1)
	if err != nil {
		t.Fatalf("GetBoard failed: %v", err)
	}
	// check board metadata
	if board.Name != "testboard4" || board.ShortName != "tb4" {
		t.Fatalf("Expected board.Name testboard4 board.shortName tb4, got %s %s", board.Name, board.ShortName)
	}
	// check thread order
	threads := board.Threads
	if len(threads) != 3 || threads[0].Title != "thread2" || threads[1].Title != "thread3" || threads[2].Title != "thread1" {
		t.Fatalf("Problem with thread order. Current threads %v", threads)
	}
	// check message order
	messages := threads[0].Messages
	if len(messages) != 4 || messages[0].Text != "text 2" || messages[1].Text != "text 4" || messages[2].Text != "text 5" || messages[3].Text != "text 8" {
		t.Fatalf("Problem with message order for thread 2. Current messages %v", messages)
	}
	messages = threads[1].Messages
	if len(messages) != 2 || messages[0].Text != "text 6" || messages[1].Text != "text 7" {
		t.Fatalf("Problem with message order for thread 3. Current messages %v", messages)
	}
	messages = threads[2].Messages
	if len(messages) != 1 || messages[0].Text != "text 1" {
		t.Fatalf("Problem with message order for thread 1. Current messages %v", messages)
	}
	// check pagination
	board, err = storage.GetBoard(boardShortName, 2)
	if err != nil {
		t.Fatalf("GetBoard2 failed: %v", err)
	}
	if len(board.Threads) != 1 || board.Threads[0].Title != "thread4" || len(board.Threads[0].Messages) != 1 || board.Threads[0].Messages[0].Text != "text 10" {
		t.Fatalf("Problem with pagination. Current board (page 2) %v", board)
	}
	// check bump limit
	// spam thread to bump limit
	for i := 0; i < storage.cfg.BumpLimit+10; i++ {
		_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 1}, "test", nil, thread2)
		if err != nil {
			t.Fatalf("CreateMessage failed: %v", err)
		}
	}
	// bump another thread
	_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 1}, "test", nil, thread3)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}
	// bump thread in bump limit
	_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 1}, "test", nil, thread2)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}
	// check that thread in bump limit didnt get bumped
	board, err = storage.GetBoard(boardShortName, 1)
	if err != nil {
		t.Fatalf("GetBoard failed: %v", err)
	}
	if board.Threads[0].Title != "thread3" || board.Threads[1].Title != "thread2" {
		t.Fatalf("Bump limit test failed. Current threads: %v", board.Threads)
	}
	// Delete the board
	err = storage.DeleteBoard(boardShortName)
	if err != nil {
		t.Fatalf("DeleteBoard failed: %v", err)
	}
	// Try to get the deleted board
	_, err = storage.GetBoard(boardShortName, 1)
	if err == nil {
		t.Fatal("Expected error when getting deleted board, got nil")
	}
	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %v", err)
	}
}

// Send new messages to several threads and check for invariants to be correct
func TestBoardInvariants(t *testing.T) {
	boardName := "testboard5"
	boardShortName := "tb5"

	// Create a board
	err := storage.CreateBoard(boardName, boardShortName)
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	messages := []domain.Message{{Author: domain.User{Id: 1}, Text: "test message1"}, {Author: domain.User{Id: 2}, Text: "test message2"}, {Author: domain.User{Id: 3}, Text: "test message3"}}
	// Create several threads
	thread1, err := storage.CreateThread("thread1", boardShortName, &messages[rand.Intn(len(messages))])
	if err != nil {
		t.Fatalf("CreateThread1 failed: %v", err)
	}
	thread2, err := storage.CreateThread("thread2", boardShortName, &messages[rand.Intn(len(messages))])
	if err != nil {
		t.Fatalf("CreateThread2 failed: %v", err)
	}
	thread3, err := storage.CreateThread("thread3", boardShortName, &messages[rand.Intn(len(messages))])
	if err != nil {
		t.Fatalf("CreateThread3 failed: %v", err)
	}
	thread4, err := storage.CreateThread("thread4", boardShortName, &messages[rand.Intn(len(messages))])
	if err != nil {
		t.Fatalf("CreateThread4 failed: %v", err)
	}
	threads := []int64{thread1, thread2, thread3, thread4}
	// Fill threads with messages
	n_messages := (storage.cfg.BumpLimit * len(threads)) + 10 // atleast 1 thread will go into bump limit
	for i := 0; i < n_messages; i++ {
		th := threads[rand.Intn(len(threads))]
		m := messages[rand.Intn(len(messages))]
		_, err := storage.CreateMessage(boardShortName, &m.Author, m.Text, m.Attachments, th)
		if err != nil {
			t.Fatalf("Message creation failed: %v", err)
		}
		board, err := storage.GetBoard(boardShortName, 1)
		if err != nil {
			t.Fatalf("GetBoard failed: %v", err)
		}
		// Check for threads per page
		if len(board.Threads) > storage.cfg.ThreadsPerPage {
			t.Fatalf("Num Threads != %d", storage.cfg.ThreadsPerPage)
		}
		// Check lastBumped correct order (desc)
		lastBumped := time.Now().Add(time.Hour)
		for _, thread := range board.Threads {
			if thread.LastBumped.After(lastBumped) {
				t.Fatalf("Wrong thread order. LastBumped: %v, Title: %s, Threads: %v", lastBumped, thread.Title, board.Threads)
			}
			lastBumped = thread.LastBumped
			// Check NLastMsg (op msg doesnt count)
			if len(thread.Messages) > (storage.cfg.NLastMsg + 1) {
				t.Fatalf("Wrong msg count. Messages: %v", thread.Messages)
			}
			// Check message correct order (asc)
			msgCreated := thread.Messages[0].CreatedAt.Add(-time.Hour)
			for _, msg := range thread.Messages {
				if msg.CreatedAt.Before(msgCreated) {
					t.Fatalf("Wrong message order. Messages: %v", thread.Messages)
				}
				msgCreated = msg.CreatedAt
			}
		}
	}

}
