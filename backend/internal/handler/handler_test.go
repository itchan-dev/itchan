package handler

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestLoadAndValidateRequestBody(t *testing.T) {
	type TestStruct struct {
		Field1 string `json:"field1" validate:"required"`
		Field2 int    `json:"field2"`
	}

	tests := []struct {
		name        string
		requestBody string
		target      interface{}
		expectedErr *internal_errors.ErrorWithStatusCode
	}{
		{
			name:        "Valid JSON and Validation",
			requestBody: `{"field1": "value", "field2": 123}`,
			target:      &TestStruct{},
			expectedErr: nil,
		},
		{
			name:        "Valid JSON and Validation [2]",
			requestBody: `{"field1": "value"}`,
			target:      &TestStruct{},
			expectedErr: nil,
		},
		{
			name:        "Invalid JSON",
			requestBody: `{"field1": "value", "field2": 123`, // Missing closing brace
			target:      &TestStruct{},
			expectedErr: &internal_errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400},
		},
		{
			name:        "Missing Required Field",
			requestBody: `{"field2": 123}`,
			target:      &TestStruct{},
			expectedErr: &internal_errors.ErrorWithStatusCode{Message: "Required fields missing", StatusCode: 400},
		},
		{
			name:        "Empty Body", // Test with empty body
			requestBody: "",
			target:      &TestStruct{},
			expectedErr: &internal_errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400}, // Or expect EOF
		},
	}

	log.SetOutput(io.Discard)      // Discard log output during tests
	defer log.SetOutput(os.Stderr) // Restore log output after tests

	validate = validator.New(validator.WithRequiredStructEnabled()) // Initialize validator once outside the loop

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create the request
			body := []byte(tt.requestBody)
			req := httptest.NewRequest("POST", "/", bytes.NewReader(body))

			err := LoadAndValidateRequestBody(req, tt.target)

			if tt.expectedErr == nil {
				assert.NoError(t, err, "Expected no error")
			} else {
				e, ok := err.(*internal_errors.ErrorWithStatusCode)
				require.True(t, ok, "Error should be ErrorWithStatusCode")
				assert.Equal(t, tt.expectedErr.Message, e.Message, "Error message mismatch")
				assert.Equal(t, tt.expectedErr.StatusCode, e.StatusCode, "Status code mismatch")
			}
		})
	}
}
