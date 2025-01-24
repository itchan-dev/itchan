package ratelimiter

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Allow(t *testing.T) {
	t.Run("allows requests within the rate limit", func(t *testing.T) {
		rl := &RateLimiter{
			tokens:     10,
			capacity:   10,
			rate:       1,
			lastRefill: time.Now(),
			mu:         sync.Mutex{},
		}

		assert.True(t, rl.Allow())
		assert.Equal(t, 9.0, rl.tokens)
	})

	t.Run("denies requests when tokens are depleted", func(t *testing.T) {
		rl := &RateLimiter{
			tokens:     0,
			capacity:   10,
			rate:       1,
			lastRefill: time.Now(),
			mu:         sync.Mutex{},
		}

		assert.False(t, rl.Allow())
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		rl := &RateLimiter{
			tokens:     0,
			capacity:   10,
			rate:       1,
			lastRefill: time.Now().Add(-2 * time.Second),
			mu:         sync.Mutex{},
		}

		assert.True(t, rl.Allow())
		assert.InDelta(t, 0.0, rl.tokens, 1.1) // Account for potential slight time discrepancies
	})

	t.Run("does not exceed capacity", func(t *testing.T) {
		rl := &RateLimiter{
			tokens:     9,
			capacity:   10,
			rate:       1,
			lastRefill: time.Now().Add(-2 * time.Second),
			mu:         sync.Mutex{},
		}

		rl.Allow()
		assert.Equal(t, float64(9), rl.tokens)
	})
	t.Run("concurrent requests", func(t *testing.T) {
		rl := &RateLimiter{
			tokens:     10,
			capacity:   10,
			rate:       10, // 10 tokens per second
			lastRefill: time.Now(),
			mu:         sync.Mutex{},
		}

		wg := sync.WaitGroup{}
		numRequests := 20
		allowed := 0
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if rl.Allow() {
					rl.mu.Lock()
					allowed++
					rl.mu.Unlock()
				}
			}()
		}
		wg.Wait()
		assert.GreaterOrEqual(t, allowed, 9)
		assert.LessOrEqual(t, allowed, 11)
	})
}

func TestUserRateLimiter_getLimiter(t *testing.T) {
	t.Run("creates a new limiter for a new user", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, time.Minute)
		limiter := url.getLimiter("user1")

		require.NotNil(t, limiter)
		assert.Equal(t, 10.0, limiter.tokens)
		assert.Equal(t, "user1", limiter.userID)
	})

	t.Run("returns the existing limiter for the same user", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, time.Minute)
		limiter1 := url.getLimiter("user1")
		limiter2 := url.getLimiter("user1")

		assert.Same(t, limiter1, limiter2)
	})

	t.Run("creates different limiters for different users", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, time.Minute)
		limiter1 := url.getLimiter("user1")
		limiter2 := url.getLimiter("user2")

		assert.NotSame(t, limiter1, limiter2)
	})

	t.Run("concurrent access for limiter creation", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, time.Minute)
		userID := "user1"
		wg := sync.WaitGroup{}
		numRoutines := 10

		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				url.getLimiter(userID)
			}()
		}
		wg.Wait()
		url.mu.RLock()
		limiter, ok := url.limiters[userID]
		url.mu.RUnlock()
		require.True(t, ok)
		require.NotNil(t, limiter)
		assert.Equal(t, 1, len(url.limiters)) // Ensure only one limiter is created
	})
}

func TestUserRateLimiter_Allow(t *testing.T) {
	t.Run("allows and denies requests based on user-specific limiters", func(t *testing.T) {
		url := NewUserRateLimiter(1, 2, time.Minute) // 1 request per second, capacity 2

		assert.True(t, url.Allow("user1"))
		assert.True(t, url.Allow("user1"))
		assert.False(t, url.Allow("user1")) // Depleted tokens

		assert.True(t, url.Allow("user2")) // User2 has their own limiter

		time.Sleep(2 * time.Second) // Wait for refill

		assert.True(t, url.Allow("user1")) // User1 tokens should be refilled
	})
}

func TestUserRateLimiter_cleanup(t *testing.T) {
	t.Run("removes limiter after expiration time", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, 1*time.Millisecond) // Short expiration time
		_ = url.getLimiter("user1")

		require.Eventually(t, func() bool {
			url.mu.RLock()
			defer url.mu.RUnlock()
			_, exists := url.limiters["user1"]
			return !exists
		}, 100*time.Millisecond, 10*time.Millisecond, "limiter should be removed after expiration")
	})

	t.Run("does not remove limiter before expiration time", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, time.Minute) // Long expiration time
		_ = url.getLimiter("user1")

		time.Sleep(100 * time.Millisecond) // Wait for a short time

		url.mu.RLock()
		_, exists := url.limiters["user1"]
		url.mu.RUnlock()
		assert.True(t, exists, "limiter should not be removed before expiration")
	})

	t.Run("resets timer on access", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, 50*time.Millisecond)

		// Wait for some time to pass, but less than the expiration time
		time.Sleep(30 * time.Millisecond)

		// Access the limiter, which should reset the timer
		url.Allow("user1")

		// Wait for longer than the original expiration time, but within a reasonable bound
		// The limiter should not have expired yet because the timer was reset
		time.Sleep(30 * time.Millisecond) // Total wait time is now 60ms, > 50ms

		url.mu.RLock()
		_, exists := url.limiters["user1"]
		url.mu.RUnlock()
		assert.True(t, exists, "limiter should not be removed because the timer was reset")

		// Now wait for the new expiration time to pass
		require.Eventually(t, func() bool {
			url.mu.RLock()
			defer url.mu.RUnlock()
			_, exists := url.limiters["user1"]
			return !exists
		}, 100*time.Millisecond, 10*time.Millisecond, "limiter should be removed after the new expiration")

	})

	t.Run("cleanup specific user", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, time.Minute)
		_ = url.getLimiter("user1")
		_ = url.getLimiter("user2")

		url.cleanup("user1")

		url.mu.RLock()
		_, exists1 := url.limiters["user1"]
		_, exists2 := url.limiters["user2"]
		url.mu.RUnlock()

		assert.False(t, exists1, "user1 limiter should be removed")
		assert.True(t, exists2, "user2 limiter should not be removed")
	})
}

func TestUserRateLimiter_Stop(t *testing.T) {
	t.Run("stops all timers", func(t *testing.T) {
		url := NewUserRateLimiter(1, 10, time.Minute)
		url.getLimiter("user1")
		url.getLimiter("user2")

		url.Stop()

		assert.False(t, url.limiters["user1"].timer.Stop(), "timer for user1 should be stopped")
		assert.False(t, url.limiters["user2"].timer.Stop(), "timer for user2 should be stopped")
	})
}
