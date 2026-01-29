//go:build !polluting

package pg

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	sharedstorage "github.com/itchan-dev/itchan/shared/storage/pg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBoardOperations verifies board CRUD operations and related queries.
// Uses transactional testing for complete isolation.
func TestBoardOperations(t *testing.T) {
	// =========================================================================
	// Test: CreateBoard
	// Verifies board creation with various configurations and error handling.
	// =========================================================================
	t.Run("CreateBoard", func(t *testing.T) {
		t.Run("success with allowed emails", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardName := "Test Create Board"
			bShortName := domain.BoardShortName(generateString(t))
			allowedEmails := &domain.Emails{"test@example.com"}

			err := storage.createBoard(tx, domain.BoardCreationData{
				Name:          boardName,
				ShortName:     bShortName,
				AllowedEmails: allowedEmails,
			})
			require.NoError(t, err)
		})

		t.Run("success without allowed emails", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardName := "Test Create Board"
			bShortName := domain.BoardShortName(generateString(t))

			err := storage.createBoard(tx, domain.BoardCreationData{
				Name:          boardName,
				ShortName:     bShortName,
				AllowedEmails: nil,
			})
			require.NoError(t, err)
		})

		t.Run("fails on duplicate short name", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardShortName := domain.BoardShortName(generateString(t))
			boardData := domain.BoardCreationData{
				Name:          "Test Board",
				ShortName:     boardShortName,
				AllowedEmails: nil,
			}

			err := storage.createBoard(tx, boardData)
			require.NoError(t, err)

			boardData.Name = "Another Name"
			err = storage.createBoard(tx, boardData)
			require.Error(t, err, "Creating board with duplicate short name should fail")
			assert.Contains(t, err.Error(), "already exists")
		})

		t.Run("fails on empty allowed emails", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			bShortName := domain.BoardShortName(generateString(t))
			err := storage.createBoard(tx, domain.BoardCreationData{
				Name:          "Test Board",
				ShortName:     bShortName,
				AllowedEmails: &domain.Emails{},
			})
			require.ErrorIs(t, err, emptyAllowedEmailsError)
		})
	})

	// =========================================================================
	// Test: GetBoard
	// Verifies board retrieval accuracy.
	// =========================================================================
	t.Run("GetBoard", func(t *testing.T) {
		t.Run("retrieves board with correct metadata", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardShortName := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardShortName)

			boardMetadata, err := storage.getBoard(tx, boardShortName, 1)
			require.NoError(t, err)
			assert.Equal(t, "Test Board "+string(boardShortName), boardMetadata.BoardMetadata.Name)
			assert.Equal(t, boardShortName, boardMetadata.BoardMetadata.ShortName)
		})

		t.Run("fails for non-existent board", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			_, err := storage.getBoard(tx, "nonexistentboard", 1)
			requireNotFoundError(t, err)
		})
	})

	// =========================================================================
	// Test: DeleteBoard
	// Verifies board deletion and cascading effects.
	// =========================================================================
	t.Run("DeleteBoard", func(t *testing.T) {
		t.Run("success with cascade cleanup", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardShortName := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardShortName)

			userID := createTestUser(t, tx, generateString(t)+"@example.com")

			threadID, _ := createTestThread(t, tx, domain.ThreadCreationData{
				Title: "Test Thread",
				Board: boardShortName,
				OpMessage: domain.MessageCreationData{
					Board:  boardShortName,
					Author: domain.User{Id: userID},
					Text:   "OP",
				},
			})

			messageID := createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Test Message",
				ThreadId: threadID,
			})

			// Add attachments to message
			attachments := getRandomAttachments(t)
			err := storage.addAttachments(tx, boardShortName, threadID, messageID, attachments)
			require.NoError(t, err)

			// Get file IDs before deletion
			attachmentsBefore, err := storage.getMessageAttachments(tx, boardShortName, threadID, messageID)
			require.NoError(t, err)
			var fileIDs []int64
			for _, att := range attachmentsBefore {
				fileIDs = append(fileIDs, att.FileId)
			}

			unquotedViewName := sharedstorage.ViewTableNameUnquoted(boardShortName)
			unquotedMsgTableName := sharedstorage.PartitionNameUnquoted(boardShortName, "messages")
			unquotedThreadsTableName := sharedstorage.PartitionNameUnquoted(boardShortName, "threads")

			var exists bool
			err = tx.QueryRow("SELECT EXISTS (SELECT FROM pg_matviews WHERE matviewname = $1)", unquotedViewName).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists, "Materialized view should exist before deletion")

			err = tx.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedMsgTableName).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists, "Messages table should exist before deletion")

			err = tx.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedThreadsTableName).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists, "Threads table should exist before deletion")

			err = storage.deleteBoard(tx, boardShortName)
			require.NoError(t, err)

			_, err = storage.getBoard(tx, boardShortName, 1)
			requireNotFoundError(t, err)

			_, err = storage.getThread(tx, boardShortName, threadID, 1)
			requireNotFoundError(t, err)

			_, err = storage.getMessage(tx, boardShortName, threadID, messageID)
			requireNotFoundError(t, err)

			err = tx.QueryRow("SELECT EXISTS (SELECT FROM pg_matviews WHERE matviewname = $1)", unquotedViewName).Scan(&exists)
			require.NoError(t, err)
			assert.False(t, exists, "Materialized view should be dropped")

			err = tx.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedMsgTableName).Scan(&exists)
			require.NoError(t, err)
			assert.False(t, exists, "Messages table should be dropped")

			err = tx.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", unquotedThreadsTableName).Scan(&exists)
			require.NoError(t, err)
			assert.False(t, exists, "Threads table should be dropped")

			// Verify files are deleted from files table
			for _, fileID := range fileIDs {
				var count int
				err = tx.QueryRow("SELECT COUNT(*) FROM files WHERE id = $1", fileID).Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 0, count, "File %d should be deleted from files table", fileID)
			}
		})

		t.Run("fails for non-existent board", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			err := storage.deleteBoard(tx, "nonexistentboard_del_test")
			requireNotFoundError(t, err)
		})
	})

	// =========================================================================
	// Test: GetBoards
	// Verifies retrieval of all board metadata.
	// =========================================================================
	t.Run("GetBoards", func(t *testing.T) {
		t.Run("returns boards in creation order", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			createdBoards := []domain.BoardCreationData{
				{Name: "Board Alpha", ShortName: "a", AllowedEmails: &domain.Emails{"one@example.com"}},
				{Name: "Board Beta", ShortName: "b", AllowedEmails: nil},
				{Name: "Board Gamma", ShortName: "c", AllowedEmails: &domain.Emails{"three@example.com", "another@example.com"}},
			}

			var expectedOrder []domain.BoardShortName
			for _, board := range createdBoards {
				err := storage.createBoard(tx, board)
				require.NoError(t, err)
				expectedOrder = append(expectedOrder, board.ShortName)
			}

			allBoards, err := storage.getBoards(tx)
			require.NoError(t, err)
			require.Len(t, allBoards, 3, "Should retrieve all 3 created boards")

			for i, expectedSN := range expectedOrder {
				assert.Equal(t, expectedSN, allBoards[i].ShortName, "Board order mismatch at index %d", i)
			}
		})
	})

	// =========================================================================
	// Test: GetBoardsByUser
	// Verifies retrieval of boards based on user permissions.
	// =========================================================================
	t.Run("GetBoardsByUser", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardPublic := domain.BoardShortName("a")
		boardDomain1 := domain.BoardShortName("b")
		boardDomain2 := domain.BoardShortName("c")

		testBoards := []domain.BoardCreationData{
			{Name: "Public Board", ShortName: boardPublic, AllowedEmails: nil},
			{Name: "Domain1 Board", ShortName: boardDomain1, AllowedEmails: &domain.Emails{"domain1.com"}},
			{Name: "Domain2 Board", ShortName: boardDomain2, AllowedEmails: &domain.Emails{"domain2.com", "another.com"}},
		}

		for _, board := range testBoards {
			err := storage.createBoard(tx, board)
			require.NoError(t, err)
		}

		t.Run("admin sees all boards", func(t *testing.T) {
			adminUser := domain.User{Id: 101, EmailDomain: "example.com", Admin: true}
			boards, err := storage.getBoardsByUser(tx, adminUser)
			require.NoError(t, err)
			assert.Len(t, boards, 3, "Admin should see all boards")

			expectedOrder := []domain.BoardShortName{boardPublic, boardDomain1, boardDomain2}
			for i, expectedSN := range expectedOrder {
				assert.Equal(t, expectedSN, boards[i].ShortName, "Admin board order mismatch")
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
					user:        domain.User{Id: 102, EmailDomain: "domain1.com", Admin: false},
					expectedSNs: []domain.BoardShortName{boardPublic, boardDomain1},
				},
				{
					name:        "domain2 user",
					user:        domain.User{Id: 103, EmailDomain: "domain2.com", Admin: false},
					expectedSNs: []domain.BoardShortName{boardPublic, boardDomain2},
				},
				{
					name:        "other domain user",
					user:        domain.User{Id: 104, EmailDomain: "other.com", Admin: false},
					expectedSNs: []domain.BoardShortName{boardPublic},
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					boards, err := storage.getBoardsByUser(tx, tc.user)
					require.NoError(t, err)
					require.Len(t, boards, len(tc.expectedSNs), "User should see expected number of boards")

					for i, expectedSN := range tc.expectedSNs {
						assert.Equal(t, expectedSN, boards[i].ShortName, "Board order mismatch for %s. Boards: %+v", tc.name, boards)
					}
				})
			}
		})

		t.Run("empty domain fails", func(t *testing.T) {
			userEmptyDomain := domain.User{Id: 105, EmailDomain: "", Admin: false}
			_, err := storage.getBoardsByUser(tx, userEmptyDomain)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "could not determine user email domain")
		})
	})

	// =========================================================================
	// Test: GetActiveBoards
	// Verifies retrieval of recently active boards based on activity timestamp.
	// =========================================================================
	t.Run("GetActiveBoards", func(t *testing.T) {
		t.Run("newly created board is active", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardShortName := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardShortName)

			boards, err := storage.getActiveBoards(tx, 1*time.Hour)
			require.NoError(t, err)

			found := false
			for _, board := range boards {
				if board.ShortName == boardShortName {
					found = true
					break
				}
			}
			assert.True(t, found, "Newly created board should be active")
		})

		t.Run("respects activity interval", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardShortName := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardShortName)

			_, err := tx.Exec("UPDATE boards SET last_activity_at = $1 WHERE short_name = $2",
				time.Now().UTC().Add(-30*time.Minute), boardShortName)
			require.NoError(t, err)

			boards, err := storage.getActiveBoards(tx, 31*time.Minute)
			require.NoError(t, err)
			found := false
			for _, board := range boards {
				if board.ShortName == boardShortName {
					found = true
					break
				}
			}
			assert.True(t, found, "Board should be active for interval > 30 mins")

			boards, err = storage.getActiveBoards(tx, 29*time.Minute)
			require.NoError(t, err)
			found = false
			for _, board := range boards {
				if board.ShortName == boardShortName {
					found = true
					break
				}
			}
			assert.False(t, found, "Board should be inactive for interval < 30 mins")
		})
	})

	// =========================================================================
	// Test: ThreadCount
	// Verifies counting threads on a board.
	// =========================================================================
	t.Run("ThreadCount", func(t *testing.T) {
		t.Run("counts correctly", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardA := domain.BoardShortName(generateString(t))
			boardB := domain.BoardShortName(generateString(t))

			createTestBoard(t, tx, boardA)
			createTestBoard(t, tx, boardB)

			count, err := storage.threadCount(tx, boardA)
			require.NoError(t, err)
			assert.Equal(t, 0, count, "Empty board should have 0 threads")

			userID := createTestUser(t, tx, generateString(t)+"@example.com")
			for i := 1; i <= 3; i++ {
				createTestThread(t, tx, domain.ThreadCreationData{
					Title: domain.ThreadTitle(fmt.Sprintf("Thread %d", i)),
					Board: boardA,
					OpMessage: domain.MessageCreationData{
						Board:  boardA,
						Author: domain.User{Id: userID},
						Text:   domain.MsgText(fmt.Sprintf("OP %d", i)),
					},
				})
			}

			createTestThread(t, tx, domain.ThreadCreationData{
				Title: "Thread B",
				Board: boardB,
				OpMessage: domain.MessageCreationData{
					Board:  boardB,
					Author: domain.User{Id: userID},
					Text:   "OP B",
				},
			})

			countA, err := storage.threadCount(tx, boardA)
			require.NoError(t, err)
			assert.Equal(t, 3, countA, "Board A should have 3 threads")

			countB, err := storage.threadCount(tx, boardB)
			require.NoError(t, err)
			assert.Equal(t, 1, countB, "Board B should have 1 thread")
		})
	})

	// =========================================================================
	// Test: LastThreadId
	// Verifies finding the ID of the least recently bumped (oldest) non-pinned thread.
	// =========================================================================
	t.Run("LastThreadId", func(t *testing.T) {
		t.Run("fails for empty board", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardA := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardA)

			_, err := storage.lastThreadId(tx, boardA)
			requireNotFoundError(t, err)
		})

		t.Run("returns only thread", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardA := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardA)
			userID := createTestUser(t, tx, generateString(t)+"@example.com")

			tID, _ := createTestThread(t, tx, domain.ThreadCreationData{
				Title: "Single Thread",
				Board: boardA,
				OpMessage: domain.MessageCreationData{
					Board:  boardA,
					Author: domain.User{Id: userID},
					Text:   "OP Single",
				},
			})

			lastID, err := storage.lastThreadId(tx, boardA)
			require.NoError(t, err)
			assert.Equal(t, tID, lastID, "Should return the only thread ID")
		})

		t.Run("returns oldest unbumped thread", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardA := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardA)
			userID := createTestUser(t, tx, generateString(t)+"@example.com")

			var threadIDs []domain.ThreadId
			for i := 1; i <= 3; i++ {
				tID, _ := createTestThread(t, tx, domain.ThreadCreationData{
					Title: domain.ThreadTitle(fmt.Sprintf("Thread %d", i)),
					Board: boardA,
					OpMessage: domain.MessageCreationData{
						Board:  boardA,
						Author: domain.User{Id: userID},
						Text:   domain.MsgText(fmt.Sprintf("OP %d", i)),
					},
				})
				threadIDs = append(threadIDs, tID)
				time.Sleep(20 * time.Millisecond)
			}

			lastID, err := storage.lastThreadId(tx, boardA)
			require.NoError(t, err)
			assert.Equal(t, threadIDs[0], lastID, "Should return oldest thread ID")
		})

		t.Run("ignores pinned threads", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardA := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardA)
			userID := createTestUser(t, tx, generateString(t)+"@example.com")

			tID1, _ := createTestThread(t, tx, domain.ThreadCreationData{
				Title:    "Pinned Thread",
				Board:    boardA,
				IsPinned: true,
				OpMessage: domain.MessageCreationData{
					Board:  boardA,
					Author: domain.User{Id: userID},
					Text:   "OP Pinned",
				},
			})
			time.Sleep(20 * time.Millisecond)

			tID2, _ := createTestThread(t, tx, domain.ThreadCreationData{
				Title: "Normal Thread",
				Board: boardA,
				OpMessage: domain.MessageCreationData{
					Board:  boardA,
					Author: domain.User{Id: userID},
					Text:   "OP Normal",
				},
			})

			lastID, err := storage.lastThreadId(tx, boardA)
			require.NoError(t, err)
			assert.Equal(t, tID2, lastID, "Should ignore pinned thread and return non-pinned thread")

			_, err = tx.Exec("UPDATE threads SET is_pinned = TRUE WHERE id = $1 AND board = $2", tID2, boardA)
			require.NoError(t, err)

			_, err = storage.lastThreadId(tx, boardA)
			requireNotFoundError(t, err)

			_, err = storage.getThread(tx, boardA, tID1, 1)
			require.NoError(t, err)
		})

		t.Run("board isolation", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			boardA := domain.BoardShortName(generateString(t))
			boardB := domain.BoardShortName(generateString(t))
			createTestBoard(t, tx, boardA)
			createTestBoard(t, tx, boardB)

			userID := createTestUser(t, tx, generateString(t)+"@example.com")

			tA, _ := createTestThread(t, tx, domain.ThreadCreationData{
				Title: "Thread A",
				Board: boardA,
				OpMessage: domain.MessageCreationData{
					Board:  boardA,
					Author: domain.User{Id: userID},
					Text:   "OP A",
				},
			})

			tB, _ := createTestThread(t, tx, domain.ThreadCreationData{
				Title: "Thread B",
				Board: boardB,
				OpMessage: domain.MessageCreationData{
					Board:  boardB,
					Author: domain.User{Id: userID},
					Text:   "OP B",
				},
			})

			lastIDA, err := storage.lastThreadId(tx, boardA)
			require.NoError(t, err)
			assert.Equal(t, tA, lastIDA, "Board A should return its own thread")

			lastIDB, err := storage.lastThreadId(tx, boardB)
			require.NoError(t, err)
			assert.Equal(t, tB, lastIDB, "Board B should return its own thread")
		})
	})
}

// TestBoardViewWorkflow verifies board view operations with materialized view refresh.
// Uses transactional testing with non-concurrent refresh to access uncommitted data.
func TestBoardViewWorkflow(t *testing.T) {
	t.Run("pagination and ordering", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName(generateString(t))
		createTestBoard(t, tx, boardShortName)

		userID := createTestUser(t, tx, generateString(t)+"@example.com")

		// Use explicit timestamps to ensure deterministic ordering within transaction
		baseTime := time.Now().UTC()

		// Create test threads with explicit timestamps
		threadTitles := []string{"thread1", "thread2", "thread3", "thread4", "thread5"}
		threadIds := make([]domain.ThreadId, len(threadTitles))

		for i, title := range threadTitles {
			threadData := domain.ThreadCreationData{
				Title: domain.ThreadTitle(title),
				Board: boardShortName,
				OpMessage: domain.MessageCreationData{
					Board:  boardShortName,
					Author: domain.User{Id: userID},
					Text:   domain.MsgText(fmt.Sprintf("op%d", i+1)),
				},
			}
			tid, _ := createTestThread(t, tx, threadData)
			threadIds[i] = tid
		}

		// Add messages to create different bump orders
		messages := []struct {
			threadIdx int
			text      string
			hasAttach bool
		}{
			{0, "msg1_t1", false},
			{1, "msg2_t2", false},
			{2, "msg3_t3", false},
			{0, "msg4_t1", true}, // thread1 gets bumped again
			{3, "msg5_t4", false},
		}

		for i, msg := range messages {
			// Add distinct timestamps (1 second apart) to ensure proper bump ordering
			timestamp := baseTime.Add(time.Duration(i+10) * time.Second)
			msgData := domain.MessageCreationData{
				Board:     boardShortName,
				Author:    domain.User{Id: userID},
				Text:      domain.MsgText(msg.text),
				ThreadId:  threadIds[msg.threadIdx],
				CreatedAt: &timestamp,
			}
			if msg.hasAttach {
			}
			createTestMessage(t, tx, msgData)
		}

		// Refresh materialized view (non-concurrent to work within transaction)
		require.NoError(t, storage.refreshMaterializedView(tx, boardShortName))

		// Page 1: should show 3 threads in bump order
		board, err := storage.getBoard(tx, boardShortName, 1)
		require.NoError(t, err)
		require.Len(t, board.Threads, storage.cfg.Public.ThreadsPerPage, "Page 1 should show %d threads", storage.cfg.Public.ThreadsPerPage)

		expectedOrder := []string{"thread4", "thread1", "thread3"}
		requireThreadOrder(t, board.Threads, expectedOrder)

		// Verify message content and order within threads
		requireMessageOrder(t, board.Threads[0].Messages, []string{"op4", "msg5_t4"})
		requireMessageOrder(t, board.Threads[1].Messages, []string{"op1", "msg1_t1", "msg4_t1"})
		requireMessageOrder(t, board.Threads[2].Messages, []string{"op3", "msg3_t3"})

		// Page 2: should show remaining 2 threads
		board, err = storage.getBoard(tx, boardShortName, 2)
		require.NoError(t, err)
		require.Len(t, board.Threads, 2, "Page 2 should show 2 threads")

		expectedOrder = []string{"thread2", "thread5"}
		requireThreadOrder(t, board.Threads, expectedOrder)
		requireMessageOrder(t, board.Threads[0].Messages, []string{"op2", "msg2_t2"})
		requireMessageOrder(t, board.Threads[1].Messages, []string{"op5"})
	})

	t.Run("structural invariants", func(t *testing.T) {
		tx, cleanup := beginTx(t)
		defer cleanup()

		boardShortName := domain.BoardShortName(generateString(t))
		createTestBoard(t, tx, boardShortName)

		userID := createTestUser(t, tx, generateString(t)+"@example.com")

		// Create multiple threads
		threadCount := storage.cfg.Public.ThreadsPerPage*2 + 1
		createdThreadIDs := make([]domain.ThreadId, threadCount)

		for i := range createdThreadIDs {
			tid, _ := createTestThread(t, tx, domain.ThreadCreationData{
				Title: domain.ThreadTitle(fmt.Sprintf("Stress Thread %d", i+1)),
				Board: boardShortName,
				OpMessage: domain.MessageCreationData{
					Board:  boardShortName,
					Author: domain.User{Id: userID},
					Text:   domain.MsgText(fmt.Sprintf("OP for thread %d", i+1)),
				},
			})
			createdThreadIDs[i] = tid
		}

		// Add messages to various threads
		r := rand.New(rand.NewSource(42))
		messageCount := threadCount * 5

		for i := 0; i < messageCount; i++ {
			targetThreadID := createdThreadIDs[r.Intn(len(createdThreadIDs))]
			createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     domain.MsgText(fmt.Sprintf("Reply %d", i)),
				ThreadId: targetThreadID,
			})
		}

		// Refresh and check invariants (non-concurrent to work within transaction)
		require.NoError(t, storage.refreshMaterializedView(tx, boardShortName))

		// Calculate expected pages
		pages := (threadCount + storage.cfg.Public.ThreadsPerPage - 1) / storage.cfg.Public.ThreadsPerPage

		for page := 1; page <= pages; page++ {
			board, err := storage.getBoard(tx, boardShortName, page)
			require.NoError(t, err)
			require.NotEmpty(t, board.Threads, "Board threads shouldn't be empty")

			// Verify thread count per page
			assert.LessOrEqual(t, len(board.Threads), storage.cfg.Public.ThreadsPerPage,
				"Page %d thread count (%d) exceeds limit (%d)", page, len(board.Threads), storage.cfg.Public.ThreadsPerPage)

			// Verify thread ordering
			var lastBumped time.Time
			for i, thread := range board.Threads {
				if i > 0 {
					assert.False(t, thread.LastBumped.After(lastBumped),
						"Thread order incorrect at index %d on page %d", i, page)
				}
				lastBumped = thread.LastBumped

				// Verify message count per thread
				assert.LessOrEqual(t, len(thread.Messages), storage.cfg.Public.NLastMsg+1,
					"Message count (%d) exceeds limit (%d) in thread on page %d",
					len(thread.Messages), storage.cfg.Public.NLastMsg+1, page)

				// Verify OP is first
				if len(thread.Messages) > 0 {
					assert.True(t, thread.Messages[0].IsOp(), "First message must be OP on page %d", page)
				}
			}
		}

		// Test empty page beyond valid range
		board, err := storage.getBoard(tx, boardShortName, pages+1)
		require.NoError(t, err)
		assert.Empty(t, board.Threads, "Page beyond valid range should be empty")
	})
}
