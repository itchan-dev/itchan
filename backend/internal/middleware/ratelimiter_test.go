package middleware

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/middleware/ratelimiter"
	"github.com/itchan-dev/itchan/shared/domain"
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
		assert.Equal(t, "Rate limit exceeded\n", w2.Body.String())
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

func TestGetEmailFromContext(t *testing.T) {
	t.Run("returns email when user exists in context", func(t *testing.T) {
		user := &domain.User{Email: "test@example.com"}
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), userClaimsKey, user)
		req = req.WithContext(ctx)

		email, err := GetEmailFromContext(req)
		assert.NoError(t, err)
		assert.Equal(t, user.Email, email)
	})

	t.Run("returns error when user not in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		email, err := GetEmailFromContext(req)
		assert.Error(t, err)
		assert.Empty(t, email)
		assert.Equal(t, "Can't get user email", err.Error())
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

	t.Run("returns hash when IP cannot be determined", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ""

		ip, err := GetIP(req)
		assert.NoError(t, err)

		// Validate it's a SHA256 hash (64 hex chars)
		assert.Len(t, ip, 64)
		_, err = hex.DecodeString(ip)
		assert.NoError(t, err, "Returned value should be a valid hex string")
	})
}
func TestGetEmailFromBody(t *testing.T) {
	t.Run("returns email from valid request body", func(t *testing.T) {
		body := bytes.NewBufferString(`{"email": "user@test.com", "password": "pass"}`)
		req := httptest.NewRequest("POST", "/", body)
		req.Header.Set("Content-Type", "application/json")

		email, err := GetEmailFromBody(req)
		assert.NoError(t, err)
		assert.Equal(t, "user@test.com", email)
	})

	t.Run("returns error when email is missing", func(t *testing.T) {
		body := bytes.NewBufferString(`{"password": "pass"}`)
		req := httptest.NewRequest("POST", "/", body)
		req.Header.Set("Content-Type", "application/json")

		email, err := GetEmailFromBody(req)
		assert.Error(t, err)
		assert.Empty(t, email)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid`)
		req := httptest.NewRequest("POST", "/", body)
		req.Header.Set("Content-Type", "application/json")

		email, err := GetEmailFromBody(req)
		assert.Error(t, err)
		assert.Empty(t, email)
	})
}
