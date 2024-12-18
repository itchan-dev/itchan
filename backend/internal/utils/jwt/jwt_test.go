package jwt

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
)

func TestNewToken(t *testing.T) {
	j := New("test_secret", time.Hour)
	user := &domain.User{
		Id:    1,
		Email: "test@example.com",
		Admin: true,
	}

	tokenString, err := j.NewToken(user)
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	if tokenString == "" {
		t.Errorf("NewToken() returned empty token")
	}

	// Verify the token can be decoded
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("test_secret"), nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse() error = %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["uid"] != float64(1) || claims["email"] != "test@example.com" || claims["admin"] != true {
		t.Fatalf("Claims are not valid")

	}
}

func TestDecodeToken(t *testing.T) {
	j := New("test_secret", time.Hour)
	user := &domain.User{
		Id:    1,
		Email: "test@example.com",
		Admin: true,
	}

	tokenString, err := j.NewToken(user)
	if err != nil {
		t.Fatalf("NewToken() error = %v", err) // Handle the error
	}

	// Valid token
	token, err := j.DecodeToken(tokenString)
	if err != nil {
		t.Errorf("DecodeToken() error = %v", err)
	}
	if token == nil {
		t.Errorf("DecodeToken() returned nil token")
	}

	// Invalid token (wrong signature)
	_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("wrong_secret"), nil
	})
	if err == nil {
		t.Fatalf("jwt.Parse() should return an error for incorrect signature, but did not")
	}

	_, err = j.DecodeToken(tokenString + "abc")
	if err == nil {
		t.Errorf("DecodeToken() expected error, but got nil")
	}

	e, ok := err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != http.StatusUnauthorized {
		t.Errorf("DecodeToken() with wrong sig expected error with status code %d, but got %v", http.StatusUnauthorized, err)
	}

	// Expired token
	jExpired := New("test_secret", -time.Hour)
	expiredTokenString, err := jExpired.NewToken(user)
	if err != nil {
		t.Fatalf("jExpired.NewToken() error = %v", err)
	}

	_, err = j.DecodeToken(expiredTokenString)
	if err == nil {
		t.Errorf("DecodeToken() expected error for expired token, but got nil")
	}
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != http.StatusUnauthorized {
		t.Errorf("DecodeToken() expected error with status code %d, but got %v", http.StatusUnauthorized, err)
	}

	// Invalid signing method
	invalidMethodToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{})
	invalidMethodTokenString, _ := invalidMethodToken.SignedString(nil) // No secret needed for none

	_, err = j.DecodeToken(invalidMethodTokenString)
	if err == nil {
		t.Error("DecodeToken() expected error for invalid signing method, but got nil") // Improved error message
	}
	e, ok = err.(*internal_errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != http.StatusUnauthorized {
		t.Errorf("DecodeToken() expected error with status code %d, but got %v", http.StatusUnauthorized, err)
	}

}
