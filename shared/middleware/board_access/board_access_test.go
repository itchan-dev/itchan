package board_access

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	mu     sync.RWMutex
	boards []domain.Board
	err    error
}

func (m *mockStorage) GetBoards() ([]domain.Board, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.boards, m.err
}

func (m *mockStorage) setBoards(boards []domain.Board) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.boards = boards
}

func TestNew(t *testing.T) {
	ba := New()
	assert.NotNil(t, ba.data, "Data map should be initialized")
	assert.Empty(t, ba.data, "Data map should be empty")

}

func TestUpdate(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		ms := &mockStorage{
			boards: []domain.Board{
				{
					BoardMetadata: domain.BoardMetadata{
						ShortName:     "test",
						AllowedEmails: &domain.Emails{"example.com"},
					},
				},
			},
		}

		ba := New()
		err := ba.Update(ms)
		require.NoError(t, err)

		domains := ba.AllowedDomains("test")
		assert.Equal(t, []string{"example.com"}, domains)
	})

	t.Run("should handle storage errors", func(t *testing.T) {
		ms := &mockStorage{err: errors.New("storage error")}
		ba := New()

		err := ba.Update(ms)
		assert.Error(t, err)
	})

	t.Run("should refresh data completely", func(t *testing.T) {
		ms := &mockStorage{}
		ms.setBoards([]domain.Board{
			{BoardMetadata: domain.BoardMetadata{ShortName: "old", AllowedEmails: &domain.Emails{"old.com"}}},
		})

		ba := New()
		require.NoError(t, ba.Update(ms))

		// Update mock data
		ms.setBoards([]domain.Board{
			{BoardMetadata: domain.BoardMetadata{ShortName: "new", AllowedEmails: &domain.Emails{"new.com"}}},
		})

		require.NoError(t, ba.Update(ms))

		assert.Nil(t, ba.AllowedDomains("old"))
		assert.Equal(t, []string{"new.com"}, ba.AllowedDomains("new"))
	})
}

func TestAllowedDomains(t *testing.T) {
	ba := New()
	ba.data["existing"] = []string{"example.com"}

	t.Run("existing board", func(t *testing.T) {
		domains := ba.AllowedDomains("existing")
		assert.Equal(t, []string{"example.com"}, domains)
	})

	t.Run("non-existing board", func(t *testing.T) {
		domains := ba.AllowedDomains("nonexistent")
		assert.Nil(t, domains)
	})
}

func TestConcurrentAccess(t *testing.T) {
	ba := New()
	ms := &mockStorage{
		boards: []domain.Board{
			{BoardMetadata: domain.BoardMetadata{ShortName: "test", AllowedEmails: &domain.Emails{"example.com"}}},
		},
	}

	var wg sync.WaitGroup
	wg.Add(3)

	// Writer
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = ba.Update(ms)
		}
	}()

	// Readers
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = ba.AllowedDomains("test")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = ba.AllowedDomains("nonexistent")
		}
	}()

	wg.Wait()
}

func TestBackgroundUpdates(t *testing.T) {
	ms := &mockStorage{}
	ba := New()
	interval := 10 * time.Millisecond

	// Initial data setup
	ms.setBoards([]domain.Board{
		{BoardMetadata: domain.BoardMetadata{ShortName: "test", AllowedEmails: &domain.Emails{"initial.com"}}},
	})

	ba.StartBackgroundUpdate(interval, ms)

	// Not equal before update
	assert.NotEqual(t, []string{"initial.com"}, ba.AllowedDomains("test"))

	// Wait for first update to complete
	time.Sleep(interval * 2)

	// Verify initial data
	assert.Equal(t, []string{"initial.com"}, ba.AllowedDomains("test"))

	// Update mock data
	ms.setBoards([]domain.Board{
		{BoardMetadata: domain.BoardMetadata{ShortName: "test", AllowedEmails: &domain.Emails{"updated.com"}}},
	})

	// Wait for refresh cycle
	time.Sleep(interval * 2)

	// Verify updated data
	assert.Equal(t, []string{"updated.com"}, ba.AllowedDomains("test"))
}
