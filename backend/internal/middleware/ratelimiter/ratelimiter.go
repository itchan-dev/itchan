package ratelimiter

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	tokens     float64
	capacity   float64
	rate       float64
	lastRefill time.Time
	mu         sync.Mutex
	timer      *time.Timer
	userID     string           // Reference to userID for cleanup
	parent     *UserRateLimiter // Reference to parent for cleanup
}

// UserRateLimiter manages rate limiting for multiple users
type UserRateLimiter struct {
	limiters       map[string]*RateLimiter
	mu             sync.RWMutex
	rate           float64
	capacity       float64
	expirationTime time.Duration
}

// NewUserRateLimiter creates a new UserRateLimiter instance
func NewUserRateLimiter(rate float64, capacity float64, expirationTime time.Duration) *UserRateLimiter {
	return &UserRateLimiter{
		limiters:       make(map[string]*RateLimiter),
		rate:           rate,
		capacity:       capacity,
		expirationTime: expirationTime,
	}
}

// cleanup removes a specific limiter
func (url *UserRateLimiter) cleanup(userID string) {
	url.mu.Lock()
	delete(url.limiters, userID)
	url.mu.Unlock()
}

// resetTimer resets the expiration timer for a limiter
func (rl *RateLimiter) resetTimer() {
	if rl.timer != nil {
		rl.timer.Stop()
	}

	// Create new timer
	rl.timer = time.AfterFunc(rl.parent.expirationTime, func() {
		rl.parent.cleanup(rl.userID)
	})
}

// getLimiter gets or creates a rate limiter for a user
func (url *UserRateLimiter) getLimiter(userID string) *RateLimiter {
	// First try read-only lookup
	url.mu.RLock()
	limiter, exists := url.limiters[userID]
	url.mu.RUnlock()

	if exists {
		limiter.resetTimer()
		return limiter
	}

	// If not found, acquire write lock and create new
	url.mu.Lock()
	defer url.mu.Unlock()

	// Double-check after acquiring write lock
	limiter, exists = url.limiters[userID]
	if exists {
		limiter.resetTimer()
		return limiter
	}

	// Create new limiter
	limiter = &RateLimiter{
		tokens:     url.capacity,
		capacity:   url.capacity,
		rate:       url.rate,
		lastRefill: time.Now(),
		userID:     userID,
		parent:     url,
	}
	url.limiters[userID] = limiter
	limiter.resetTimer()

	return limiter
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	// Refill tokens based on elapsed time
	rl.tokens += elapsed * rl.rate
	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity
	}

	rl.lastRefill = now

	// Check if we have enough tokens
	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}

	return false
}

// Allow checks if a request should be allowed for a given user
func (url *UserRateLimiter) Allow(userID string) bool {
	limiter := url.getLimiter(userID)

	return limiter.Allow()
}

// Stop cleans up all timers
func (url *UserRateLimiter) Stop() {
	url.mu.Lock()
	defer url.mu.Unlock()

	for _, limiter := range url.limiters {
		if limiter.timer != nil {
			limiter.timer.Stop()
		}
	}
}

var OnceInMinute = NewUserRateLimiter(1/60, 1, 1*time.Hour)
var OnceInSecond = NewUserRateLimiter(1, 1, 1*time.Hour)
