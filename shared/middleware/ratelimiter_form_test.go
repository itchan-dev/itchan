package middleware

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGetEmailFromForm_ValidFormData(t *testing.T) {
	// Create form data
	formData := url.Values{}
	formData.Set("email", "user@example.com")
	formData.Set("password", "secret123")

	req := httptest.NewRequest("POST", "/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	email, err := GetEmailFromForm(req)
	if err != nil {
		t.Fatalf("GetEmailFromForm failed: %v", err)
	}
	if email != "user@example.com" {
		t.Errorf("Expected email 'user@example.com', got '%s'", email)
	}
}

func TestGetEmailFromForm_EmptyEmail(t *testing.T) {
	formData := url.Values{}
	formData.Set("password", "secret123")
	// No email field

	req := httptest.NewRequest("POST", "/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err := GetEmailFromForm(req)
	if err == nil {
		t.Error("Expected error for missing email, got nil")
	}
	if err.Error() != "email field is required" {
		t.Errorf("Expected 'email field is required', got '%s'", err.Error())
	}
}

func TestGetEmailFromForm_MultipleReads(t *testing.T) {
	// Test that we can read the email multiple times (for multiple middleware)
	formData := url.Values{}
	formData.Set("email", "test@test.com")

	req := httptest.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Read 1
	email1, err := GetEmailFromForm(req)
	if err != nil {
		t.Fatalf("First GetEmailFromForm failed: %v", err)
	}

	// Read 2 (should still work because ParseForm caches the result)
	email2, err := GetEmailFromForm(req)
	if err != nil {
		t.Fatalf("Second GetEmailFromForm failed: %v", err)
	}

	if email1 != email2 {
		t.Errorf("Emails don't match: %s vs %s", email1, email2)
	}

	// Verify handler can still read FormValue
	if req.FormValue("email") != "test@test.com" {
		t.Error("Handler cannot read form value after GetEmailFromForm")
	}

	t.Log("✅ Multiple reads work correctly")
}

func TestGetEmailFromForm_WithOtherFields(t *testing.T) {
	formData := url.Values{}
	formData.Set("email", "admin@site.com")
	formData.Set("password", "pass")
	formData.Set("remember_me", "true")
	formData.Set("other_field", "value")

	req := httptest.NewRequest("POST", "/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	email, err := GetEmailFromForm(req)
	if err != nil {
		t.Fatalf("GetEmailFromForm failed: %v", err)
	}
	if email != "admin@site.com" {
		t.Errorf("Expected 'admin@site.com', got '%s'", email)
	}

	// Verify other fields still accessible
	if req.FormValue("password") != "pass" {
		t.Error("Password field not preserved")
	}
	if req.FormValue("remember_me") != "true" {
		t.Error("Remember_me field not preserved")
	}
}

func TestGetEmailFromForm_URLEncodedSpaces(t *testing.T) {
	// Test with email that has spaces (should be URL encoded)
	formData := url.Values{}
	formData.Set("email", "user with spaces@example.com") // Intentionally malformed

	req := httptest.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	email, err := GetEmailFromForm(req)
	if err != nil {
		t.Fatalf("GetEmailFromForm failed: %v", err)
	}
	// url.Values.Encode() will properly handle the encoding
	if email != "user with spaces@example.com" {
		t.Errorf("Expected 'user with spaces@example.com', got '%s'", email)
	}
}
