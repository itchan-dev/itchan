package blacklist

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

// BlacklistCacheStorage defines the minimal database operations needed for cache updates.
// This is intentionally minimal - only read operations needed for cache population.
// Admin operations (blacklist/unblacklist) belong in backend-specific storage.
type BlacklistCacheStorage interface {
	GetRecentlyBlacklistedUsers(since time.Time) ([]domain.UserId, error)
}

// Cache maintains an in-memory cache of recently blacklisted users
// to avoid database queries on every authentication request.
type Cache struct {
	storage        BlacklistCacheStorage
	cache          map[domain.UserId]bool
	mu             sync.RWMutex
	jwtTTL         time.Duration
	lastUpdateTime time.Time
}

// NewCache creates a new blacklist cache instance.
// jwtTTL is used to determine how far back to query for blacklisted users.
func NewCache(storage BlacklistCacheStorage, jwtTTL time.Duration) *Cache {
	return &Cache{
		storage: storage,
		cache:   make(map[domain.UserId]bool),
		jwtTTL:  jwtTTL,
	}
}

// Update fetches recently blacklisted users from the database and updates the cache.
// It queries for users blacklisted within (JWT TTL + 10% buffer) to handle clock skew.
func (bc *Cache) Update() error {
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
func (bc *Cache) IsBlacklisted(userId domain.UserId) bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.cache[userId]
}

// StartBackgroundUpdate starts a background goroutine that periodically refreshes
// the blacklist cache. It follows the same pattern as MediaGarbageCollector.
func (bc *Cache) StartBackgroundUpdate(ctx context.Context, interval time.Duration) {
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
