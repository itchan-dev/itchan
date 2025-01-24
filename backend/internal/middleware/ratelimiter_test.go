package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/middleware/ratelimiter"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit(t *testing.T) {
	t.Run("allows request within rate limit", func(t *testing.T) {
		rl := ratelimiter.NewUserRateLimiter(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "user1", nil })
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("error getting identity", func(t *testing.T) {
		rl := ratelimiter.NewUserRateLimiter(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "", errors.New("Test error") })
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("blocks request exceeding rate limit", func(t *testing.T) {
		rl := ratelimiter.NewUserRateLimiter(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "user1", nil })
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req1 := httptest.NewRequest("GET", "/", nil)
		w1 := httptest.NewRecorder()
		handler(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		req2 := httptest.NewRequest("GET", "/", nil)
		w2 := httptest.NewRecorder()
		handler(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.Equal(t, "Rate limit exceeded\n", w2.Body.String())
	})

	t.Run("allows request after rate limit reset", func(t *testing.T) {
		rl := ratelimiter.NewUserRateLimiter(1, 1, time.Millisecond*100)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "user1", nil })
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req1 := httptest.NewRequest("GET", "/", nil)
		w1 := httptest.NewRecorder()
		handler(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		time.Sleep(200 * time.Millisecond)

		req2 := httptest.NewRequest("GET", "/", nil)
		w2 := httptest.NewRecorder()
		handler(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("uses identity function to determine user", func(t *testing.T) {
		rl := ratelimiter.NewUserRateLimiter(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return r.Header.Get("X-User-ID"), nil })
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req1 := httptest.NewRequest("GET", "/", nil)
		req1.Header.Set("X-User-ID", "user1")
		w1 := httptest.NewRecorder()
		handler(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		req2 := httptest.NewRequest("GET", "/", nil)
		req2.Header.Set("X-User-ID", "user2")
		w2 := httptest.NewRecorder()
		handler(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		req3 := httptest.NewRequest("GET", "/", nil)
		req3.Header.Set("X-User-ID", "user1")
		w3 := httptest.NewRecorder()
		handler(w3, req3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	})
}
