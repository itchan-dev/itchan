package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/middleware/ratelimiter"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit(t *testing.T) {
	t.Run("allows request within rate limit", func(t *testing.T) {
		rl := ratelimiter.New(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "user1", nil })
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("error getting identity", func(t *testing.T) {
		rl := ratelimiter.New(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "", errors.New("Test error") })
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("blocks request exceeding rate limit", func(t *testing.T) {
		rl := ratelimiter.New(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "user1", nil })
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req1 := httptest.NewRequest("GET", "/", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		req2 := httptest.NewRequest("GET", "/", nil)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.Equal(t, "Rate limit exceeded, try again later\n", w2.Body.String())
	})

	t.Run("allow admin request exceeding rate limit", func(t *testing.T) {
		rl := ratelimiter.New(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "user1", nil })
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req1 := httptest.NewRequest("GET", "/", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// forbid for non admin
		user := &domain.User{Admin: false}
		req2 := httptest.NewRequest("GET", "/", nil)
		ctx2 := context.WithValue(req2.Context(), UserClaimsKey, user)
		req2 = req2.WithContext(ctx2)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)

		// accept for admin
		user = &domain.User{Admin: true}
		req3 := httptest.NewRequest("GET", "/", nil)
		ctx3 := context.WithValue(req3.Context(), UserClaimsKey, user)
		req3 = req2.WithContext(ctx3)
		w3 := httptest.NewRecorder()
		handler.ServeHTTP(w3, req3)

		assert.Equal(t, http.StatusOK, w3.Code)
	})

	t.Run("allows request after rate limit reset", func(t *testing.T) {
		rl := ratelimiter.New(1, 1, time.Millisecond*100)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return "user1", nil })
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req1 := httptest.NewRequest("GET", "/", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		time.Sleep(200 * time.Millisecond)

		req2 := httptest.NewRequest("GET", "/", nil)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("uses identity function to determine user", func(t *testing.T) {
		rl := ratelimiter.New(1, 1, time.Minute)
		defer rl.Stop()
		middleware := RateLimit(rl, func(r *http.Request) (string, error) { return r.Header.Get("X-User-ID"), nil })
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req1 := httptest.NewRequest("GET", "/", nil)
		req1.Header.Set("X-User-ID", "user1")
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		req2 := httptest.NewRequest("GET", "/", nil)
		req2.Header.Set("X-User-ID", "user2")
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		req3 := httptest.NewRequest("GET", "/", nil)
		req3.Header.Set("X-User-ID", "user1")
		w3 := httptest.NewRecorder()
		handler.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	})
}

func TestRateLimitWithHandler(t *testing.T) {
	t.Run("calls custom handler on rate limit exceeded", func(t *testing.T) {
		rl := ratelimiter.New(1, 1, time.Minute)
		defer rl.Stop()

		customHandlerCalled := false
		middleware := RateLimitWithHandler(rl,
			func(r *http.Request) (string, error) { return "user1", nil },
			func(w http.ResponseWriter, r *http.Request) {
				customHandlerCalled = true
				w.WriteHeader(http.StatusSeeOther)
				w.Header().Set("Location", "/")
			},
		)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// First request: allowed
		req1 := httptest.NewRequest("POST", "/board", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)
		assert.False(t, customHandlerCalled)

		// Second request: rate limited, custom handler called
		req2 := httptest.NewRequest("POST", "/board", nil)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		assert.True(t, customHandlerCalled)
		assert.Equal(t, http.StatusSeeOther, w2.Code)
	})
}

func TestGetUserIDFromContext(t *testing.T) {
	t.Run("returns user id when user exists in context", func(t *testing.T) {
		user := &domain.User{Id: 123, EmailDomain: "example.com"}
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), UserClaimsKey, user)
		req = req.WithContext(ctx)

		userID, err := GetUserIDFromContext(req)
		assert.NoError(t, err)
		assert.Equal(t, "user_123", userID)
	})

	t.Run("returns error when user not in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		userID, err := GetUserIDFromContext(req)
		assert.Error(t, err)
		assert.Empty(t, userID)
		assert.Equal(t, "Can't get user id", err.Error())
	})
}

func TestGetIP(t *testing.T) {
	t.Run("returns IP from request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		ip, err := GetIP(req)
		assert.NoError(t, err)
		assert.Equal(t, "192.168.1.1", ip)
	})

	t.Run("returns error when RemoteAddr is invalid", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ""

		_, err := GetIP(req)
		assert.Error(t, err, "Should return error for empty RemoteAddr")
		assert.Contains(t, err.Error(), "invalid IP address")
	})
}

// func TestGetEmailFromBody(t *testing.T) {
// 	t.Run("returns email from valid request body", func(t *testing.T) {
// 		body := bytes.NewBufferString(`{"email": "user@test.com", "password": "pass"}`)
// 		req := httptest.NewRequest("POST", "/", body)
// 		req.Header.Set("Content-Type", "application/json")

// 		email, err := GetEmailFromBody(req)
// 		assert.NoError(t, err)
// 		assert.Equal(t, "user@test.com", email)
// 	})

// 	t.Run("returns error when email is missing", func(t *testing.T) {
// 		body := bytes.NewBufferString(`{"password": "pass"}`)
// 		req := httptest.NewRequest("POST", "/", body)
// 		req.Header.Set("Content-Type", "application/json")

// 		email, err := GetEmailFromBody(req)
// 		assert.Error(t, err)
// 		assert.Empty(t, email)
// 	})

// 	t.Run("returns error on invalid JSON", func(t *testing.T) {
// 		body := bytes.NewBufferString(`{invalid`)
// 		req := httptest.NewRequest("POST", "/", body)
// 		req.Header.Set("Content-Type", "application/json")

// 		email, err := GetEmailFromBody(req)
// 		assert.Error(t, err)
// 		assert.Empty(t, email)
// 	})
// }
