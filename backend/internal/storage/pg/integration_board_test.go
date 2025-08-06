package pg

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper functions for test readability

// assertBoardInActiveList checks if a board with given shortName is present in the boards list
func assertBoardInActiveList(t *testing.T, boards []domain.Board, shortName domain.BoardShortName, shouldExist bool, msgAndArgs ...interface{}) {
	t.Helper()
	found := false
	for _, b := range boards {
		if b.ShortName == shortName {
			found = true
			break
		}
	}
	if shouldExist {
		assert.True(t, found, msgAndArgs...)
	} else {
		assert.False(t, found, msgAndArgs...)
	}
}

// setBoardActivityTime sets the last_activity timestamp for a board
func setBoardActivityTime(t *testing.T, storage *Storage, boardShortName domain.BoardShortName, activityTime time.Time) {
	t.Helper()
	_, err := storage.db.Exec("UPDATE boards SET last_activity = $1 WHERE short_name = $2", activityTime, boardShortName)
	require.NoError(t, err, "Failed to set board activity time")
}

// TestCreateBoard verifies the board creation logic.
func TestCreateBoard(t *testing.T) {
	t.Run("success with allowed emails", func(t *testing.T) {
		boardName := "Test Create Board"
		bShortName := generateString(t)
		allowedEmails := &domain.Emails{"test@example.com"}

		err := storage.CreateBoard(domain.BoardCreationData{
			Name:          boardName,
			ShortName:     bShortName,
			AllowedEmails: allowedEmails,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, storage.DeleteBoard(bShortName))
		})
	})

	t.Run("success without allowed emails", func(t *testing.T) {
		boardName := "Test Create Board"
		bShortName := generateString(t)

		err := storage.CreateBoard(domain.BoardCreationData{
			Name:          boardName,
			ShortName:     bShortName,
			AllowedEmails: nil,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, storage.DeleteBoard(bShortName))
		})
	})

	t.Run("fails on duplicate short name", func(t *testing.T) {
		boardShortName := generateString(t)
		boardData := domain.BoardCreationData{
			Name:          "Test Board",
			ShortName:     boardShortName,
			AllowedEmails: nil,
		}

		err := storage.CreateBoard(boardData)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, storage.DeleteBoard(boardShortName))
		})

		// Attempt to create again with same short name
		boardData.Name = "Another Name"
		err = storage.CreateBoard(boardData)
		require.Error(t, err, "Creating board with duplicate short name should fail")
		assert.Contains(t, err.Error(), "possibly duplicate short_name")
	})

	t.Run("fails on empty allowed emails", func(t *testing.T) {
		bShortName := generateString(t)
		err := storage.CreateBoard(domain.BoardCreationData{
			Name:          "Test Board",
			ShortName:     bShortName,
			AllowedEmails: &domain.Emails{}, // Empty slice
		})
		require.ErrorIs(t, err, emptyAllowedEmailsError)
		_ = storage.DeleteBoard(bShortName) // Cleanup attempt
	})
}

// TestGetBoard verifies retrieving board details.
func TestGetBoard(t *testing.T) {
	t.Run("success with metadata validation", func(t *testing.T) {
		boardName := "Test Get Board"
		allowedEmails := &domain.Emails{"@test.ru", "@test2.ru"}
		boardShortName := generateString(t)
		testBegins := time.Now().UTC()

		err := storage.CreateBoard(domain.BoardCreationData{
			Name:          boardName,
			ShortName:     boardShortName,
			AllowedEmails: allowedEmails,
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, storage.DeleteBoard(boardShortName)) })

		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))

		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		assert.Equal(t, boardName, board.Name)
		assert.Equal(t, boardShortName, board.ShortName)
		assert.Equal(t, allowedEmails, board.AllowedEmails)
		assert.True(t, !board.CreatedAt.Before(testBegins), "Creation time should not be before test begins")
		assert.True(t, !board.LastActivity.Before(board.CreatedAt), "Last activity should not be before creation")
		assert.Empty(t, board.Threads, "Board should have no threads initially")
	})

	t.Run("fails for non-existent board", func(t *testing.T) {
		_, err := storage.GetBoard("nonexistentboard", 1)
		requireNotFoundError(t, err)
	})
}

// TestDeleteBoard verifies board deletion and cascading effects.
func TestDeleteBoard(t *testing.T) {
	t.Run("success with cascade cleanup", func(t *testing.T) {
		boardShortNameToTest, threadID, messageID := setupBoardAndThreadAndMessage(t)

		// Verify resources exist before deletion
		unquotedViewName := viewTableName(boardShortNameToTest)
		unquotedMsgTableName := messagesPartitionName(boardShortNameToTest)
		unquotedThreadsTableName := threadsPartitionName(boardShortNameToTest)

		var exists bool
		err := storage.db.QueryRow("SELECT EXISTS (SELECT FROM pg_matviews WHERE matviewname = $1)", unquotedViewName).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Materialized view should exist before deletion")

		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedMsgTableName).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Messages table should exist before deletion")

		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedThreadsTableName).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Threads table should exist before deletion")

		// Delete board
		err = storage.DeleteBoard(boardShortNameToTest)
		require.NoError(t, err)

		// Verify all resources are cleaned up
		_, err = storage.GetBoard(boardShortNameToTest, 1)
		requireNotFoundError(t, err)

		_, err = storage.GetThread(boardShortNameToTest, threadID)
		requireNotFoundError(t, err)

		_, err = storage.GetMessage(boardShortNameToTest, messageID)
		requireNotFoundError(t, err)

		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM pg_matviews WHERE matviewname = $1)", unquotedViewName).Scan(&exists)
		require.NoError(t, err)
		assert.False(t, exists, "Materialized view should be dropped")

		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedMsgTableName).Scan(&exists)
		require.NoError(t, err)
		assert.False(t, exists, "Messages table should be dropped")

		err = storage.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedThreadsTableName).Scan(&exists)
		require.NoError(t, err)
		assert.False(t, exists, "Threads table should be dropped")
	})

	t.Run("fails for non-existent board", func(t *testing.T) {
		err := storage.DeleteBoard("nonexistentboard_del_test")
		requireNotFoundError(t, err)
	})
}

// TestBoardWorkflow simulates a typical board usage: creating threads/messages and verifying pagination.
func TestBoardWorkflow(t *testing.T) {
	boardShortName := setupBoard(t)

	// Create test threads
	threadTitles := []string{"thread1", "thread2", "thread3", "thread4", "thread5"}
	threadIds := make([]domain.ThreadId, len(threadTitles))

	for i, title := range threadTitles {
		threadData := domain.ThreadCreationData{
			Title: title,
			Board: boardShortName,
			OpMessage: domain.MessageCreationData{
				Author: domain.User{Id: int64(200 + i)},
				Text:   fmt.Sprintf("op%d", i+1),
			},
		}
		threadIds[i] = createTestThread(t, threadData)
	}

	// Add messages to create different bump orders
	// thread4 gets msg -> most recent
	// thread1 gets 2 msgs -> second most recent
	// thread3 gets msg -> third most recent
	// thread2, thread5 stay at creation time (least recent)
	messages := []domain.MessageCreationData{
		{Board: boardShortName, Author: domain.User{Id: 300}, Text: "msg1_t1", ThreadId: threadIds[0]},
		{Board: boardShortName, Author: domain.User{Id: 301}, Text: "msg2_t2", ThreadId: threadIds[1]},
		{Board: boardShortName, Author: domain.User{Id: 302}, Text: "msg3_t3", ThreadId: threadIds[2]},
		{Board: boardShortName, Author: domain.User{Id: 303}, Text: "msg4_t1", Attachments: &domain.Attachments{"file.txt", "file2.png"}, ThreadId: threadIds[0]},
		{Board: boardShortName, Author: domain.User{Id: 304}, Text: "msg5_t4", ThreadId: threadIds[3]},
	}
	for _, msg := range messages {
		createTestMessage(t, msg)
	}

	t.Run("pagination and ordering", func(t *testing.T) {
		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))

		// Page 1: should show 3 threads in bump order
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		require.Len(t, board.Threads, 3, "Page 1 should show 3 threads")

		expectedOrder := []string{"thread4", "thread1", "thread3"}
		requireThreadOrder(t, board.Threads, expectedOrder)

		// Verify message content and order within threads
		requireMessageOrder(t, board.Threads[0].Messages, []string{"op4", "msg5_t4"})
		requireMessageOrder(t, board.Threads[1].Messages, []string{"op1", "msg1_t1", "msg4_t1"})
		requireMessageOrder(t, board.Threads[2].Messages, []string{"op3", "msg3_t3"})

		// Page 2: should show remaining 2 threads
		board, err = storage.GetBoard(boardShortName, 2)
		require.NoError(t, err)
		require.Len(t, board.Threads, 2, "Page 2 should show 2 threads")

		expectedOrder = []string{"thread2", "thread5"}
		requireThreadOrder(t, board.Threads, expectedOrder)
		requireMessageOrder(t, board.Threads[0].Messages, []string{"op2", "msg2_t2"})
		requireMessageOrder(t, board.Threads[1].Messages, []string{"op5"})
	})

	t.Run("bump limit enforcement", func(t *testing.T) {
		// Create a thread that will hit bump limit
		bumpLimitThread := domain.ThreadCreationData{
			Title: "Bump Limit Test",
			Board: boardShortName,
			OpMessage: domain.MessageCreationData{
				Author: domain.User{Id: 400},
				Text:   "OP Bump Limit",
			},
		}
		bumpLimitThreadID := createTestThread(t, bumpLimitThread)

		// Fill thread to bump limit
		for i := 0; i < storage.cfg.Public.BumpLimit; i++ {
			msgData := domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: 401},
				Text:     fmt.Sprintf("bump %d", i+1),
				ThreadId: bumpLimitThreadID,
			}
			createTestMessage(t, msgData)
		}

		// Bump other threads to verify ordering
		createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 402}, Text: "bump_other1", ThreadId: threadIds[4]}) // thread5
		createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 403}, Text: "bump_other2", ThreadId: threadIds[0]}) // thread1

		// Try to bump the bump-limit thread (should not affect order)
		createTestMessage(t, domain.MessageCreationData{Board: boardShortName, Author: domain.User{Id: 404}, Text: "post_after_limit", ThreadId: bumpLimitThreadID})

		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))
		board, err := storage.GetBoard(boardShortName, 1)
		require.NoError(t, err)
		require.Len(t, board.Threads, 3)

		// thread1 should be first (most recent bump), thread5 second, bump-limit thread third
		requireThreadOrder(t, board.Threads, []string{"thread1", "thread5", "Bump Limit Test"})
	})
}

// TestBoardInvariants performs stress testing and checks structural invariants.
func TestBoardInvariants(t *testing.T) {
	boardShortName := setupBoard(t)

	// Create multiple threads
	threadCount := storage.cfg.Public.ThreadsPerPage*2 + 1 // Enough for multiple pages
	createdThreadIDs := make([]domain.ThreadId, threadCount)

	for i := range createdThreadIDs {
		threadData := domain.ThreadCreationData{
			Title: fmt.Sprintf("Stress Thread %d", i+1),
			Board: boardShortName,
			OpMessage: domain.MessageCreationData{
				Author: domain.User{Id: int64(i%3 + 1)}, // Rotate between 3 users
				Text:   fmt.Sprintf("OP for thread %d", i+1),
			},
		}
		createdThreadIDs[i] = createTestThread(t, threadData)
	}

	// Add messages to various threads to test bumping
	r := rand.New(rand.NewSource(42)) // Fixed seed for reproducible tests
	messageCount := threadCount * 5   // 5 messages per thread on average

	for i := 0; i < messageCount; i++ {
		targetThreadID := createdThreadIDs[r.Intn(len(createdThreadIDs))]
		replyData := domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: int64(r.Intn(5) + 1)},
			Text:     fmt.Sprintf("Reply %d", i),
			ThreadId: targetThreadID,
		}
		createTestMessage(t, replyData)
	}

	// Calculate expected pages
	pages := (threadCount + storage.cfg.Public.ThreadsPerPage - 1) / storage.cfg.Public.ThreadsPerPage

	t.Run("structural invariants", func(t *testing.T) {
		require.NoError(t, storage.refreshMaterializedViewConcurrent(boardShortName, time.Second*1))

		for page := 1; page <= pages; page++ {
			checkBoardInvariants(t, boardShortName, page)
		}

		// Test empty page beyond valid range
		board, err := storage.GetBoard(boardShortName, pages+1)
		require.NoError(t, err)
		assert.Empty(t, board.Threads, "Page beyond valid range should be empty")
	})
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
	t.Run("newly created board is active", func(t *testing.T) {
		boardShortName := setupBoard(t)

		boards, err := storage.GetActiveBoards(1 * time.Hour)
		require.NoError(t, err)
		assertBoardInActiveList(t, boards, boardShortName, true, "Newly created board should be active")
	})

	t.Run("respects activity interval", func(t *testing.T) {
		boardShortName := setupBoard(t)

		// Set activity to 30 minutes ago
		setBoardActivityTime(t, storage, boardShortName, time.Now().UTC().Add(-30*time.Minute))

		// Should be active for 31 minute interval
		boards, err := storage.GetActiveBoards(31 * time.Minute)
		require.NoError(t, err)
		assertBoardInActiveList(t, boards, boardShortName, true, "Board should be active for interval > 30 mins")

		// Should be inactive for 29 minute interval
		boards, err = storage.GetActiveBoards(29 * time.Minute)
		require.NoError(t, err)
		assertBoardInActiveList(t, boards, boardShortName, false, "Board should be inactive for interval < 30 mins")
	})

	t.Run("activity updates on message creation", func(t *testing.T) {
		boardShortName := setupBoard(t)

		// Create a thread first
		threadData := domain.ThreadCreationData{
			Title: "Thread for message creation test",
			Board: boardShortName,
			OpMessage: domain.MessageCreationData{
				Board:  boardShortName,
				Author: domain.User{Id: 1},
				Text:   "OP for message test",
			},
		}
		threadId := createTestThread(t, threadData)

		// Make board inactive
		setBoardActivityTime(t, storage, boardShortName, time.Now().UTC().Add(-10*time.Minute))

		boards, err := storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		assertBoardInActiveList(t, boards, boardShortName, false, "Board should be inactive initially")

		// Create message to reactivate board
		msgData := domain.MessageCreationData{
			Board:    boardShortName,
			Author:   domain.User{Id: 1},
			Text:     "test msg for activity",
			ThreadId: threadId,
		}
		_ = createTestMessage(t, msgData)

		boards, err = storage.GetActiveBoards(5 * time.Minute)
		require.NoError(t, err)
		assertBoardInActiveList(t, boards, boardShortName, true, "Board should become active after message creation")
	})

	t.Run("activity updates on operations", func(t *testing.T) {
		// Test that all CRUD operations update activity
		testCases := []struct {
			name   string
			action func(domain.BoardShortName) error
		}{
			{
				name: "thread creation",
				action: func(boardShortName domain.BoardShortName) error {
					opMsg := domain.MessageCreationData{
						Board:  boardShortName,
						Author: domain.User{Id: 1},
						Text:   "op msg activity",
					}
					threadData := domain.ThreadCreationData{
						Title:     "activity test thread",
						Board:     boardShortName,
						OpMessage: opMsg,
					}
					_ = createTestThread(t, threadData)
					return nil
				},
			},
			{
				name: "message deletion",
				action: func(boardShortName domain.BoardShortName) error {
					// Create a thread and message to delete within this board
					threadData := domain.ThreadCreationData{
						Title: "Thread for deletion test",
						Board: boardShortName,
						OpMessage: domain.MessageCreationData{
							Board:  boardShortName,
							Author: domain.User{Id: 1},
							Text:   "OP for deletion test",
						},
					}
					threadId := createTestThread(t, threadData)

					msgData := domain.MessageCreationData{
						Board:    boardShortName,
						Author:   domain.User{Id: 2},
						Text:     "Message to delete",
						ThreadId: threadId,
					}
					msgId := createTestMessage(t, msgData)

					return storage.DeleteMessage(boardShortName, msgId)
				},
			},
			{
				name: "thread deletion",
				action: func(boardShortName domain.BoardShortName) error {
					// Create a thread to delete within this board
					threadData := domain.ThreadCreationData{
						Title: "Thread for deletion test",
						Board: boardShortName,
						OpMessage: domain.MessageCreationData{
							Board:  boardShortName,
							Author: domain.User{Id: 1},
							Text:   "OP for deletion test",
						},
					}
					threadId := createTestThread(t, threadData)

					return storage.DeleteThread(boardShortName, threadId)
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				boardShortName := setupBoard(t)

				// Make board inactive
				setBoardActivityTime(t, storage, boardShortName, time.Now().UTC().Add(-10*time.Minute))

				boards, err := storage.GetActiveBoards(5 * time.Minute)
				require.NoError(t, err)
				assertBoardInActiveList(t, boards, boardShortName, false, "Board should be inactive before operation")

				// Perform action
				err = tc.action(boardShortName)
				require.NoError(t, err)

				boards, err = storage.GetActiveBoards(5 * time.Minute)
				require.NoError(t, err)
				assertBoardInActiveList(t, boards, boardShortName, true, "Board should become active after %s", tc.name)
			})
		}
	})
}

// TestGetBoards verifies retrieval of all board metadata.
func TestGetBoards(t *testing.T) {
	t.Run("returns boards in creation order", func(t *testing.T) {
		// Get initial state to calculate delta
		initialBoards, err := storage.GetBoards()
		require.NoError(t, err)

		// Create test boards
		createdBoards := []domain.BoardCreationData{
			{Name: "Board Alpha", ShortName: generateString(t), AllowedEmails: &domain.Emails{"one@example.com"}},
			{Name: "Board Beta", ShortName: generateString(t), AllowedEmails: nil},
			{Name: "Board Gamma", ShortName: generateString(t), AllowedEmails: &domain.Emails{"three@example.com", "another@example.com"}},
		}

		createdMap := make(map[domain.BoardShortName]domain.BoardCreationData)
		var expectedOrder []domain.BoardShortName

		for _, board := range createdBoards {
			err = storage.CreateBoard(board)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, storage.DeleteBoard(board.ShortName)) })
			createdMap[board.ShortName] = board
			expectedOrder = append(expectedOrder, board.ShortName)
		}

		allBoards, err := storage.GetBoards()
		require.NoError(t, err)
		require.Len(t, allBoards, len(initialBoards)+3, "Should retrieve all boards plus the 3 created")

		// Extract test boards maintaining their order
		var testBoards []domain.Board
		for _, b := range allBoards {
			if _, ok := createdMap[b.ShortName]; ok {
				testBoards = append(testBoards, b)

				// Verify metadata consistency with GetBoard
				board, err := storage.GetBoard(b.ShortName, 1)
				require.NoError(t, err)
				assert.Equal(t, board.BoardMetadata, b.BoardMetadata, "Metadata should match GetBoard result")
			}
		}

		require.Len(t, testBoards, 3, "Should find all 3 test boards")

		// Verify creation order is preserved
		for i, expectedSN := range expectedOrder {
			assert.Equal(t, expectedSN, testBoards[i].ShortName, "Board order mismatch at index %d", i)
		}
	})
}

// TestGetBoardsByUser verifies retrieval of boards based on user permissions.
func TestGetBoardsByUser(t *testing.T) {
	// Setup test boards with different access levels
	boardPublic := generateString(t)
	boardDomain1 := generateString(t)
	boardDomain2 := generateString(t)

	testBoards := []domain.BoardCreationData{
		{Name: "Public Board", ShortName: boardPublic, AllowedEmails: nil},
		{Name: "Domain1 Board", ShortName: boardDomain1, AllowedEmails: &domain.Emails{"domain1.com"}},
		{Name: "Domain2 Board", ShortName: boardDomain2, AllowedEmails: &domain.Emails{"domain2.com", "another.com"}},
	}

	for _, board := range testBoards {
		err := storage.CreateBoard(board)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, storage.DeleteBoard(board.ShortName)) })
	}

	// Helper to filter and verify test-specific boards
	filterTestBoards := func(boards []domain.Board) []domain.Board {
		var filtered []domain.Board
		testBoardNames := map[domain.BoardShortName]bool{
			boardPublic: true, boardDomain1: true, boardDomain2: true,
		}
		for _, b := range boards {
			if testBoardNames[b.ShortName] {
				filtered = append(filtered, b)
			}
		}
		return filtered
	}

	t.Run("admin sees all boards", func(t *testing.T) {
		adminUser := domain.User{Id: 101, Email: "admin@example.com", Admin: true}
		boards, err := storage.GetBoardsByUser(adminUser)
		require.NoError(t, err)

		testBoards := filterTestBoards(boards)
		assert.Len(t, testBoards, 3, "Admin should see all boards")

		// Verify order matches creation order
		expectedOrder := []domain.BoardShortName{boardPublic, boardDomain1, boardDomain2}
		for i, expectedSN := range expectedOrder {
			assert.Equal(t, expectedSN, testBoards[i].ShortName, "Admin board order mismatch")
		}
	})

	t.Run("user sees allowed boards only", func(t *testing.T) {
		testCases := []struct {
			name        string
			user        domain.User
			expectedSNs []domain.BoardShortName
		}{
			{
				name:        "domain1 user",
				user:        domain.User{Id: 102, Email: "user@domain1.com", Admin: false},
				expectedSNs: []domain.BoardShortName{boardPublic, boardDomain1},
			},
			{
				name:        "domain2 user",
				user:        domain.User{Id: 103, Email: "user@domain2.com", Admin: false},
				expectedSNs: []domain.BoardShortName{boardPublic, boardDomain2},
			},
			{
				name:        "other domain user",
				user:        domain.User{Id: 104, Email: "user@other.com", Admin: false},
				expectedSNs: []domain.BoardShortName{boardPublic},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				boards, err := storage.GetBoardsByUser(tc.user)
				require.NoError(t, err)

				testBoards := filterTestBoards(boards)
				require.Len(t, testBoards, len(tc.expectedSNs), "User should see expected number of boards")

				for i, expectedSN := range tc.expectedSNs {
					assert.Equal(t, expectedSN, testBoards[i].ShortName, "Board order mismatch for %s", tc.name)
				}
			})
		}
	})

	t.Run("malformed email fails", func(t *testing.T) {
		userMalformedEmail := domain.User{Id: 105, Email: "malformedemail", Admin: false}
		_, err := storage.GetBoardsByUser(userMalformedEmail)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not determine user email domain")
	})
}

// TestThreadCount verifies counting threads on a board.
func TestThreadCount(t *testing.T) {
	t.Run("counts correctly", func(t *testing.T) {
		boardA := setupBoard(t)
		boardB := setupBoard(t)

		// Empty board
		count, err := storage.ThreadCount(boardA)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Empty board should have 0 threads")

		// Add threads to boardA
		for i := 1; i <= 3; i++ {
			opMsg := domain.MessageCreationData{
				Board:  boardA,
				Author: domain.User{Id: int64(i)},
				Text:   fmt.Sprintf("OP %d", i),
			}
			threadData := domain.ThreadCreationData{
				Title:     fmt.Sprintf("Thread %d", i),
				Board:     boardA,
				OpMessage: opMsg,
			}
			_ = createTestThread(t, threadData)
		}

		// Add one thread to boardB
		opMsgB := domain.MessageCreationData{
			Board:  boardB,
			Author: domain.User{Id: 10},
			Text:   "OP B",
		}
		threadDataB := domain.ThreadCreationData{
			Title:     "Thread B",
			Board:     boardB,
			OpMessage: opMsgB,
		}
		_ = createTestThread(t, threadDataB)

		// Verify counts
		countA, err := storage.ThreadCount(boardA)
		require.NoError(t, err)
		assert.Equal(t, 3, countA, "Board A should have 3 threads")

		countB, err := storage.ThreadCount(boardB)
		require.NoError(t, err)
		assert.Equal(t, 1, countB, "Board B should have 1 thread")
	})
}

// TestLastThreadId verifies finding the ID of the least recently bumped (oldest) non-sticky thread.
func TestLastThreadId(t *testing.T) {
	t.Run("fails for empty board", func(t *testing.T) {
		boardA := setupBoard(t)
		_, err := storage.LastThreadId(boardA)
		requireNotFoundError(t, err)
	})

	t.Run("returns only thread", func(t *testing.T) {
		boardA := setupBoard(t)
		opMsg := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP Single"}
		threadData := domain.ThreadCreationData{Title: "Single Thread", Board: boardA, OpMessage: opMsg}
		tID := createTestThread(t, threadData)

		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID, lastID, "Should return the only thread ID")
	})

	t.Run("returns oldest unbumped thread", func(t *testing.T) {
		boardA := setupBoard(t)

		// Create threads with delays to ensure different creation times
		var threadIDs []domain.ThreadId
		for i := 1; i <= 3; i++ {
			opMsg := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: int64(i)}, Text: fmt.Sprintf("OP %d", i)}
			threadData := domain.ThreadCreationData{Title: fmt.Sprintf("Thread %d", i), Board: boardA, OpMessage: opMsg}
			tID := createTestThread(t, threadData)
			threadIDs = append(threadIDs, tID)
			time.Sleep(20 * time.Millisecond) // Ensure distinct creation times
		}

		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, threadIDs[0], lastID, "Should return oldest thread ID")
	})

	t.Run("updates after bumping", func(t *testing.T) {
		boardA := setupBoard(t)

		// Create two threads
		op1 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP 1"}
		tID1 := createTestThread(t, domain.ThreadCreationData{Title: "Thread 1", Board: boardA, OpMessage: op1})
		time.Sleep(20 * time.Millisecond)

		op2 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 2}, Text: "OP 2"}
		tID2 := createTestThread(t, domain.ThreadCreationData{Title: "Thread 2", Board: boardA, OpMessage: op2})

		// Initially, oldest should be tID1
		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID1, lastID, "Initially oldest should be tID1")

		// Bump tID1 by adding a message
		_ = createTestMessage(t, domain.MessageCreationData{
			Board:    boardA,
			Author:   domain.User{Id: 3},
			Text:     "Bump message",
			ThreadId: tID1,
		})

		// Now oldest should be tID2
		lastID, err = storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID2, lastID, "After bumping tID1, oldest should be tID2")
	})

	t.Run("ignores sticky threads", func(t *testing.T) {
		boardA := setupBoard(t)

		// Create two threads
		op1 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP Sticky"}
		tID1 := createTestThread(t, domain.ThreadCreationData{Title: "Sticky Thread", Board: boardA, OpMessage: op1})
		time.Sleep(20 * time.Millisecond)

		op2 := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 2}, Text: "OP Normal"}
		tID2 := createTestThread(t, domain.ThreadCreationData{Title: "Normal Thread", Board: boardA, OpMessage: op2})

		// Make first thread sticky
		_, err := storage.db.Exec("UPDATE threads SET is_sticky = TRUE WHERE id = $1 AND board = $2", tID1, boardA)
		require.NoError(t, err)

		// Should return tID2 (non-sticky) even though tID1 is older
		lastID, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tID2, lastID, "Should ignore sticky thread and return non-sticky thread")

		// Make second thread sticky too
		_, err = storage.db.Exec("UPDATE threads SET is_sticky = TRUE WHERE id = $1 AND board = $2", tID2, boardA)
		require.NoError(t, err)

		// Should fail when all threads are sticky
		_, err = storage.LastThreadId(boardA)
		requireNotFoundError(t, err)
	})

	t.Run("board isolation", func(t *testing.T) {
		boardA := setupBoard(t)
		boardB := setupBoard(t)

		// Create threads on both boards
		opA := domain.MessageCreationData{Board: boardA, Author: domain.User{Id: 1}, Text: "OP A"}
		tA := createTestThread(t, domain.ThreadCreationData{Title: "Thread A", Board: boardA, OpMessage: opA})

		opB := domain.MessageCreationData{Board: boardB, Author: domain.User{Id: 2}, Text: "OP B"}
		tB := createTestThread(t, domain.ThreadCreationData{Title: "Thread B", Board: boardB, OpMessage: opB})

		// Each board should return its own thread
		lastIDA, err := storage.LastThreadId(boardA)
		require.NoError(t, err)
		assert.Equal(t, tA, lastIDA, "Board A should return its own thread")

		lastIDB, err := storage.LastThreadId(boardB)
		require.NoError(t, err)
		assert.Equal(t, tB, lastIDB, "Board B should return its own thread")
	})
}
