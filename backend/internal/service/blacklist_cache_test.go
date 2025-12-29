package service

import (
	"context"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock storage for testing
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

func (m *mockBlacklistStorage) BlacklistUser(userId domain.UserId, reason string, blacklistedBy domain.UserId) error {
	return m.err
}

func (m *mockBlacklistStorage) UnblacklistUser(userId domain.UserId) error {
	return m.err
}

func (m *mockBlacklistStorage) IsUserBlacklisted(userId domain.UserId) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	for _, id := range m.blacklistedUsers {
		if id == userId {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockBlacklistStorage) GetBlacklistedUsersWithDetails() ([]domain.BlacklistEntry, error) {
	return nil, m.err
}

func TestNewBlacklistCache(t *testing.T) {
	storage := &mockBlacklistStorage{}
	cache := NewBlacklistCache(storage, time.Hour)

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.cache)
	assert.Equal(t, time.Hour, cache.jwtTTL)
}

func TestBlacklistCache_Update(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		storage := &mockBlacklistStorage{
			blacklistedUsers: []domain.UserId{1, 2, 3},
		}
		cache := NewBlacklistCache(storage, time.Hour)

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
		cache := NewBlacklistCache(storage, time.Hour)

		err := cache.Update()
		assert.Error(t, err)
	})

	t.Run("update replaces cache", func(t *testing.T) {
		storage := &mockBlacklistStorage{
			blacklistedUsers: []domain.UserId{1, 2},
		}
		cache := NewBlacklistCache(storage, time.Hour)

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

func TestBlacklistCache_IsBlacklisted(t *testing.T) {
	storage := &mockBlacklistStorage{
		blacklistedUsers: []domain.UserId{1, 2, 3},
	}
	cache := NewBlacklistCache(storage, time.Hour)
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

func TestBlacklistCache_ConcurrentAccess(t *testing.T) {
	storage := &mockBlacklistStorage{
		blacklistedUsers: []domain.UserId{1, 2, 3},
	}
	cache := NewBlacklistCache(storage, time.Hour)
	cache.Update()

	// Simulate concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cache.IsBlacklisted(1)
			}
			done <- true
		}()
	}

	go func() {
		for i := 0; i < 10; i++ {
			storage.blacklistedUsers = []domain.UserId{4, 5, 6}
			cache.Update()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// No assertion needed - test passes if no race condition detected
}

func TestBlacklistCache_BackgroundUpdate(t *testing.T) {
	storage := &mockBlacklistStorage{
		blacklistedUsers: []domain.UserId{1},
	}
	cache := NewBlacklistCache(storage, time.Hour)

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
