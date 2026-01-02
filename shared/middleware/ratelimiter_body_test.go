package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
)

func TestGetEmailFromBody_DoesNotDestroyBody(t *testing.T) {
	// Create test data
	testData := map[string]string{
		"email":    "test@example.com",
		"password": "secretpass123",
		"other":    "data",
	}
	bodyBytes, _ := json.Marshal(testData)

	// Create request
	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	// First read: GetEmailFromBody (middleware)
	email, err := GetEmailFromBody(req)
	if err != nil {
		t.Fatalf("GetEmailFromBody failed: %v", err)
	}
	if email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", email)
	}

	// Second read: Simulate handler reading body
	bodyAfter, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("Failed to read body after GetEmailFromBody: %v", err)
	}

	// Verify body content is still there
	var dataAfter map[string]string
	if err := json.Unmarshal(bodyAfter, &dataAfter); err != nil {
		t.Fatalf("Failed to unmarshal body after GetEmailFromBody: %v", err)
	}

	// Verify all data is intact
	if dataAfter["email"] != "test@example.com" {
		t.Errorf("Email not preserved: expected 'test@example.com', got '%s'", dataAfter["email"])
	}
	if dataAfter["password"] != "secretpass123" {
		t.Errorf("Password not preserved: expected 'secretpass123', got '%s'", dataAfter["password"])
	}
	if dataAfter["other"] != "data" {
		t.Errorf("Other data not preserved: expected 'data', got '%s'", dataAfter["other"])
	}

	t.Log("✅ Body successfully preserved after GetEmailFromBody")
}

func TestGetEmailFromBody_MultipleReads(t *testing.T) {
	testData := map[string]string{"email": "user@test.com"}
	bodyBytes, _ := json.Marshal(testData)
	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(bodyBytes))

	// Read 1: GetEmailFromBody
	email1, err := GetEmailFromBody(req)
	if err != nil {
		t.Fatalf("First GetEmailFromBody failed: %v", err)
	}

	// Read 2: GetEmailFromBody again (simulating another middleware)
	email2, err := GetEmailFromBody(req)
	if err != nil {
		t.Fatalf("Second GetEmailFromBody failed: %v", err)
	}

	if email1 != email2 {
		t.Errorf("Emails don't match: %s vs %s", email1, email2)
	}

	// Read 3: Handler
	var data map[string]string
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		t.Fatalf("Handler decode failed: %v", err)
	}

	if data["email"] != "user@test.com" {
		t.Errorf("Handler got wrong email: %s", data["email"])
	}

	t.Log("✅ Multiple reads work correctly")
}

func TestGetEmailFromBody_EmptyEmail(t *testing.T) {
	testData := map[string]string{"email": "", "password": "test"}
	bodyBytes, _ := json.Marshal(testData)
	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(bodyBytes))

	_, err := GetEmailFromBody(req)
	if err == nil {
		t.Error("Expected error for empty email, got nil")
	}
	if err.Error() != "email field is required" {
		t.Errorf("Expected 'email field is required', got '%s'", err.Error())
	}
}

func TestGetEmailFromBody_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString("not valid json"))

	_, err := GetEmailFromBody(req)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
