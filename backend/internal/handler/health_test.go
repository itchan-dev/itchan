package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/stretchr/testify/assert"
)

// --- Mock for HealthChecker ---

type MockHealthChecker struct {
	PingFunc func(ctx context.Context) error
}

func (m *MockHealthChecker) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil // Default: healthy
}

// --- Tests ---

func TestHealth(t *testing.T) {
	t.Run("always returns 200 OK", func(t *testing.T) {
		// Arrange
		handler := &Handler{
			cfg:    &config.Config{},
			health: &MockHealthChecker{},
		}

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		// Act
		handler.Health(rr, req)

		// Assert
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "ok", rr.Body.String())
	})
}

func TestReady(t *testing.T) {
	t.Run("returns 200 OK when database is available", func(t *testing.T) {
		// Arrange
		healthChecker := &MockHealthChecker{
			PingFunc: func(ctx context.Context) error {
				return nil // Database is healthy
			},
		}

		handler := &Handler{
			cfg:    &config.Config{},
			health: healthChecker,
		}

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()

		// Act
		handler.Ready(rr, req)

		// Assert
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "ok", rr.Body.String())
	})

	t.Run("returns 503 Service Unavailable when database is down", func(t *testing.T) {
		// Arrange
		dbError := errors.New("connection refused")
		healthChecker := &MockHealthChecker{
			PingFunc: func(ctx context.Context) error {
				return dbError // Database is unavailable
			},
		}

		handler := &Handler{
			cfg:    &config.Config{},
			health: healthChecker,
		}

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()

		// Act
		handler.Ready(rr, req)

		// Assert
		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		assert.Equal(t, "database unavailable", rr.Body.String())
	})

	t.Run("uses timeout context for ping check", func(t *testing.T) {
		// Arrange
		var receivedContext context.Context
		healthChecker := &MockHealthChecker{
			PingFunc: func(ctx context.Context) error {
				receivedContext = ctx
				// Verify context has timeout
				_, hasDeadline := ctx.Deadline()
				assert.True(t, hasDeadline, "Context should have a deadline")
				return nil
			},
		}

		handler := &Handler{
			cfg:    &config.Config{},
			health: healthChecker,
		}

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()

		// Act
		handler.Ready(rr, req)

		// Assert
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotNil(t, receivedContext, "Ping should have been called with a context")
	})

	t.Run("handles context cancellation gracefully", func(t *testing.T) {
		// Arrange
		ctxError := context.DeadlineExceeded
		healthChecker := &MockHealthChecker{
			PingFunc: func(ctx context.Context) error {
				return ctxError // Simulate timeout
			},
		}

		handler := &Handler{
			cfg:    &config.Config{},
			health: healthChecker,
		}

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()

		// Act
		handler.Ready(rr, req)

		// Assert
		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		assert.Equal(t, "database unavailable", rr.Body.String())
	})
}
