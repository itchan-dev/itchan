package utils

import (
	"bytes"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeValidate(t *testing.T) {
	type TestStruct struct {
		Field1 string `json:"field1" validate:"required"`
		Field2 int    `json:"field2"`
	}

	tests := []struct {
		name        string
		requestBody string
		target      interface{}
		expectedErr *errors.ErrorWithStatusCode
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
			expectedErr: &errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400},
		},
		{
			name:        "Missing Required Field",
			requestBody: `{"field2": 123}`,
			target:      &TestStruct{},
			expectedErr: &errors.ErrorWithStatusCode{Message: "Required fields missing", StatusCode: 400},
		},
		{
			name:        "Empty Body", // Test with empty body
			requestBody: "",
			target:      &TestStruct{},
			expectedErr: &errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400}, // Or expect EOF
		},
	}

	log.SetOutput(io.Discard)      // Discard log output during tests
	defer log.SetOutput(os.Stderr) // Restore log output after tests

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create the request
			body := []byte(tt.requestBody)
			req := httptest.NewRequest("POST", "/", bytes.NewReader(body))

			err := DecodeValidate(req.Body, tt.target)

			if tt.expectedErr == nil {
				assert.NoError(t, err, "Expected no error")
			} else {
				e, ok := err.(*errors.ErrorWithStatusCode)
				require.True(t, ok, "Error should be ErrorWithStatusCode")
				assert.Equal(t, tt.expectedErr.Message, e.Message, "Error message mismatch")
				assert.Equal(t, tt.expectedErr.StatusCode, e.StatusCode, "Status code mismatch")
			}
		})
	}
}
