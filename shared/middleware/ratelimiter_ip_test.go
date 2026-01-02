package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestGetIP_RemoteAddrOnly(t *testing.T) {
	// Test with valid RemoteAddr
	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	ip, err := GetIP(req)
	if err != nil {
		t.Fatalf("GetIP failed: %v", err)
	}
	if ip != "192.168.1.100" {
		t.Errorf("Expected IP '192.168.1.100', got '%s'", ip)
	}
}

func TestGetIP_IgnoresSpoofedHeaders(t *testing.T) {
	// Test that spoofed headers are IGNORED
	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "203.0.113.50:12345" // Real client IP

	// Attacker tries to spoof IP via headers
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.Header.Set("X-Forwarded-For", "10.0.0.2, 10.0.0.3")

	ip, err := GetIP(req)
	if err != nil {
		t.Fatalf("GetIP failed: %v", err)
	}

	// Should return RemoteAddr, NOT spoofed headers
	if ip != "203.0.113.50" {
		t.Errorf("GetIP returned spoofed IP '%s', should be '203.0.113.50'", ip)
	}

	t.Log("✅ Spoofed headers correctly ignored")
}

func TestGetIP_IPv6(t *testing.T) {
	// Test with IPv6 address
	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "[2001:db8::1]:8080"

	ip, err := GetIP(req)
	if err != nil {
		t.Fatalf("GetIP failed for IPv6: %v", err)
	}
	if ip != "2001:db8::1" {
		t.Errorf("Expected IPv6 '2001:db8::1', got '%s'", ip)
	}
}

func TestGetIP_Localhost(t *testing.T) {
	// Test with localhost (common in development)
	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	ip, err := GetIP(req)
	if err != nil {
		t.Fatalf("GetIP failed for localhost: %v", err)
	}
	if ip != "127.0.0.1" {
		t.Errorf("Expected '127.0.0.1', got '%s'", ip)
	}
}

func TestGetIP_NoPort(t *testing.T) {
	// Test with IP without port (fallback case)
	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.1"

	ip, err := GetIP(req)
	if err != nil {
		t.Fatalf("GetIP failed for IP without port: %v", err)
	}
	if ip != "192.168.1.1" {
		t.Errorf("Expected '192.168.1.1', got '%s'", ip)
	}
}

func TestGetIP_InvalidIP(t *testing.T) {
	// Test with invalid IP
	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "not-an-ip:1234"

	_, err := GetIP(req)
	if err == nil {
		t.Error("Expected error for invalid IP, got nil")
	}
	if err.Error() != "invalid IP address: not-an-ip" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestGetIP_DifferentPorts(t *testing.T) {
	// Verify same IP with different ports returns same IP (for rate limiting)
	testCases := []string{
		"192.168.1.100:54321",
		"192.168.1.100:11111",
		"192.168.1.100:22222",
	}

	for _, remoteAddr := range testCases {
		req := httptest.NewRequest("POST", "/test", nil)
		req.RemoteAddr = remoteAddr

		ip, err := GetIP(req)
		if err != nil {
			t.Fatalf("GetIP failed for %s: %v", remoteAddr, err)
		}
		if ip != "192.168.1.100" {
			t.Errorf("For %s, expected '192.168.1.100', got '%s'", remoteAddr, ip)
		}
	}

	t.Log("✅ Port correctly stripped from IP")
}
