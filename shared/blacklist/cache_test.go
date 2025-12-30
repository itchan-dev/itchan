package blacklist

import (
	"context"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock storage for testing - implements minimal BlacklistCacheStorage interface
type mockBlacklistStorage struct {
	blacklistedUsers []domain.UserId
	err              error
}

func (m *mockBlacklistStorage) GetRecentlyBlacklistedUsers(since time.Time) ([]domain.UserId, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.blacklistedUsers, nil
}

func TestNewCache(t *testing.T) {
	storage := &mockBlacklistStorage{}
	cache := NewCache(storage, time.Hour)

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.cache)
	assert.Equal(t, time.Hour, cache.jwtTTL)
}

func TestCache_Update(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		storage := &mockBlacklistStorage{
			blacklistedUsers: []domain.UserId{1, 2, 3},
		}
		cache := NewCache(storage, time.Hour)

		err := cache.Update()
		require.NoError(t, err)

		// Verify cache was updated
		assert.True(t, cache.IsBlacklisted(1))
		assert.True(t, cache.IsBlacklisted(2))
		assert.True(t, cache.IsBlacklisted(3))
		assert.False(t, cache.IsBlacklisted(4))
	})

	t.Run("update with error", func(t *testing.T) {
		storage := &mockBlacklistStorage{
			err: assert.AnError,
		}
		cache := NewCache(storage, time.Hour)

		err := cache.Update()
		assert.Error(t, err)
	})

	t.Run("update replaces cache", func(t *testing.T) {
		storage := &mockBlacklistStorage{
			blacklistedUsers: []domain.UserId{1, 2},
		}
		cache := NewCache(storage, time.Hour)

		// First update
		err := cache.Update()
		require.NoError(t, err)
		assert.True(t, cache.IsBlacklisted(1))
		assert.True(t, cache.IsBlacklisted(2))

		// Change storage data
		storage.blacklistedUsers = []domain.UserId{3, 4}

		// Second update should replace cache
		err = cache.Update()
		require.NoError(t, err)
		assert.False(t, cache.IsBlacklisted(1))
		assert.False(t, cache.IsBlacklisted(2))
		assert.True(t, cache.IsBlacklisted(3))
		assert.True(t, cache.IsBlacklisted(4))
	})
}

func TestCache_IsBlacklisted(t *testing.T) {
	storage := &mockBlacklistStorage{
		blacklistedUsers: []domain.UserId{1, 2, 3},
	}
	cache := NewCache(storage, time.Hour)
	cache.Update()

	tests := []struct {
		name        string
		userId      domain.UserId
		blacklisted bool
	}{
		{"blacklisted user 1", 1, true},
		{"blacklisted user 2", 2, true},
		{"blacklisted user 3", 3, true},
		{"non-blacklisted user 4", 4, false},
		{"non-blacklisted user 100", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.IsBlacklisted(tt.userId)
			assert.Equal(t, tt.blacklisted, result)
		})
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	storage := &mockBlacklistStorage{
		blacklistedUsers: []domain.UserId{1, 2, 3},
	}
	cache := NewCache(storage, time.Hour)
	cache.Update()

	// Simulate concurrent reads and writes
	done := make(chan bool)
	for range 10 {
		go func() {
			for range 100 {
				cache.IsBlacklisted(1)
			}
			done <- true
		}()
	}

	go func() {
		for range 10 {
			storage.blacklistedUsers = []domain.UserId{4, 5, 6}
			cache.Update()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}

	// No assertion needed - test passes if no race condition detected
}

func TestCache_BackgroundUpdate(t *testing.T) {
	storage := &mockBlacklistStorage{
		blacklistedUsers: []domain.UserId{1},
	}
	cache := NewCache(storage, time.Hour)

	// Start background updates with short interval
	ctx, cancel := context.WithCancel(context.Background())
	cache.StartBackgroundUpdate(ctx, 50*time.Millisecond)

	// Initial update
	cache.Update()
	assert.True(t, cache.IsBlacklisted(1))

	// Change storage data
	storage.blacklistedUsers = []domain.UserId{2}

	// Wait for background update to run
	time.Sleep(100 * time.Millisecond)

	// Cache should have updated
	assert.False(t, cache.IsBlacklisted(1))
	assert.True(t, cache.IsBlacklisted(2))

	// Cancel context to stop background updates
	cancel()
	time.Sleep(50 * time.Millisecond)
}
