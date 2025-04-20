package pg

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBoard(t *testing.T) {
	boardName := "Test Board"
	boardShortName := generateString(t)

	t.Run("create new board", func(t *testing.T) {
		err := storage.CreateBoard(boardName, boardShortName, nil)
		require.NoError(t, err)

	})

	t.Cleanup(func() {
		require.NoError(t, storage.DeleteBoard(boardShortName))
	})

	t.Run("duplicate short name should fail", func(t *testing.T) {
		err := storage.CreateBoard(boardName, boardShortName, nil)
		require.Error(t, err)
	})

	t.Run("allowedEmails of 0 length forbidden", func(t *testing.T) {
		err := storage.CreateBoard(boardName, boardShortName, &domain.Emails{})
		require.ErrorIs(t, err, emptyAllowedEmailsError)
	})
}

func TestGetBoard(t *testing.T) {
	boardName := "Test Board"
	allowedEmails := domain.Emails{"@test.ru", "@test2.ru"}
	boardShortName := generateString(t)
	testBegins := time.Now()
	time.Sleep(50 * time.Millisecond)

	// Setup
	require.NoError(t, storage.CreateBoard(boardName, boardShortName, &allowedEmails))
	t.Cleanup(func() { require.NoError(t, storage.DeleteBoard(boardShortName)) })

	t.Run("get existing board", func(t *testing.T) {
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		require.Equal(t, boardName, board.Name)
		require.Equal(t, boardShortName, board.ShortName)
		require.Equal(t, allowedEmails, *board.AllowedEmails)
		require.Equal(t, board.CreatedAt, board.LastActivity, "Create should be equal to last_activity when no other activies present")
		require.True(t, board.CreatedAt.After(testBegins), "Creation should be after test begins")
	})

	t.Run("non-existent board should 404", func(t *testing.T) {
		_, err := storage.GetBoard("nonexistent", 1)
		requireNotFoundError(t, err)
	})
}

func TestDeleteBoard(t *testing.T) {
	boardShortName := generateString(t)
	boardName := "Test Board"

	// Setup
	require.NoError(t, storage.CreateBoard(boardName, boardShortName, nil))
	threadID := createTestThread(t, boardShortName, "Test Title", &domain.Message{
		Author: domain.User{Id: 229},
		Text:   "Test text",
	})
	msgID := createTestMessage(t, boardShortName, &domain.User{Id: 229}, "Test text 2", nil, threadID)

	t.Run("delete existing board", func(t *testing.T) {
		require.NoError(t, storage.DeleteBoard(boardShortName))

		t.Run("associated thread should be deleted", func(t *testing.T) {
			_, err := storage.GetThread(threadID)
			requireNotFoundError(t, err)
		})

		t.Run("associated messages should be deleted", func(t *testing.T) {
			msg, err := storage.GetMessage(threadID)
			require.Error(t, err)
			require.Nil(t, msg)
			requireNotFoundError(t, err)

			_, err = storage.GetMessage(msgID)
			requireNotFoundError(t, err)
		})
	})

	t.Run("delete non-existent board should 404", func(t *testing.T) {
		err := storage.DeleteBoard("nonexistent")
		requireNotFoundError(t, err)
	})
}

func TestBoardWorkflow(t *testing.T) {
	boardShortName := setupBoard(t)

	// Create test threads
	threads := []struct {
		title string
		msg   domain.Message
	}{
		{"thread1", domain.Message{Author: domain.User{Id: 228}, Text: "op1"}},
		{"thread2", domain.Message{Author: domain.User{Id: 229}, Text: "op2"}},
		{"thread3", domain.Message{Author: domain.User{Id: 229}, Text: "op3"}},
		{"thread4", domain.Message{Author: domain.User{Id: 229}, Text: "op4"}},
		{"thread5", domain.Message{Author: domain.User{Id: 229}, Text: "op5"}},
	}

	threadIds := make([]int64, len(threads))
	for i, th := range threads {
		threadIds[i] = createTestThread(t, boardShortName, th.title, &th.msg)
	}

	// Create test messages
	messages := []struct {
		threadId int64
		text     string
		authorID int64
		files    domain.Attachments
	}{
		{1, "msg1", 228, nil},
		{1, "msg2", 300, domain.Attachments{"file.txt", "file2.png"}},
		{2, "msg3", 301, nil},
		{1, "msg4", 303, domain.Attachments{"file3.txt", "file4.png"}},
		{0, "msg5", 303, nil},
		{3, "msg6", 303, nil},
	}

	for _, msg := range messages {
		createTestMessage(t, boardShortName, &domain.User{Id: msg.authorID}, msg.text, &msg.files, threadIds[msg.threadId])
	}

	t.Run("verify board structure", func(t *testing.T) {
		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1), "Refresh view shouldnt throw error") // manually refresh board view
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)

		assert.Equal(t, boardShortName, board.ShortName)
		assert.Len(t, board.Threads, 3, "Page 1 should show 3 threads, instead it has %d", len(board.Threads))

		t.Run("thread order by last bump", func(t *testing.T) {
			requireThreadOrder(t, board.Threads, []string{"thread4", "thread1", "thread2"})
		})

		t.Run("message order within threads", func(t *testing.T) {
			requireMessageOrder(t, board.Threads[0].Messages, []string{"op4", "msg6"})
			requireMessageOrder(t, board.Threads[1].Messages, []string{"op1", "msg5"})
			requireMessageOrder(t, board.Threads[2].Messages, []string{"op2", "msg1", "msg2", "msg4"})
		})
	})

	t.Run("pagination", func(t *testing.T) {
		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1), "Refresh view shouldnt throw error") // manually refresh board view
		board, err := storage.GetBoard(boardShortName, 2)
		require.NoError(t, err)
		require.Len(t, board.Threads, 2, "Page 2 should show 2 threads")

		t.Run("thread order by last bump", func(t *testing.T) {
			requireThreadOrder(t, board.Threads, []string{"thread3", "thread5"})
		})

		t.Run("message order within threads", func(t *testing.T) {
			requireMessageOrder(t, board.Threads[0].Messages, []string{"op3", "msg3"})
			requireMessageOrder(t, board.Threads[1].Messages, []string{"op5"})
		})
	})

	t.Run("bump limit enforcement", func(t *testing.T) {
		threadID := createTestThread(t, boardShortName, "Bump Test", &domain.Message{
			Author: domain.User{Id: 1},
			Text:   "Bump test",
		})

		// get thread to bump limit
		for i := 0; i < storage.cfg.Public.BumpLimit+10; i++ {
			createTestMessage(t, boardShortName, &domain.User{Id: 1}, "bump", nil, threadID)
		}
		// bump another threads
		createTestMessage(t, boardShortName, &domain.User{Id: 1}, "bump", nil, threadIds[0]) // thread1
		createTestMessage(t, boardShortName, &domain.User{Id: 1}, "bump", nil, threadIds[1]) // thread2

		// bump thread with bump limit
		createTestMessage(t, boardShortName, &domain.User{Id: 1}, "bump", nil, threadID)

		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1), "Refresh view shouldnt throw error") // manually refresh board view
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		requireThreadOrder(t, board.Threads, []string{"thread2", "thread1", "Bump Test"})
	})
}

func TestBoardInvariants(t *testing.T) {
	boardShortName := setupBoard(t)

	// Create initial threads
	messages := []domain.Message{
		{Author: domain.User{Id: 1}, Text: "msg1"},
		{Author: domain.User{Id: 2}, Text: "msg2"},
		{Author: domain.User{Id: 3}, Text: "msg3"},
	}

	threadCount := 4
	threads := make([]int64, threadCount)
	for i := range threads {
		threads[i] = createTestThread(t, boardShortName, "Thread"+strconv.Itoa(i+1), &messages[rand.Intn(len(messages))])
	}

	// Stress test with random messages
	messageCount := storage.cfg.Public.BumpLimit*len(threads) + 10
	for i := 0; i < messageCount; i++ {
		thread := threads[rand.Intn(len(threads))]
		msg := messages[rand.Intn(len(messages))]
		createTestMessage(t, boardShortName, &msg.Author, msg.Text, msg.Attachments, thread)
		checkBoardInvariants(t, boardShortName)
	}
}

func TestGetBoardsAllowedEmails(t *testing.T) {
	t.Run("returns boards with non-null allowed_emails", func(t *testing.T) {
		toCreate := []domain.Board{
			{Name: "Board 1", ShortName: "b1", AllowedEmails: nil},
			{Name: "Board 2", ShortName: "b2", AllowedEmails: &domain.Emails{"@test1.ru", "@test2.ru"}},
			{Name: "Board 3", ShortName: "b3", AllowedEmails: &domain.Emails{"@test3.ru"}},
		}
		for _, b := range toCreate {
			require.NoError(t, storage.CreateBoard(b.Name, b.ShortName, b.AllowedEmails))
			t.Cleanup(func() {
				require.NoError(t, storage.DeleteBoard(b.ShortName), "board short name %s", b.ShortName)
			})
		}

		boards, err := storage.GetBoardsAllowedEmails()
		require.NoError(t, err)

		require.Len(t, boards, 2)
		require.Equal(t, boards[0], toCreate[1], "Got %v", boards)
		require.Equal(t, boards[1], toCreate[2], "Got %v", boards)
	})

	t.Run("returns empty slice when no allowed emails", func(t *testing.T) {
		toCreate := []domain.Board{
			{Name: "Board 1", ShortName: "b1", AllowedEmails: nil},
			{Name: "Board 2", ShortName: "b2", AllowedEmails: nil},
			{Name: "Board 3", ShortName: "b3", AllowedEmails: nil},
		}
		for _, b := range toCreate {
			require.NoError(t, storage.CreateBoard(b.Name, b.ShortName, b.AllowedEmails))
			t.Cleanup(func() {
				require.NoError(t, storage.DeleteBoard(b.ShortName), "board short name %s", b.ShortName)
			})
		}

		boards, err := storage.GetBoardsAllowedEmails()
		require.NoError(t, err)
		require.Empty(t, boards, "Expected empty boards, got %v", boards)
	})

	t.Run("every board has allowed emails", func(t *testing.T) {
		toCreate := []domain.Board{
			{Name: "Board 1", ShortName: "b1", AllowedEmails: &domain.Emails{"testcompany"}},
			{Name: "Board 2", ShortName: "b2", AllowedEmails: &domain.Emails{"@test1.ru", "@test2.ru"}},
			{Name: "Board 3", ShortName: "b3", AllowedEmails: &domain.Emails{"@test3.ru"}},
		}
		for _, b := range toCreate {
			require.NoError(t, storage.CreateBoard(b.Name, b.ShortName, b.AllowedEmails))
			t.Cleanup(func() {
				require.NoError(t, storage.DeleteBoard(b.ShortName), "board short name %s", b.ShortName)
			})
		}

		boards, err := storage.GetBoardsAllowedEmails()
		require.NoError(t, err)

		require.Len(t, boards, 3)
		require.Equal(t, toCreate, boards)
	})
}

func checkBoardInvariants(t *testing.T, boardShortName string) {
	t.Helper()
	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1), "Refresh view shouldnt throw error") // manually refresh board view
	board, err := storage.GetBoard(boardShortName, 1)
	require.NoError(t, err)

	require.LessOrEqual(t, len(board.Threads), storage.cfg.Public.ThreadsPerPage)

	var lastBumped time.Time
	for i, thread := range board.Threads {
		if i > 0 {
			require.False(t, thread.LastBumped.After(lastBumped), "Thread order incorrect at index %d", i)
		}
		lastBumped = thread.LastBumped

		require.LessOrEqual(t, len(thread.Messages), storage.cfg.Public.NLastMsg+1)
		for j := 1; j < len(thread.Messages); j++ {
			require.False(t, thread.Messages[j].CreatedAt.Before(thread.Messages[j-1].CreatedAt),
				"Message order incorrect in thread %s", thread.Title)
		}
	}
}

func TestGetActiveBoards(t *testing.T) {
	t.Run("created board in active by default", func(t *testing.T) {
		_ = setupBoard(t)
		boards, err := storage.GetActiveBoards(time.Hour)
		require.NoError(t, err)
		assert.Len(t, boards, 1, "Should be 1 active board")
	})

	t.Run("board activity after message creation", func(t *testing.T) {
		boardShortName, threadId := setupBoardAndThread(t)

		// Make board inactive
		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, boardShortName)
		require.NoError(t, err, "Failed to set old modified time")

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		assert.Empty(t, boards, "Board should be inactive after modified time is made old")

		_, err = storage.CreateMessage(boardShortName, &domain.User{Id: 1}, "test msg", nil, threadId)
		require.NoError(t, err, "Failed to create message")

		boards, err = storage.GetActiveBoards(5 * time.Minute) // Same short interval
		require.NoError(t, err)
		require.Len(t, boards, 1, "Board should become active again immediately after creation")
		assert.Equal(t, boardShortName, boards[0].ShortName, "Board short name mismatch after creation")
	})

	t.Run("board activity after message deletion", func(t *testing.T) {
		boardShortName, _, msgId := setupBoardAndThreadAndMessage(t)

		// Make board inactive
		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, boardShortName)
		require.NoError(t, err, "Failed to set old modified time")

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		assert.Empty(t, boards, "Board should be inactive after modified time is made old")

		err = storage.DeleteMessage(boardShortName, msgId)
		require.NoError(t, err, "Failed to delete message")

		boards, err = storage.GetActiveBoards(5 * time.Minute) // Same short interval
		require.NoError(t, err)
		require.Len(t, boards, 1, "Board should become active again immediately after deletion")
		assert.Equal(t, boardShortName, boards[0].ShortName, "Board short name mismatch after deletion")
	})

	t.Run("board activity after thread creation", func(t *testing.T) {
		boardShortName := setupBoard(t)

		// Make board inactive
		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, boardShortName)
		require.NoError(t, err, "Failed to set old modified time")

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		assert.Empty(t, boards, "Board should be inactive after modified time is made old")

		_, err = storage.CreateThread("test thread", boardShortName, &domain.Message{Text: "op msg"})
		require.NoError(t, err, "Failed to delete message")

		boards, err = storage.GetActiveBoards(5 * time.Minute) // Same short interval
		require.NoError(t, err)
		require.Len(t, boards, 1, "Board should become active again immediately after thread creation")
		assert.Equal(t, boardShortName, boards[0].ShortName, "Board short name mismatch after thread creation")
	})

	t.Run("board activity after thread deletion", func(t *testing.T) {
		boardShortName, threadId := setupBoardAndThread(t)

		// Make board inactive
		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, boardShortName)
		require.NoError(t, err, "Failed to set old modified time")

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		assert.Empty(t, boards, "Board should be inactive after modified time is made old")

		err = storage.DeleteThread(boardShortName, threadId)
		require.NoError(t, err, "Failed to delete message")

		boards, err = storage.GetActiveBoards(5 * time.Minute) // Same short interval
		require.NoError(t, err)
		require.Len(t, boards, 1, "Board should become active again immediately after thread deletion")
		assert.Equal(t, boardShortName, boards[0].ShortName, "Board short name mismatch after thread deletion")
	})

	t.Run("board is active with recent message modification", func(t *testing.T) {
		boardShortName, _ := setupBoardAndThread(t)

		// Interval 10 minutes should return 1 board because the message was just created/modified
		boards, err := storage.GetActiveBoards(10 * time.Minute)
		require.NoError(t, err)
		require.Len(t, boards, 1, "Board should be active shortly after creation")
		assert.Equal(t, boardShortName, boards[0].ShortName)

		// Manually set the board's LAST_ACTIVITY time to 30 minutes ago
		modifiedTime := time.Now().UTC().Add(-30 * time.Minute)
		_, err = storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", modifiedTime, boardShortName)
		require.NoError(t, err, "Failed to update modified timestamp")

		// Interval 31 minutes should still include the board
		boards, err = storage.GetActiveBoards(31 * time.Minute)
		require.NoError(t, err)
		require.Len(t, boards, 1, "Board should be active for interval > 30 mins")
		assert.Equal(t, boardShortName, boards[0].ShortName)

		// Interval 29 minutes should exclude the board
		boards, err = storage.GetActiveBoards(29 * time.Minute)
		require.NoError(t, err)
		assert.Empty(t, boards, "Board should be inactive for interval < 30 mins")
	})

	t.Run("multiple active boards", func(t *testing.T) {
		_ = setupBoard(t)
		_ = setupBoard(t)
		_ = setupBoard(t)
		boards, err := storage.GetActiveBoards(time.Hour)
		require.NoError(t, err)
		assert.Len(t, boards, 3, "Should be 1 active board")

		time.Sleep(500 * time.Millisecond)
		boards, err = storage.GetActiveBoards(200 * time.Millisecond)
		require.NoError(t, err)
		assert.Empty(t, boards, "Should be 0 active board")
	})
}
