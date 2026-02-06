//go:build !polluting

package pg

import (
	"fmt"
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBoardViewOperations verifies materialized view refresh functionality.
// Uses transactional testing with non-concurrent refresh for isolation.
func TestBoardViewOperations(t *testing.T) {
	t.Run("RefreshMaterializedView", func(t *testing.T) {
		t.Run("success with existing board", func(t *testing.T) {
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
					Text:   "Test OP",
				},
			})

			err := storage.refreshMaterializedView(tx, boardShortName)
			require.NoError(t, err)

			board, err := storage.getBoard(tx, boardShortName, 1)
			require.NoError(t, err)
			assert.Equal(t, boardShortName, board.BoardMetadata.ShortName)
			require.Len(t, board.Threads, 1)
			assert.Equal(t, threadID, board.Threads[0].Id)
		})

		t.Run("fails for nonexistent board", func(t *testing.T) {
			tx, cleanup := beginTx(t)
			defer cleanup()

			nonExistentBoard := domain.BoardShortName(generateString(t))
			err := storage.refreshMaterializedView(tx, nonExistentBoard)
			require.Error(t, err)
		})

		t.Run("respects NLastMsg limit", func(t *testing.T) {
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
					Text:   "Test OP",
				},
			})

			for i := 1; i <= storage.cfg.Public.NLastMsg; i++ {
				createTestMessage(t, tx, domain.MessageCreationData{
					Board:    boardShortName,
					Author:   domain.User{Id: userID},
					Text:     domain.MsgText(fmt.Sprintf("Reply %d", i)),
					ThreadId: threadID,
				})
			}

			require.NoError(t, storage.refreshMaterializedView(tx, boardShortName))

			board, err := storage.getBoard(tx, boardShortName, 1)
			require.NoError(t, err)
			require.Len(t, board.Threads, 1)
			require.Len(t, board.Threads[0].Messages, storage.cfg.Public.NLastMsg+1)

			expectedTexts := []string{"Test OP", "Reply 1", "Reply 2", "Reply 3"}
			requireMessageOrder(t, board.Threads[0].Messages, expectedTexts)
		})

		t.Run("maintains limit when adding new messages", func(t *testing.T) {
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
					Text:   "Test OP",
				},
			})

			for i := 1; i <= storage.cfg.Public.NLastMsg; i++ {
				createTestMessage(t, tx, domain.MessageCreationData{
					Board:    boardShortName,
					Author:   domain.User{Id: userID},
					Text:     domain.MsgText(fmt.Sprintf("Reply %d", i)),
					ThreadId: threadID,
				})
			}

			require.NoError(t, storage.refreshMaterializedView(tx, boardShortName))

			createTestMessage(t, tx, domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Reply 4",
				ThreadId: threadID,
			})

			require.NoError(t, storage.refreshMaterializedView(tx, boardShortName))
			board, err := storage.getBoard(tx, boardShortName, 1)
			require.NoError(t, err)

			expectedTexts := []string{"Test OP", "Reply 2", "Reply 3", "Reply 4"}
			requireMessageOrder(t, board.Threads[0].Messages, expectedTexts)
		})
	})
}
