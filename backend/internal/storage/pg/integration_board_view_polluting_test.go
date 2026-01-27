//go:build polluting

package pg

import (
	"sync"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBoardViewConcurrentRefresh verifies concurrent refresh functionality.
// These tests CANNOT use transactions because REFRESH MATERIALIZED VIEW CONCURRENTLY
// requires committed data.
//
// IMPORTANT: This test is separated into its own file with the 'polluting' build tag
// because it modifies the shared database and cannot be safely run with other tests.
//
// Run this test with: go test -tags=polluting ./backend/internal/storage/pg
func TestBoardViewConcurrentRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping non-transactional test in short mode")
	}
	t.Run("RefreshMaterializedViewConcurrent", func(t *testing.T) {
		t.Run("success with existing board", func(t *testing.T) {
			boardShortName := domain.BoardShortName(generateString(t))
			err := storage.CreateBoard(domain.BoardCreationData{
				Name:      "Test Board",
				ShortName: boardShortName,
			})
			require.NoError(t, err)
			defer storage.DeleteBoard(boardShortName)

			userID, err := storage.SaveUser(generateString(t)+"@example.com", "test", false)
			require.NoError(t, err)

			_, err = storage.CreateThread(domain.ThreadCreationData{
				Title: "Test Thread",
				Board: boardShortName,
				OpMessage: domain.MessageCreationData{
					Board:  boardShortName,
					Author: domain.User{Id: userID},
					Text:   "Test OP",
				},
			})
			require.NoError(t, err)

			err = storage.refreshMaterializedViewConcurrent(boardShortName, 2*time.Second)
			require.NoError(t, err)

			board, err := storage.GetBoard(boardShortName, 1)
			require.NoError(t, err)
			assert.Equal(t, boardShortName, board.ShortName)
			require.Len(t, board.Threads, 1)
		})

		t.Run("fails for nonexistent board", func(t *testing.T) {
			nonExistentBoard := domain.BoardShortName(generateString(t))
			err := storage.refreshMaterializedViewConcurrent(nonExistentBoard, 500*time.Millisecond)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "concurrent refresh failed")
		})
	})

	t.Run("ConcurrentRefreshSafety", func(t *testing.T) {
		t.Run("handles multiple simultaneous refreshes", func(t *testing.T) {
			boardShortName := domain.BoardShortName(generateString(t))
			err := storage.CreateBoard(domain.BoardCreationData{
				Name:      "Concurrent Test Board",
				ShortName: boardShortName,
			})
			require.NoError(t, err)
			defer storage.DeleteBoard(boardShortName)

			userID, err := storage.SaveUser(generateString(t)+"@example.com", "test", false)
			require.NoError(t, err)

			threadID, err := storage.CreateThread(domain.ThreadCreationData{
				Title: "Test Thread",
				Board: boardShortName,
				OpMessage: domain.MessageCreationData{
					Board:  boardShortName,
					Author: domain.User{Id: userID},
					Text:   "Test OP",
				},
			})
			require.NoError(t, err)

			_, err = storage.CreateMessage(domain.MessageCreationData{
				Board:    boardShortName,
				Author:   domain.User{Id: userID},
				Text:     "Message 1",
				ThreadId: threadID,
			})
			require.NoError(t, err)

			var wg sync.WaitGroup
			numConcurrent := 3
			errors := make(chan error, numConcurrent)

			for i := 0; i < numConcurrent; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := storage.refreshMaterializedViewConcurrent(boardShortName, 2*time.Second)
					if err != nil {
						errors <- err
					}
				}()
			}

			wg.Wait()
			close(errors)

			for err := range errors {
				require.NoError(t, err, "Concurrent refresh should not error")
			}

			board, err := storage.GetBoard(boardShortName, 1)
			require.NoError(t, err)
			assert.NotEmpty(t, board.Threads)
			assert.GreaterOrEqual(t, len(board.Threads[0].Messages), 2)
		})
	})
}
