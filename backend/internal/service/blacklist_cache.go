package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

// BlacklistStorage defines the database operations needed for blacklist management.
type BlacklistStorage interface {
	GetRecentlyBlacklistedUsers(since time.Time) ([]domain.UserId, error)
	BlacklistUser(userId domain.UserId, reason string, blacklistedBy domain.UserId) error
	UnblacklistUser(userId domain.UserId) error
	IsUserBlacklisted(userId domain.UserId) (bool, error)
	GetBlacklistedUsersWithDetails() ([]domain.BlacklistEntry, error)
}

// BlacklistCache maintains an in-memory cache of recently blacklisted users
// to avoid database queries on every authentication request.
type BlacklistCache struct {
	storage        BlacklistStorage
	cache          map[domain.UserId]bool
	mu             sync.RWMutex
	jwtTTL         time.Duration
	lastUpdateTime time.Time
}

// NewBlacklistCache creates a new blacklist cache instance.
// jwtTTL is used to determine how far back to query for blacklisted users.
func NewBlacklistCache(storage BlacklistStorage, jwtTTL time.Duration) *BlacklistCache {
	return &BlacklistCache{
		storage: storage,
		cache:   make(map[domain.UserId]bool),
		jwtTTL:  jwtTTL,
	}
}

// Update fetches recently blacklisted users from the database and updates the cache.
// It queries for users blacklisted within (JWT TTL + 10% buffer) to handle clock skew.
func (bc *BlacklistCache) Update() error {
	// Calculate cutoff time with 10% buffer
	bufferMultiplier := 1.1
	since := time.Now().Add(-time.Duration(float64(bc.jwtTTL) * bufferMultiplier))

	// Fetch recently blacklisted users from database
	userIds, err := bc.storage.GetRecentlyBlacklistedUsers(since)
	if err != nil {
		return err
	}

	// Build new cache map
	newCache := make(map[domain.UserId]bool, len(userIds))
	for _, userId := range userIds {
		newCache[userId] = true
	}

	// Atomically replace the cache
	bc.mu.Lock()
	bc.cache = newCache
	bc.lastUpdateTime = time.Now()
	bc.mu.Unlock()

	log.Printf("BlacklistCache: updated cache with %d entries (since: %v)", len(newCache), since.Format(time.RFC3339))
	return nil
}

// IsBlacklisted checks if a user ID is in the blacklist cache.
// This is a thread-safe, high-performance read operation.
func (bc *BlacklistCache) IsBlacklisted(userId domain.UserId) bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.cache[userId]
}

// StartBackgroundUpdate starts a background goroutine that periodically refreshes
// the blacklist cache. It follows the same pattern as MediaGarbageCollector.
func (bc *BlacklistCache) StartBackgroundUpdate(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	log.Printf("Started BlacklistCache background updates (interval: %v, JWT TTL: %v)", interval, bc.jwtTTL)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := bc.Update(); err != nil {
					log.Printf("BlacklistCache: update error: %v", err)
				}
			case <-ctx.Done():
				log.Printf("BlacklistCache: shutting down gracefully")
				return
			}
		}
	}()
}
