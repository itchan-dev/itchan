package handler

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createRequest(t *testing.T, method, url string, body []byte, cookies ...*http.Cookie) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, url, bytes.NewBuffer(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name             string
		input            interface{}
		expected         string
		status           int
		checkContentType bool
	}{
		{
			name:     "Valid JSON",
			input:    map[string]string{"message": "hello"},
			expected: `{"message":"hello"}`,
			status:   http.StatusOK,
		},
		{
			name:             "Invalid JSON (channel)", // Test for encoding errors
			input:            make(chan int),
			expected:         "Internal error",
			status:           http.StatusInternalServerError,
			checkContentType: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize the recorder
			rr := httptest.NewRecorder()

			// Capture log output using discard
			log.SetOutput(io.Discard)      // Discard logs to prevent clutter during testing
			defer log.SetOutput(os.Stderr) // Restore log output

			writeJSON(rr, tt.input)

			// Check status code
			assert.Equal(t, tt.status, rr.Code, "handler returned wrong status code")

			// Check content type header
			if tt.checkContentType {
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "handler returned wrong content type")
			}

			// Check response body
			assert.Equal(t, tt.expected+"\n", rr.Body.String(), "handler returned unexpected body")

		})
	}
}
