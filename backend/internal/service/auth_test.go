package service

// mock jwt and test error in jwt service

import (
	"errors"
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	"golang.org/x/crypto/bcrypt"
)

type MockAuthStorage struct {
	SaveUserFunc   func(email string, passHash []byte) (int64, error)
	UserFunc       func(email string) (*domain.User, error)
	DeleteUserFunc func(email string) error
}

func (m *MockAuthStorage) SaveUser(email string, passHash []byte) (int64, error) {
	if m.SaveUserFunc != nil {
		return m.SaveUserFunc(email, passHash)
	}
	return 1, nil
}

func (m *MockAuthStorage) User(email string) (*domain.User, error) {
	if m.UserFunc != nil {
		return m.UserFunc(email)
	}
	passHash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	return &domain.User{Id: 1, Email: email, PassHash: passHash}, nil
}

func (m *MockAuthStorage) DeleteUser(email string) error {
	if m.DeleteUserFunc != nil {
		return m.DeleteUserFunc(email)
	}
	return nil
}

// later swap to real email implementation
type MockEmail struct {
	ConfirmFunc   func(email string) error
	IsCorrectFunc func(email string) error
}

func (m *MockEmail) Confirm(email string) error {
	if m.ConfirmFunc != nil {
		return m.ConfirmFunc(email)
	}
	return nil
}

func (m *MockEmail) IsCorrect(email string) error {
	if m.IsCorrectFunc != nil {
		return m.IsCorrectFunc(email)
	}
	return nil
}

type MockJwt struct {
	NewTokenFunc func(user *domain.User) (string, error)
}

func (m *MockJwt) NewToken(user *domain.User) (string, error) {
	if m.NewTokenFunc != nil {
		return m.NewTokenFunc(user)
	}
	return "test_token", nil
}

// TestSignup tests the Signup method of the Auth service.
func TestSignup(t *testing.T) {
	storage := &MockAuthStorage{}
	email := &MockEmail{}
	jwt := &MockJwt{} // Not used in Signup, but needed for the constructor
	service := NewAuth(storage, email, jwt)

	// Test successful signup
	email.IsCorrectFunc = func(e string) error { return nil }
	uid, err := service.Signup("test@example.com", "password")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if uid != 1 { // Assuming mock storage returns 1
		t.Errorf("Unexpected UID: got %d, expected %d", uid, 1)
	}

	// Test 2: GenerateFromPassword fail due to long password
	_, err = service.Signup("test@example.com", "passwordpasswordpasswordpasswordpasswordpasswordpasswordpasswordpasswordpasswordpasswordpasswordpassword")
	if err == nil {
		t.Error("Expected error")
	}

	// Test 3: storage SaveUser error
	var mockError error = errors.New("Mock SaveUserFunc")
	storage.SaveUserFunc = func(email string, passHash []byte) (int64, error) { return 0, mockError }
	uid, err = service.Signup("test@example.com", "password")
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Expected error %v, got %v", mockError, err)
	}

	// Test 4: invalid email
	mockError = errors.New("Mock IsCorrectFunc")
	email.IsCorrectFunc = func(e string) error { return mockError }
	_, err = service.Signup("invalid_email", "password")
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Expected %v, got: %v", mockError, err)
	}

	// Test 5: email confirm error
	mockError = errors.New("Mock ConfirmFunc")
	email.IsCorrectFunc = func(e string) error { return nil }
	email.ConfirmFunc = func(e string) error { return mockError }
	_, err = service.Signup("test@example.com", "password")
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Expected %v, got: %v", mockError, err)
	}
}

func TestLogin(t *testing.T) {
	storage := &MockAuthStorage{}
	email := &MockEmail{}
	jwt := &MockJwt{}
	service := NewAuth(storage, email, jwt)

	email.IsCorrectFunc = func(e string) error { return nil }

	// Test successful login
	token, err := service.Login("test@example.com", "password")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if token != "test_token" {
		t.Errorf("Unexpected token: got %s, expected %s", token, "test_token")
	}

	// Test jwt new token error
	mockError := errors.New("Mock UserFunc")
	service.jwt = &MockJwt{NewTokenFunc: func(user *domain.User) (string, error) { return "", mockError }}
	token, err = service.Login("test@example.com", "password")
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Expected error %v, got %v", mockError, err)
	}

	// Test incorrect password
	storage.UserFunc = func(email string) (*domain.User, error) {
		return &domain.User{Id: 1, Email: email, PassHash: []byte("$2a$10$7LqN.zLqN.zLqN.zLqN.zLqN.zLqN.zLqN.zLqN.zO")}, nil // Incorrect hash
	}
	_, err = service.Login("test@example.com", "wrong_password")
	if err == nil || err.Error() != "Wrong password" {
		t.Error("Expected error for incorrect password, got nil")
	}

	// Test storage.User error
	mockError = errors.New("Mock UserFunc")
	storage.UserFunc = func(email string) (*domain.User, error) { return nil, mockError }
	_, err = service.Login("invalid_email", "password")
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Expected %v, got: %v", mockError, err)
	}

	// Test email validation error
	mockError = errors.New("Mock IsCorrectFunc")
	email.IsCorrectFunc = func(e string) error { return mockError }

	_, err = service.Login("invalid_email", "password")
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Expected %v, got: %v", mockError, err)
	}

}
