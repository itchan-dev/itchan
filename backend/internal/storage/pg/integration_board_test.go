package pg

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain" // Use alias to avoid conflict if needed
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateBoard verifies the board creation logic.
func TestCreateBoard(t *testing.T) {
	boardName := "Test Create Board"
	boardShortName := generateString(t)
	allowedEmails := &domain.Emails{"test@example.com"}

	t.Run("create new board without allowed emails", func(t *testing.T) {
		bShortName := generateString(t)
		err := storage.CreateBoard(domain.BoardCreationData{Name: boardName, ShortName: bShortName, AllowedEmails: nil})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, storage.DeleteBoard(bShortName))
		})
	})

	t.Run("create new board with allowed emails", func(t *testing.T) {
		bShortName := generateString(t)
		err := storage.CreateBoard(domain.BoardCreationData{Name: boardName, ShortName: bShortName, AllowedEmails: allowedEmails})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, storage.DeleteBoard(bShortName))
		})
	})

	t.Run("duplicate short name should fail", func(t *testing.T) {
		// Setup board for duplicate check
		err := storage.CreateBoard(domain.BoardCreationData{Name: boardName, ShortName: boardShortName, AllowedEmails: nil})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, storage.DeleteBoard(boardShortName))
		})

		// Attempt to create again
		err = storage.CreateBoard(domain.BoardCreationData{Name: "Another Name", ShortName: boardShortName, AllowedEmails: nil})
		require.Error(t, err, "Creating board with duplicate short name should fail")
		assert.Contains(t, err.Error(), "possibly duplicate short_name") // Check for specific error message part
	})

	t.Run("allowedEmails of 0 length forbidden", func(t *testing.T) {
		bShortName := generateString(t)
		err := storage.CreateBoard(domain.BoardCreationData{Name: boardName, ShortName: bShortName, AllowedEmails: &domain.Emails{}})
		require.ErrorIs(t, err, emptyAllowedEmailsError)
		_ = storage.DeleteBoard(bShortName) // Attempt to clean up, ignore error if not created
	})
}

// TestGetBoard verifies retrieving board details.
func TestGetBoard(t *testing.T) {
	boardName := "Test Get Board"
	allowedEmails := &domain.Emails{"@test.ru", "@test2.ru"}
	boardShortName := generateString(t)
	testBegins := time.Now().UTC()

	err := storage.CreateBoard(domain.BoardCreationData{Name: boardName, ShortName: boardShortName, AllowedEmails: allowedEmails})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.DeleteBoard(boardShortName)) })

	t.Run("get existing board", func(t *testing.T) {
		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1), "Refresh view shouldn't throw error")

		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		assert.Equal(t, boardName, board.Name)
		assert.Equal(t, boardShortName, board.ShortName)
		assert.Equal(t, allowedEmails, board.AllowedEmails)
		assert.True(t, !board.CreatedAt.Before(testBegins), "Creation time %v should not be before test begins %v", board.CreatedAt, testBegins)
		assert.True(t, !board.LastActivity.Before(board.CreatedAt), "Last activity %v should not be before creation %v", board.LastActivity, board.CreatedAt)
		assert.Empty(t, board.Threads, "Board should have no threads initially")
	})

	t.Run("non-existent board should 404", func(t *testing.T) {
		_, err := storage.GetBoard("nonexistentboard", 1)
		requireNotFoundError(t, err)
	})
}

// TestDeleteBoard verifies board deletion and cascading effects.
func TestDeleteBoard(t *testing.T) {
	boardShortNameToTest, threadID, messageID := setupBoardAndThreadAndMessage(t)

	t.Run("delete existing board", func(t *testing.T) {
		var exists bool
		unquotedViewName := viewTableName(boardShortNameToTest)
		err := storage.db.QueryRow("SELECT EXISTS (SELECT FROM pg_matviews WHERE matviewname = $1)", unquotedViewName).Scan(&exists)
		require.NoError(t, err, "DB query for view existence failed")
		assert.True(t, exists, "Materialized view %s should be presented", unquotedViewName)

		unquotedMsgTableName := messagesPartitionName(boardShortNameToTest)
		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedMsgTableName).Scan(&exists)
		require.NoError(t, err, "DB query for table existence failed")
		assert.True(t, exists, "Messages table %s should be presented", unquotedMsgTableName)

		unquotedThreadsTableName := threadsPartitionName(boardShortNameToTest)
		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedThreadsTableName).Scan(&exists)
		require.NoError(t, err, "DB query for table existence failed")
		assert.True(t, exists, "Threads table %s should be presented", unquotedThreadsTableName)

		err = storage.DeleteBoard(boardShortNameToTest)
		require.NoError(t, err)

		_, err = storage.GetBoard(boardShortNameToTest, 1)
		requireNotFoundError(t, err)

		_, err = storage.GetThread(boardShortNameToTest, threadID)
		requireNotFoundError(t, err)

		_, err = storage.GetMessage(boardShortNameToTest, messageID)
		requireNotFoundError(t, err)

		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM pg_matviews WHERE matviewname = $1)", unquotedViewName).Scan(&exists)
		require.NoError(t, err, "DB query for view existence failed")
		assert.False(t, exists, "Materialized view %s should be dropped", unquotedViewName)

		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedMsgTableName).Scan(&exists)
		require.NoError(t, err, "DB query for table existence failed")
		assert.False(t, exists, "Messages table %s should be dropped", unquotedMsgTableName)

		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedThreadsTableName).Scan(&exists)
		require.NoError(t, err, "DB query for table existence failed")
		assert.False(t, exists, "Threads table %s should be dropped", unquotedThreadsTableName)
	})

	t.Run("delete non-existent board should 404", func(t *testing.T) {
		err := storage.DeleteBoard("nonexistentboard_del_test") // use unique name to avoid conflict
		requireNotFoundError(t, err)
	})
}

// TestBoardWorkflow simulates a typical usage flow: creating posts, checking board state, pagination.
func TestBoardWorkflow(t *testing.T) {
	boardShortName := setupBoard(t)

	threads := []domain.ThreadCreationData{
		{Title: "thread1", Board: boardShortName, OpMessage: domain.MessageCreationData{Author: domain.User{Id: 228}, Text: "op1"}},
		{Title: "thread2", Board: boardShortName, OpMessage: domain.MessageCreationData{Author: domain.User{Id: 229}, Text: "op2"}},
		{Title: "thread3", Board: boardShortName, OpMessage: domain.MessageCreationData{Author: domain.User{Id: 229}, Text: "op3"}},
		{Title: "thread4", Board: boardShortName, OpMessage: domain.MessageCreationData{Author: domain.User{Id: 229}, Text: "op4"}},
		{Title: "thread5", Board: boardShortName, OpMessage: domain.MessageCreationData{Author: domain.User{Id: 229}, Text: "op5"}},
	}
	threadIds := make([]domain.ThreadId, len(threads))
	for i, threadCreationInput := range threads {
		threadIds[i] = createTestThread(t, threadCreationInput)
	}

	messages := []domain.MessageCreationData{
		{Board: boardShortName, Author: domain.User{Id: 300}, Text: "msg1_t1", ThreadId: threadIds[0]},
		{Board: boardShortName, Author: domain.User{Id: 301}, Text: "msg2_t2", ThreadId: threadIds[1]},
		{Board: boardShortName, Author: domain.User{Id: 303}, Text: "msg3_t3", ThreadId: threadIds[2]},
		{Board: boardShortName, Author: domain.User{Id: 303}, Text: "msg4_t1", Attachments: &domain.Attachments{"file.txt", "file2.png"}, ThreadId: threadIds[0]},
		{Board: boardShortName, Author: domain.User{Id: 303}, Text: "msg5_t4", ThreadId: threadIds[3]},
	}
	for _, msg := range messages {
		createTestMessage(t, msg)
	}

	t.Run("verify board structure page 1", func(t *testing.T) {
		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)

		assert.Equal(t, boardShortName, board.ShortName)
		require.Len(t, board.Threads, 3, "Page 1 should show 3 threads")

		t.Run("thread order by last bump", func(t *testing.T) {
			requireThreadOrder(t, board.Threads, []string{"thread4", "thread1", "thread3"})
		})

		t.Run("message order within threads", func(t *testing.T) {
			requireMessageOrder(t, board.Threads[0].Messages, []string{"op4", "msg5_t4"})
			requireMessageOrder(t, board.Threads[1].Messages, []string{"op1", "msg1_t1", "msg4_t1"})
			requireMessageOrder(t, board.Threads[2].Messages, []string{"op3", "msg3_t3"})
		})
	})

	t.Run("pagination page 2", func(t *testing.T) {
		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))
		board, err := storage.GetBoard(boardShortName, 2)
		require.NoError(t, err)
		require.Len(t, board.Threads, 2, "Page 2 should show 2 threads")

		t.Run("thread order by last bump", func(t *testing.T) {
			requireThreadOrder(t, board.Threads, []string{"thread2", "thread5"})
		})

		t.Run("message order within threads", func(t *testing.T) {
			requireMessageOrder(t, board.Threads[0].Messages, []string{"op2", "msg2_t2"})
			requireMessageOrder(t, board.Threads[1].Messages, []string{"op5"})
		})
	})

	t.Run("bump limit enforcement", func(t *testing.T) {
		opMsgForBumpLimit := domain.MessageCreationData{
			Author: domain.User{Id: 1},
			Text:   "OP Bump Limit",
		}
		threadDataForBumpLimit := domain.ThreadCreationData{
			Title:     "Bump Limit Test",
			Board:     boardShortName,
			OpMessage: opMsgForBumpLimit,
		}
		bumpLimitThreadID := createTestThread(t, threadDataForBumpLimit)

		// Get created thread to the bump limit
		for i := 0; i < storage.cfg.Public.BumpLimit; i++ {
			msgData := domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: 1},
				Text:     fmt.Sprintf("bump %d", i+1),
				ThreadId: bumpLimitThreadID,
			}
			createTestMessage(t, msgData)
		}

		// Bump several threads
		createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 1}, Text: "bump_other1", ThreadId: threadIds[4]}) // thread5
		createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 1}, Text: "bump_other2", ThreadId: threadIds[0]}) // thread1
		// Bump (unsuccessfully) thread after bump limit
		createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 1}, Text: "post_after_limit", ThreadId: bumpLimitThreadID})

		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))
		board, err := storage.GetBoard(boardShortName, 1) // Page 1 has 3 threads
		require.NoError(t, err)
		require.Len(t, board.Threads, 3)

		requireThreadOrder(t, board.Threads, []string{"thread1", "thread5", "Bump Limit Test"})
	})
}

// TestBoardInvariants performs stress testing and checks structural invariants.
func TestBoardInvariants(t *testing.T) {
	boardShortName := setupBoard(t)

	opMessagesData := []domain.MessageCreationData{
		{Author: domain.User{Id: 1}, Text: "op_inv_1"},
		{Author: domain.User{Id: 2}, Text: "op_inv_2"},
		{Author: domain.User{Id: 3}, Text: "op_inv_3"},
	}

	threadCount := storage.cfg.Public.ThreadsPerPage*3 + 1
	createdThreadIDs := make([]domain.ThreadId, threadCount)
	for i := range createdThreadIDs {
		opMsg := opMessagesData[rand.Intn(len(opMessagesData))]
		threadData := domain.ThreadCreationData{
			Title:     "Invariant Thread " + strconv.Itoa(i+1),
			Board:     boardShortName,
			OpMessage: opMsg,
		}
		createdThreadIDs[i] = createTestThread(t, threadData)
	}

	pages := threadCount / storage.cfg.Public.ThreadsPerPage
	if threadCount%storage.cfg.Public.ThreadsPerPage != 0 {
		pages++
	}

	messageCount := storage.cfg.Public.BumpLimit*threadCount + 1 // Create enough messages to test bumping and message limits
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < messageCount; i++ {
		targetThreadID := createdThreadIDs[r.Intn(len(createdThreadIDs))]
		templateMsg := opMessagesData[r.Intn(len(opMessagesData))]

		replyData := domain.MessageCreationData{
			Board:       boardShortName,
			Author:      templateMsg.Author,
			Text:        fmt.Sprintf("Reply %d to %s", i, templateMsg.Text),
			Attachments: templateMsg.Attachments,
			ThreadId:    targetThreadID,
		}
		createTestMessage(t, replyData)
		if i%10 == 0 { // Periodically check invariants
			for page := 1; page <= pages; page++ {
				checkBoardInvariants(t, boardShortName, page)
			}
		}
	}

	for page := 1; page <= pages; page++ {
		checkBoardInvariants(t, boardShortName, page)
	}

	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))
	page := pages + 1
	board, err := storage.GetBoard(boardShortName, page)
	require.NoError(t, err)
	require.Empty(t, board.Threads, "There are should be no threads on page > number_of_threads/threads_per_page")
}

// checkBoardInvariants is a helper to verify board structure rules.
func checkBoardInvariants(t *testing.T, boardShortName domain.BoardShortName, page int) {
	t.Helper()
	require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))
	board, err := storage.GetBoard(boardShortName, page)
	require.NoError(t, err)
	require.NotEmpty(t, board.Threads, "Board threads shouldnt be empty")

	// If it's page 1, it can have up to ThreadsPerPage.
	// If it's a later page, it can also have up to ThreadsPerPage, or fewer if it's the last page.
	assert.LessOrEqual(t, len(board.Threads), storage.cfg.Public.ThreadsPerPage, "Page %d thread count (%d) exceeds limit (%d)", page, len(board.Threads), storage.cfg.Public.ThreadsPerPage)

	var lastBumped time.Time
	isFirstThread := true
	for i, thread := range board.Threads {
		if !isFirstThread {
			// Threads should be ordered by LastBumped DESC.
			// So, current thread's LastBumped should be <= previous thread's LastBumped.
			assert.False(t, thread.LastBumped.After(lastBumped), "Thread order incorrect at index %d on page %d: thread %s (%v) bumped after previous %s (%v)", i, page, thread.Title, thread.LastBumped, board.Threads[i-1].Title, lastBumped)
		}
		lastBumped = thread.LastBumped
		isFirstThread = false

		// Messages include OP + NLastMsg replies.
		assert.LessOrEqual(t, len(thread.Messages), storage.cfg.Public.NLastMsg+1, "Message count (%d) exceeds limit (%d) in thread %s on page %d", len(thread.Messages), storage.cfg.Public.NLastMsg+1, thread.Title, page)

		var lastMsgCreated time.Time
		isFirstMsg := true
		opCount := 0
		for j, msg := range thread.Messages {
			if msg.Op {
				opCount++
				assert.Equal(t, 0, j, "Op message should be first")
			}
			if !isFirstMsg {
				// Messages should be ordered by CreatedAt ASC.
				assert.False(t, msg.CreatedAt.Before(lastMsgCreated),
					"Message order incorrect in thread %s at index %d on page %d: msg %d (%v) created before previous (%v)", thread.Title, j, page, msg.Id, msg.CreatedAt, lastMsgCreated)
			}
			lastMsgCreated = msg.CreatedAt
			isFirstMsg = false

			// Check if all messages belong to the correct thread and board
			assert.Equal(t, thread.Id, msg.ThreadId, "Message %d in thread %s (page %d) has incorrect ThreadId %d", msg.Id, thread.Title, page, msg.ThreadId)
			assert.Equal(t, boardShortName, msg.Board, "Message %d in thread %s (page %d) has incorrect Board %s", msg.Id, thread.Title, page, msg.Board)
		}
		if len(thread.Messages) > 0 { // Ensure OP message is present if messages are shown
			assert.Equal(t, 1, opCount, "There are more than 1 op message in thread %s (page %d), first message: %v", thread.Title, page, thread.Messages[0].Op)
			assert.True(t, thread.Messages[0].Op, "First message in thread %s (page %d) must be OP", thread.Title, page)
		}
	}
}

// TestGetActiveBoards verifies retrieval of recently active boards.
func TestGetActiveBoards(t *testing.T) {
	t.Run("created board is active by default", func(t *testing.T) {
		boardShortName := setupBoard(t)

		boards, err := storage.GetActiveBoards(1 * time.Hour)
		require.NoError(t, err)

		found := false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.True(t, found, "Newly created board %s should be in active list", boardShortName)
	})

	t.Run("board activity after message creation", func(t *testing.T) {
		boardShortName, threadId := setupBoardAndThread(t)

		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, boardShortName)
		require.NoError(t, err, "Failed to set old activity time")

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		found := false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.False(t, found, "Board %s should be inactive after activity time is made old", boardShortName)

		msgData := domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 1}, Text: "test msg for activity", ThreadId: threadId}
		_ = createTestMessage(t, msgData)

		boards, err = storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		found = false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.True(t, found, "Board %s should become active again immediately after message creation", boardShortName)
	})

	t.Run("board activity after message deletion", func(t *testing.T) {
		boardShortName, _, msgId := setupBoardAndThreadAndMessage(t)

		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, boardShortName)
		require.NoError(t, err, "Failed to set old activity time")

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		found := false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.False(t, found, "Board should be inactive before message deletion")

		err = storage.DeleteMessage(boardShortName, msgId)
		require.NoError(t, err, "Failed to delete message")

		boards, err = storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		found = false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.True(t, found, "Board should become active again after message deletion")
	})

	t.Run("board activity after thread creation", func(t *testing.T) {
		boardShortName := setupBoard(t)

		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, boardShortName)
		require.NoError(t, err, "Failed to set old activity time")

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		found := false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.False(t, found, "Board should be inactive before thread creation")

		opMsg := domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 1}, Text: "op msg activity"}
		threadCreateData := domain.ThreadCreationData{Title: "activity test thread", Board: boardShortName, OpMessage: opMsg}
		_ = createTestThread(t, threadCreateData)

		boards, err = storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		found = false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.True(t, found, "Board should become active again immediately after thread creation")
	})

	t.Run("board activity after thread deletion", func(t *testing.T) {
		boardShortName, threadId := setupBoardAndThread(t)

		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, boardShortName)
		require.NoError(t, err, "Failed to set old activity time")

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		found := false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.False(t, found, "Board should be inactive before thread deletion")

		err = storage.DeleteThread(boardShortName, threadId)
		require.NoError(t, err, "Failed to delete thread")

		boards, err = storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		found = false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.True(t, found, "Board should become active again immediately after thread deletion")
	})

	t.Run("board activity respects interval", func(t *testing.T) {
		boardShortName, _ := setupBoardAndThread(t)

		activityTime := time.Now().UTC().Add(-30 * time.Minute)
		_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", activityTime, boardShortName)
		require.NoError(t, err, "Failed to update activity timestamp")

		boards, err := storage.GetActiveBoards(31 * time.Minute)
		require.NoError(t, err)
		found := false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.True(t, found, "Board should be active for interval > 30 mins")

		boards, err = storage.GetActiveBoards(29 * time.Minute)
		require.NoError(t, err)
		found = false
		for _, b := range boards {
			if b.ShortName == boardShortName {
				found = true
				break
			}
		}
		assert.False(t, found, "Board should be inactive for interval < 30 mins")
	})

	t.Run("multiple active boards", func(t *testing.T) {
		board1 := setupBoard(t)
		board2 := setupBoard(t)
		board3 := setupBoard(t)
		activeShortNames := map[domain.BoardShortName]bool{board1: true, board2: true, board3: true}

		boards, err := storage.GetActiveBoards(1 * time.Hour)
		require.NoError(t, err)

		returnedCount := 0
		for _, b := range boards {
			if _, ok := activeShortNames[b.ShortName]; ok {
				returnedCount++
			}
		}
		assert.Equal(t, len(activeShortNames), returnedCount, "Should find all 3 newly created boards active")

		oldTime := time.Now().UTC().Add(-2 * time.Hour) // Make sure it's older than 1 hour
		for sn := range activeShortNames {
			_, errDb := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", oldTime, sn)
			require.NoError(t, errDb)
		}

		// Interval is 1 hour, boards are 2 hours old, so should not be active
		boards, err = storage.GetActiveBoards(1 * time.Hour)
		require.NoError(t, err)
		returnedCount = 0
		for _, b := range boards {
			if _, ok := activeShortNames[b.ShortName]; ok {
				returnedCount++
			}
		}
		assert.Zero(t, returnedCount, "Should find 0 boards active after setting their activity to be old")
	})
}

// TestGetBoards verifies retrieval of all board metadata.
func TestGetBoards(t *testing.T) {
	t.Run("no boards initially, then returns created boards", func(t *testing.T) {
		// Get initial state (might include boards from other parallel tests, focus on delta)
		initialBoards, err := storage.GetBoards()
		require.NoError(t, err)

		// Create boards
		createdBoards := make(map[domain.BoardShortName]domain.BoardCreationData)
		var boardsOrder []domain.BoardShortName
		for _, board := range []domain.BoardCreationData{
			{Name: "Board Alpha", ShortName: "b1", AllowedEmails: &domain.Emails{"one@example.com"}},
			{Name: "Board Beta", ShortName: "b2", AllowedEmails: nil},
			{Name: "Board Gamma", ShortName: "b3", AllowedEmails: &domain.Emails{"three@example.com", "another@example.com"}},
		} {
			err = storage.CreateBoard(board)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, storage.DeleteBoard(board.ShortName)) })
			createdBoards[board.ShortName] = board
			boardsOrder = append(boardsOrder, board.ShortName)
		}

		allBoards, err := storage.GetBoards()
		require.NoError(t, err)
		require.Len(t, allBoards, len(initialBoards)+3, "Should retrieve all boards plus the 3 created in this test")

		// Extract the boards created in this test, maintaining their order from allBoards
		var testBoards []domain.Board
		for _, b := range allBoards {
			if _, ok := createdBoards[b.ShortName]; ok {
				testBoards = append(testBoards, b)
				board, err := storage.GetBoard(b.ShortName, 1)
				require.NoError(t, err)
				assert.Equal(t, board.BoardMetadata, b.BoardMetadata, "Board from GetBoards and GetBoard should have equal metadata")
			}
		}

		require.Len(t, testBoards, 3, "Should find all 3 test boards among all boards")

		// Verify order: board1SN should appear before board2SN, which should appear before board3SN
		assert.Equal(t, boardsOrder[0], testBoards[0].ShortName, "Board1 not in correct order")
		assert.Equal(t, boardsOrder[1], testBoards[1].ShortName, "Board2 not in correct order")
		assert.Equal(t, boardsOrder[2], testBoards[2].ShortName, "Board3 not in correct order")
	})

	t.Run("returns empty list if no boards exist", func(t *testing.T) {
		// To test this reliably, we'd need to ensure the DB is empty or
		// filter out boards not belonging to this specific test scope.
		// Assuming other tests clean up, we can create a temporary board and delete it.
		tempBoardSN := generateString(t)
		err := storage.CreateBoard(domain.BoardCreationData{Name: "Temp", ShortName: tempBoardSN})
		require.NoError(t, err)
		err = storage.DeleteBoard(tempBoardSN)
		require.NoError(t, err)

		boards, err := storage.GetBoards() // Just ensure it doesn't error.
		require.NoError(t, err)
		assert.Len(t, boards, 0)
	})
}

// TestGetBoardsByUser verifies retrieval of boards based on user permissions.
func TestGetBoardsByUser(t *testing.T) {
	adminUser := domain.User{Id: 101, Email: "admin@example.com", Admin: true}
	userDomain1 := domain.User{Id: 102, Email: "user@domain1.com", Admin: false}
	userDomain2 := domain.User{Id: 103, Email: "user@domain2.com", Admin: false}
	userOtherDomain := domain.User{Id: 104, Email: "user@other.com", Admin: false}

	boardASN := generateString(t)
	boardBSN := generateString(t)
	boardCSN := generateString(t)
	createdBoards := make(map[domain.BoardShortName]domain.BoardCreationData)
	for _, board := range []domain.BoardCreationData{
		{Name: "Public Board A", ShortName: boardASN, AllowedEmails: nil},
		{Name: "Domain1 Board B", ShortName: boardBSN, AllowedEmails: &domain.Emails{"domain1.com"}},
		{Name: "MultiDomain Board C", ShortName: boardCSN, AllowedEmails: &domain.Emails{"domain2.com", "another.com"}},
	} {
		err := storage.CreateBoard(board)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, storage.DeleteBoard(board.ShortName)) })
		createdBoards[board.ShortName] = board
	}

	// Helper to verify board metadata fields now that all are populated
	verifyFullBoardMetadata := func(t *testing.T, board domain.Board) {
		t.Helper()

		expectedBoard, err := storage.GetBoard(board.ShortName, 1)
		require.NoError(t, err)
		assert.Equal(t, expectedBoard.BoardMetadata, board.BoardMetadata, "Board metadata should be equal to GetBoard call")
	}

	t.Run("admin user gets all boards with full metadata", func(t *testing.T) {
		boards, err := storage.GetBoardsByUser(adminUser)
		require.NoError(t, err)

		orderedExpectedSNs := []domain.BoardShortName{boardASN, boardBSN, boardCSN} // Order of creation
		// Filter for boards created in this test, maintaining order
		var testSpecificBoards []domain.Board
		for _, b := range boards {
			if _, ok := createdBoards[b.ShortName]; ok {
				testSpecificBoards = append(testSpecificBoards, b)
			}
		}

		require.Equal(t, len(orderedExpectedSNs), len(testSpecificBoards), "Admin should see all 3 test-created boards")
		for i, sn := range orderedExpectedSNs {
			require.Equal(t, sn, testSpecificBoards[i].ShortName, "Admin: board order mismatch at index %d", i)
			verifyFullBoardMetadata(t, testSpecificBoards[i])
		}
	})

	t.Run("non-admin user on domain1.com gets relevant boards with full metadata", func(t *testing.T) {
		boards, err := storage.GetBoardsByUser(userDomain1)
		require.NoError(t, err)

		orderedExpectedSNs := []domain.BoardShortName{boardASN, boardBSN} // Order of creation
		// Filter for boards created in this test, maintaining order
		var testSpecificBoards []domain.Board
		for _, b := range boards {
			if _, ok := createdBoards[b.ShortName]; ok {
				testSpecificBoards = append(testSpecificBoards, b)
			}
		}

		require.Equal(t, len(orderedExpectedSNs), len(testSpecificBoards), "User from domain1.com should see 2 specific boards")
		for i, sn := range orderedExpectedSNs {
			require.Equal(t, sn, testSpecificBoards[i].ShortName, "User on domain1.com: board order mismatch at index %d", i)
			verifyFullBoardMetadata(t, testSpecificBoards[i])
		}

		// Ensure Board C is not present
		foundC := false
		for _, b := range boards {
			if b.ShortName == boardCSN {
				foundC = true
				break
			}
		}
		assert.False(t, foundC, "User from domain1.com should NOT see board C")
	})

	t.Run("non-admin user on domain2.com gets relevant boards with full metadata", func(t *testing.T) {
		boards, err := storage.GetBoardsByUser(userDomain2)
		require.NoError(t, err)

		orderedExpectedSNs := []domain.BoardShortName{boardASN, boardCSN} // Order of creation
		// Filter for boards created in this test, maintaining order
		var testSpecificBoards []domain.Board
		for _, b := range boards {
			if _, ok := createdBoards[b.ShortName]; ok {
				testSpecificBoards = append(testSpecificBoards, b)
			}
		}

		require.Equal(t, len(orderedExpectedSNs), len(testSpecificBoards), "User from domain2.com should see 2 specific boards")
		for i, sn := range orderedExpectedSNs {
			require.Equal(t, sn, testSpecificBoards[i].ShortName, "User on domain2.com: board order mismatch at index %d", i)
			verifyFullBoardMetadata(t, testSpecificBoards[i])
		}

		// Ensure Board B is not present
		foundB := false
		for _, b := range boards {
			if b.ShortName == boardBSN {
				foundB = true
				break
			}
		}
		assert.False(t, foundB, "User from domain2.com should NOT see board B")
	})

	t.Run("non-admin user on other.com (sees only public) with full metadata", func(t *testing.T) {
		boards, err := storage.GetBoardsByUser(userOtherDomain)
		require.NoError(t, err)

		var testSpecificBoardA *domain.Board
		for i := range boards { // Iterate by index to get a pointer
			if boards[i].ShortName == boardASN {
				testSpecificBoardA = &boards[i]
				break
			}
		}
		require.NotNil(t, testSpecificBoardA, "User from other.com should see public board A")
		verifyFullBoardMetadata(t, *testSpecificBoardA)

		// Ensure Board B and C are not present
		foundB := false
		for _, b := range boards {
			if b.ShortName == boardBSN {
				foundB = true
				break
			}
		}
		assert.False(t, foundB, "User from other.com should not see board B")
		foundC := false
		for _, b := range boards {
			if b.ShortName == boardCSN {
				foundC = true
				break
			}
		}
		assert.False(t, foundC, "User from other.com should not see board C")
	})

	t.Run("user with malformed email (domain extraction fails)", func(t *testing.T) {
		userMalformedEmail := domain.User{Id: 105, Email: "malformedemail", Admin: false} // No @ symbol
		_, err := storage.GetBoardsByUser(userMalformedEmail)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Cant get user domain")                  // Error from domain.User.EmailDomain()
		assert.Contains(t, err.Error(), "could not determine user email domain") // Wrapper error from GetBoardsByUser
	})

	t.Run("user with email domain that matches no restricted boards", func(t *testing.T) {
		// This user should only see public boards (like boardASN) if any exist.
		// They should not see boardBSN or boardCSN.
		// An isolated board is also created to ensure it's not seen.
		isolatedBoardSN := generateString(t)
		isolatedBoardName := "Isolated Board For NoAccess Test"
		allowedIsolated := &domain.Emails{"isolated-domain.com"}
		err := storage.CreateBoard(domain.BoardCreationData{Name: isolatedBoardName, ShortName: isolatedBoardSN, AllowedEmails: allowedIsolated})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, storage.DeleteBoard(isolatedBoardSN)) })

		userNoAccessToRestricted := domain.User{Id: 106, Email: "nobody@nowhere.com", Admin: false}
		boards, err := storage.GetBoardsByUser(userNoAccessToRestricted)
		require.NoError(t, err)

		foundIsolated := false
		var foundBoardA *domain.Board
		for i := range boards {
			if boards[i].ShortName == isolatedBoardSN {
				foundIsolated = true
			}
			if boards[i].ShortName == boardASN { // Check if public board A is seen
				foundBoardA = &boards[i]
			}
		}
		assert.False(t, foundIsolated, "User with no matching domain should not see the isolated board")

		// This user *should* see public board A.
		require.NotNil(t, foundBoardA, "User with no domain match should still see public board A")
		if foundBoardA != nil {
			verifyFullBoardMetadata(t, *foundBoardA)
		}
	})
}

// TestThreadCount verifies counting threads on a board.
func TestThreadCount(t *testing.T) {
	boardA := setupBoard(t)
	boardB := setupBoard(t)

	t.Run("EmptyBoard", func(t *testing.T) {
		count, err := storage.ThreadCount(boardA)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Count should be 0 for an empty board")
	})

	t.Run("OneThread", func(t *testing.T) {
		opMsg := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP A1"}
		threadData := domain.ThreadCreationData{Title: "Thread Count 1", Board: boardA, OpMessage: opMsg}
		threadID := createTestThread(t, threadData)
		// No t.Cleanup for threadID here, board cleanup will handle it.

		count, err := storage.ThreadCount(boardA)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Count should be 1")
		// Deleting thread explicitly to reset count for next subtest if needed, though board cleanup is primary.
		err = storage.DeleteThread(boardA, threadID)
		require.NoError(t, err)
	})

	t.Run("MultipleThreads", func(t *testing.T) {
		op1 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP A1"}
		td1 := domain.ThreadCreationData{Title: "Thread Count A1", Board: boardA, OpMessage: op1}
		tID1 := createTestThread(t, td1)

		op2 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 2}, Text: "OP A2"}
		td2 := domain.ThreadCreationData{Title: "Thread Count A2", Board: boardA, OpMessage: op2}
		tID2 := createTestThread(t, td2)

		op3 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 3}, Text: "OP A3"}
		td3 := domain.ThreadCreationData{Title: "Thread Count A3", Board: boardA, OpMessage: op3}
		tID3 := createTestThread(t, td3)

		count, err := storage.ThreadCount(boardA)
		require.NoError(t, err)
		assert.Equal(t, 3, count, "Count should be 3")

		// Clean up threads for this sub-test to ensure independence if boardA is reused.
		require.NoError(t, storage.DeleteThread(boardA, tID1))
		require.NoError(t, storage.DeleteThread(boardA, tID2))
		require.NoError(t, storage.DeleteThread(boardA, tID3))
	})

	t.Run("MultipleBoards", func(t *testing.T) {
		opA1 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP A1 Multi"}
		_ = createTestThread(t, domain.ThreadCreationData{Title: "Thread A1 Multi", Board: boardA, OpMessage: opA1})
		opA2 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 2}, Text: "OP A2 Multi"}
		_ = createTestThread(t, domain.ThreadCreationData{Title: "Thread A2 Multi", Board: boardA, OpMessage: opA2})

		opB1 := domain.MessageCreationData{Board: boardB, Author: domain.User{Id: 3}, Text: "OP B1 Multi"}
		_ = createTestThread(t, domain.ThreadCreationData{Title: "Thread B1 Multi", Board: boardB, OpMessage: opB1})

		countA, errA := storage.ThreadCount(boardA)
		require.NoError(t, errA)
		assert.Equal(t, 2, countA, "Count for board A should be 2")

		countB, errB := storage.ThreadCount(boardB)
		require.NoError(t, errB)
		assert.Equal(t, 1, countB, "Count for board B should be 1")
	})
}

// TestLastThreadId verifies finding the ID of the least recently bumped (oldest) non-sticky thread.
func TestLastThreadId(t *testing.T) {
	t.Run("NoThreads", func(t *testing.T) {
		boardA := setupBoard(t)
		_, err := storage.LastThreadId(boardA)
		requireNotFoundError(t, err)
	})

	t.Run("OneThread", func(t *testing.T) {
		boardA := setupBoard(t)
		opMsg := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP Single"}
		threadData := domain.ThreadCreationData{Title: "LastThreadId Single", Board: boardA, OpMessage: opMsg}
		tID := createTestThread(t, threadData)
		defer storage.DeleteThread(boardA, tID) // Defer ensures cleanup even on test failure within subtest

		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID, lastID, "Last ID should be the only thread's ID")
	})

	t.Run("MultipleThreadsNoBumps", func(t *testing.T) {
		boardA := setupBoard(t)

		op1 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP 1"}
		td1 := domain.ThreadCreationData{Title: "LastThreadId NoBump 1", Board: boardA, OpMessage: op1}
		tID1 := createTestThread(t, td1)
		defer storage.DeleteThread(boardA, tID1)
		time.Sleep(20 * time.Millisecond) // Ensure distinct creation/bump times

		op2 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 2}, Text: "OP 2"}
		td2 := domain.ThreadCreationData{Title: "LastThreadId NoBump 2", Board: boardA, OpMessage: op2}
		tID2 := createTestThread(t, td2)
		defer storage.DeleteThread(boardA, tID2)
		time.Sleep(20 * time.Millisecond)

		op3 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 3}, Text: "OP 3"}
		td3 := domain.ThreadCreationData{Title: "LastThreadId NoBump 3", Board: boardA, OpMessage: op3}
		tID3 := createTestThread(t, td3)
		defer storage.DeleteThread(boardA, tID3)

		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID1, lastID, "Last ID should be the oldest created thread (tID1)")
	})

	t.Run("MultipleThreadsWithBumps", func(t *testing.T) {
		boardA := setupBoard(t)
		boardB := setupBoard(t)

		op1 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP 1B"}
		tID1 := createTestThread(t, domain.ThreadCreationData{Title: "LastThreadId Bump 1", Board: boardA, OpMessage: op1})
		defer storage.DeleteThread(boardA, tID1)

		op2 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 2}, Text: "OP 2B"}
		tID2 := createTestThread(t, domain.ThreadCreationData{Title: "LastThreadId Bump 2", Board: boardA, OpMessage: op2})
		defer storage.DeleteThread(boardA, tID2)

		op3 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 3}, Text: "OP 3B"}
		tID3 := createTestThread(t, domain.ThreadCreationData{Title: "LastThreadId Bump 3", Board: boardB, OpMessage: op3})
		defer storage.DeleteThread(boardA, tID3)

		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID1, lastID, "Initially, oldest is tID1")

		_ = createTestMessage(t, domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 4}, Text: "Bump msg for tID1", ThreadId: tID1})

		lastID, err = storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID2, lastID, "After bumping tID1, oldest should be tID2")

		_ = createTestMessage(t, domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 5}, Text: "Bump msg for tID2", ThreadId: tID2})

		lastID, err = storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID3, lastID, "After bumping tID2, oldest should be tID3")
	})

	t.Run("MultipleBoardsIsolation", func(t *testing.T) {
		boardA := setupBoard(t)
		boardB := setupBoard(t)

		opA1 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP A1"}
		tA1 := createTestThread(t, domain.ThreadCreationData{Title: "LastThreadId Iso A1", Board: boardA, OpMessage: opA1})
		defer storage.DeleteThread(boardA, tA1)
		time.Sleep(20 * time.Millisecond)
		opA2 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 2}, Text: "OP A2"}
		tA2 := createTestThread(t, domain.ThreadCreationData{Title: "LastThreadId Iso A2", Board: boardA, OpMessage: opA2})
		defer storage.DeleteThread(boardA, tA2)

		opB1 := domain.MessageCreationData{Board: boardB, Author: domain.User{Id: 3}, Text: "OP B1"}
		tB1 := createTestThread(t, domain.ThreadCreationData{Title: "LastThreadId Iso B1", Board: boardB, OpMessage: opB1})
		defer storage.DeleteThread(boardB, tB1)

		lastIDA, errA := storage.LastThreadId(boardA)
		require.NoError(t, errA)
		assert.Equal(t, tA1, lastIDA, "Last ID for board A should be tA1")

		lastIDB, errB := storage.LastThreadId(boardB)
		require.NoError(t, errB)
		assert.Equal(t, tB1, lastIDB, "Last ID for board B should be tB1")
	})

	t.Run("StickyIgnored", func(t *testing.T) {
		boardA := setupBoard(t)

		opS1 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP Sticky 1"}
		tID1 := createTestThread(t, domain.ThreadCreationData{Title: "LastThreadId Sticky 1", Board: boardA, OpMessage: opS1})
		defer storage.DeleteThread(boardA, tID1) // Defer ensures cleanup

		opNS2 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 2}, Text: "OP NonSticky 2"}
		tID2 := createTestThread(t, domain.ThreadCreationData{Title: "LastThreadId NonSticky 2", Board: boardA, OpMessage: opNS2})
		defer storage.DeleteThread(boardA, tID2) // Defer ensures cleanup

		// Manually set thread 1 sticky
		// Note: Thread creation timestamp (last_bump_ts by default) for tID1 is older than tID2
		_, err := storage.db.Exec("UPDATE threads SET is_sticky = TRUE WHERE id = $1 AND board = $2", tID1, boardA)
		require.NoError(t, err, "Failed to manually set thread 1 sticky")

		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID2, lastID, "LastThreadId should ignore sticky thread tID1 and return non-sticky tID2")

		// Set thread 2 sticky as well
		_, err = storage.db.Exec("UPDATE threads SET is_sticky = TRUE WHERE id = $1 AND board = $2", tID2, boardA)
		require.NoError(t, err, "Failed to manually set thread 2 sticky")

		// Now all (test-specific) threads on boardA are sticky, should get 404
		_, err = storage.LastThreadId(boardA)
		requireNotFoundError(t, err) // Expect "No non-sticky threads found"
	})
}
