package service

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// --- Mocks ---

type MockAuthStorage struct {
	SaveUserFunc               func(user domain.User) (domain.UserId, error)
	UserFunc                   func(email domain.Email) (domain.User, error)
	DeleteUserFunc             func(email domain.Email) error
	UpdatePasswordFunc         func(creds domain.Credentials) error
	SaveConfirmationDataFunc   func(data domain.ConfirmationData) error
	ConfirmationDataFunc       func(email domain.Email) (domain.ConfirmationData, error)
	DeleteConfirmationDataFunc func(email domain.Email) error
}

func (m *MockAuthStorage) SaveUser(user domain.User) (domain.UserId, error) {
	if m.SaveUserFunc != nil {
		return m.SaveUserFunc(user)
	}
	return 1, nil
}

func (m *MockAuthStorage) User(email domain.Email) (domain.User, error) {
	if m.UserFunc != nil {
		return m.UserFunc(email)
	}
	// Default success case for login tests
	passHash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	return domain.User{Id: 1, Email: email, PassHash: string(passHash)}, nil
}

func (m *MockAuthStorage) DeleteUser(email domain.Email) error {
	if m.DeleteUserFunc != nil {
		return m.DeleteUserFunc(email)
	}
	return nil
}

func (m *MockAuthStorage) UpdatePassword(creds domain.Credentials) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(creds)
	}
	return nil
}

func (m *MockAuthStorage) SaveConfirmationData(data domain.ConfirmationData) error {
	if m.SaveConfirmationDataFunc != nil {
		return m.SaveConfirmationDataFunc(data)
	}
	return nil
}

func (m *MockAuthStorage) ConfirmationData(email domain.Email) (domain.ConfirmationData, error) {
	if m.ConfirmationDataFunc != nil {
		return m.ConfirmationDataFunc(email)
	}
	// Default: Not found
	return domain.ConfirmationData{}, &internal_errors.ErrorWithStatusCode{
		Message:    "Confirmation data not found",
		StatusCode: http.StatusNotFound,
	}
}

func (m *MockAuthStorage) DeleteConfirmationData(email domain.Email) error {
	if m.DeleteConfirmationDataFunc != nil {
		return m.DeleteConfirmationDataFunc(email)
	}
	return nil
}

type MockEmail struct {
	SendFunc      func(recipientEmail, subject, body string) error
	IsCorrectFunc func(email domain.Email) error
}

func (m *MockEmail) Send(recipientEmail, subject, body string) error {
	if m.SendFunc != nil {
		return m.SendFunc(recipientEmail, subject, body)
	}
	return nil
}

func (m *MockEmail) IsCorrect(email domain.Email) error {
	if m.IsCorrectFunc != nil {
		return m.IsCorrectFunc(email)
	}
	// Default: Correct
	if !strings.Contains(email, "@") {
		return errors.New("invalid email format")
	}
	return nil
}

type MockJwt struct {
	NewTokenFunc func(user domain.User) (string, error)
}

func (m *MockJwt) NewToken(user domain.User) (string, error) {
	if m.NewTokenFunc != nil {
		return m.NewTokenFunc(user)
	}
	return "test_token", nil
}

// --- Tests ---

func TestRegister(t *testing.T) {
	storage := &MockAuthStorage{}
	email := &MockEmail{}
	jwt := &MockJwt{} // Not used in Register, but needed for constructor
	service := NewAuth(storage, email, jwt)

	creds := domain.Credentials{Email: "test@example.com", Password: "password"}
	lowerCaseEmail := strings.ToLower(creds.Email)

	t.Run("Successful registration", func(t *testing.T) {
		// Reset mocks for this subtest
		storage.ConfirmationDataFunc = nil // Reset to default (not found)
		storage.DeleteConfirmationDataFunc = nil
		storage.SaveConfirmationDataFunc = nil
		email.SendFunc = nil

		// Arrange
		saveCalled := false
		sendCalled := false
		storage.SaveConfirmationDataFunc = func(data domain.ConfirmationData) error {
			saveCalled = true
			assert.Equal(t, lowerCaseEmail, data.Email)
			assert.NotEmpty(t, data.NewPassHash)
			assert.NotEmpty(t, data.ConfirmationCodeHash)
			assert.True(t, data.Expires.After(time.Now().UTC().Add(-1*time.Minute))) // Allow for slight clock skew
			assert.True(t, data.Expires.Before(time.Now().UTC().Add(6*time.Minute))) // Should be around 5 mins expiry
			// Check if password was hashed correctly
			err := bcrypt.CompareHashAndPassword([]byte(data.NewPassHash), []byte(creds.Password))
			assert.NoError(t, err)
			return nil
		}
		email.SendFunc = func(recipientEmail, subject, body string) error {
			sendCalled = true
			assert.Equal(t, lowerCaseEmail, recipientEmail)
			assert.Equal(t, "Please confirm your email address", subject)
			assert.Contains(t, body, "Your confirmation code below")
			return nil
		}

		// Act
		err := service.Register(creds)

		// Assert
		require.NoError(t, err)
		assert.True(t, saveCalled, "SaveConfirmationData should be called")
		assert.True(t, sendCalled, "Send should be called")
	})

	t.Run("email.IsCorrect error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock IsCorrectFunc error")
		email.IsCorrectFunc = func(e domain.Email) error { return mockError }
		defer func() { email.IsCorrectFunc = nil }() // Restore default mock behavior

		// Act
		err := service.Register(domain.Credentials{Email: "invalid-email", Password: "password"})

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("storage.ConfirmationData general error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock ConfirmationDataFunc general error")
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return domain.ConfirmationData{}, mockError
		}
		defer func() { storage.ConfirmationDataFunc = nil }() // Restore default

		// Act
		err := service.Register(creds)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("Existing valid confirmation data", func(t *testing.T) {
		// Arrange
		expires := time.Now().UTC().Add(10 * time.Minute)
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return domain.ConfirmationData{
				Email:   email,
				Expires: expires,
			}, nil
		}
		defer func() { storage.ConfirmationDataFunc = nil }() // Restore default

		// Act
		err := service.Register(creds)

		// Assert
		require.Error(t, err)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusTooEarly, errWithStatus.StatusCode)
		assert.Contains(t, err.Error(), "Previous confirmation code is still valid. Retry after")
	})

	t.Run("Existing expired confirmation data gets deleted", func(t *testing.T) {
		// Arrange
		deleted := false
		saveCalled := false
		sendCalled := false
		expiredTime := time.Now().UTC().Add(-10 * time.Minute)

		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			// Return expired data on first call
			return domain.ConfirmationData{
				Email:   email,
				Expires: expiredTime,
			}, nil
		}
		storage.DeleteConfirmationDataFunc = func(email domain.Email) error {
			assert.Equal(t, lowerCaseEmail, email)
			deleted = true
			// After deleting, the next ConfirmationData call (if any) should find nothing
			storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
				return domain.ConfirmationData{}, &internal_errors.ErrorWithStatusCode{StatusCode: http.StatusNotFound}
			}
			return nil
		}
		storage.SaveConfirmationDataFunc = func(data domain.ConfirmationData) error {
			saveCalled = true
			return nil
		}
		email.SendFunc = func(recipientEmail, subject, body string) error {
			sendCalled = true
			return nil
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.DeleteConfirmationDataFunc = nil
			storage.SaveConfirmationDataFunc = nil
			email.SendFunc = nil
		}()

		// Act
		err := service.Register(creds)

		// Assert
		require.NoError(t, err)
		assert.True(t, deleted, "DeleteConfirmationData should be called for expired data")
		assert.True(t, saveCalled, "SaveConfirmationData should be called after deletion")
		assert.True(t, sendCalled, "Send should be called after deletion")
	})

	t.Run("storage.DeleteConfirmationData error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock DeleteConfirmationData error")
		expiredTime := time.Now().UTC().Add(-10 * time.Minute)
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return domain.ConfirmationData{Expires: expiredTime}, nil
		}
		storage.DeleteConfirmationDataFunc = func(email domain.Email) error {
			return mockError
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.DeleteConfirmationDataFunc = nil
		}()

		// Act
		err := service.Register(creds)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("bcrypt password hashing error", func(t *testing.T) {
		// This is hard to test reliably without mocking bcrypt, which is complex.
		// We assume bcrypt works correctly or handle its errors generically.
		// If bcrypt fails, it should return an error propagated by Register.
		// For now, we skip direct testing of bcrypt failure.
		t.Skip("Skipping direct test for bcrypt password hashing failure")
	})

	t.Run("bcrypt confirmation code hashing error", func(t *testing.T) {
		// Similar to password hashing, skipping direct test.
		t.Skip("Skipping direct test for bcrypt confirmation code hashing failure")
	})

	t.Run("storage.SaveConfirmationData error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock SaveConfirmationDataFunc error")
		storage.ConfirmationDataFunc = nil // Ensure default "not found"
		storage.SaveConfirmationDataFunc = func(data domain.ConfirmationData) error {
			return mockError
		}
		defer func() {
			storage.ConfirmationDataFunc = nil
			storage.SaveConfirmationDataFunc = nil
		}()

		// Act
		err := service.Register(creds)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("email.Send error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock SendFunc error")
		storage.ConfirmationDataFunc = nil                                                         // Ensure default "not found"
		storage.SaveConfirmationDataFunc = func(data domain.ConfirmationData) error { return nil } // Assume save works
		email.SendFunc = func(recipientEmail, subject, body string) error {
			return mockError
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.SaveConfirmationDataFunc = nil
			email.SendFunc = nil
		}()

		// Act
		err := service.Register(creds)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})
}

func TestCheckConfirmationCode(t *testing.T) {
	storage := &MockAuthStorage{}
	emailMock := &MockEmail{} // Renamed to avoid conflict with package name
	jwt := &MockJwt{}         // Not used in CheckConfirmationCode, but needed for constructor
	service := NewAuth(storage, emailMock, jwt)

	testEmail := "test@example.com"
	lowerCaseEmail := strings.ToLower(testEmail)
	confirmationCode := "123456"
	correctPassHash := "correct_hashed_password"
	correctCodeHashBytes, _ := bcrypt.GenerateFromPassword([]byte(confirmationCode), bcrypt.DefaultCost)
	correctCodeHash := string(correctCodeHashBytes)

	validConfirmationData := domain.ConfirmationData{
		Email:                lowerCaseEmail,
		NewPassHash:          correctPassHash,
		ConfirmationCodeHash: correctCodeHash,
		Expires:              time.Now().UTC().Add(5 * time.Minute),
	}

	t.Run("Successful confirmation (existing user)", func(t *testing.T) {
		// Arrange
		updateCalled := false
		deleteCalled := false
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			assert.Equal(t, lowerCaseEmail, email)
			return validConfirmationData, nil
		}
		// Simulate existing user found
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			assert.Equal(t, lowerCaseEmail, email)
			return domain.User{Id: 1, Email: email, PassHash: "old_hash"}, nil
		}
		storage.UpdatePasswordFunc = func(creds domain.Credentials) error {
			updateCalled = true
			assert.Equal(t, lowerCaseEmail, creds.Email)
			assert.Equal(t, correctPassHash, creds.Password) // Password field now holds the new hash
			return nil
		}
		storage.DeleteConfirmationDataFunc = func(email domain.Email) error {
			deleteCalled = true
			assert.Equal(t, lowerCaseEmail, email)
			return nil
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.UserFunc = nil
			storage.UpdatePasswordFunc = nil
			storage.DeleteConfirmationDataFunc = nil
		}()

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		require.NoError(t, err)
		assert.True(t, updateCalled, "UpdatePassword should be called for existing user")
		assert.True(t, deleteCalled, "DeleteConfirmationData should be called on success")
	})

	t.Run("Successful confirmation (new user creation)", func(t *testing.T) {
		// Arrange
		saveUserCalled := false
		deleteCalled := false
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			assert.Equal(t, lowerCaseEmail, email)
			return validConfirmationData, nil
		}
		// Simulate user not found
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			assert.Equal(t, lowerCaseEmail, email)
			return domain.User{}, &internal_errors.ErrorWithStatusCode{StatusCode: http.StatusNotFound}
		}
		storage.SaveUserFunc = func(user domain.User) (domain.UserId, error) {
			saveUserCalled = true
			assert.Equal(t, lowerCaseEmail, user.Email)
			assert.Equal(t, correctPassHash, user.PassHash)
			assert.False(t, user.Admin) // Ensure default admin status is false
			assert.Zero(t, user.Id)     // ID should be zero before saving
			return 5, nil               // Return some user ID
		}
		storage.DeleteConfirmationDataFunc = func(email domain.Email) error {
			deleteCalled = true
			assert.Equal(t, lowerCaseEmail, email)
			return nil
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.UserFunc = nil
			storage.SaveUserFunc = nil
			storage.DeleteConfirmationDataFunc = nil
		}()

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		require.NoError(t, err)
		assert.True(t, saveUserCalled, "SaveUser should be called for new user")
		assert.True(t, deleteCalled, "DeleteConfirmationData should be called on success")
	})

	t.Run("email.IsCorrect error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock IsCorrectFunc error")
		emailMock.IsCorrectFunc = func(e domain.Email) error { return mockError }
		defer func() { emailMock.IsCorrectFunc = nil }() // Restore default

		// Act
		err := service.CheckConfirmationCode("invalid-email", confirmationCode)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("storage.ConfirmationData not found error", func(t *testing.T) {
		// Arrange
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return domain.ConfirmationData{}, &internal_errors.ErrorWithStatusCode{StatusCode: http.StatusNotFound, Message: "not found"}
		}
		defer func() { storage.ConfirmationDataFunc = nil }() // Restore default

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		require.Error(t, err)
		// Check if it's the expected NotFound error from storage
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusNotFound, errWithStatus.StatusCode)
	})

	t.Run("storage.ConfirmationData general error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock ConfirmationDataFunc general error")
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return domain.ConfirmationData{}, mockError
		}
		defer func() { storage.ConfirmationDataFunc = nil }() // Restore default

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError)) // Should propagate the original error
	})

	t.Run("Confirmation time expired", func(t *testing.T) {
		// Arrange
		expiredData := validConfirmationData
		expiredData.Expires = time.Now().UTC().Add(-5 * time.Minute)
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return expiredData, nil
		}
		defer func() { storage.ConfirmationDataFunc = nil }() // Restore default

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		require.Error(t, err)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusBadRequest, errWithStatus.StatusCode)
		assert.Equal(t, "Confirmation time expired", errWithStatus.Message)
	})

	t.Run("Wrong confirmation code", func(t *testing.T) {
		// Arrange
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return validConfirmationData, nil // Use data with the CORRECT hash
		}
		defer func() { storage.ConfirmationDataFunc = nil }() // Restore default

		// Act
		err := service.CheckConfirmationCode(testEmail, "wrong_code_654321") // Provide the WRONG code

		// Assert
		require.Error(t, err)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusBadRequest, errWithStatus.StatusCode)
		assert.Equal(t, "Wrong confirmation code", errWithStatus.Message)
	})

	t.Run("storage.User general error (during check if user exists)", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock storage.User general error")
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return validConfirmationData, nil
		}
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			return domain.User{}, mockError // Return a non-NotFound error
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.UserFunc = nil
		}()

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError)) // Propagates the storage error
	})

	t.Run("storage.SaveUser error (during new user creation)", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock SaveUser error")
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return validConfirmationData, nil
		}
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			return domain.User{}, &internal_errors.ErrorWithStatusCode{StatusCode: http.StatusNotFound} // Trigger new user path
		}
		storage.SaveUserFunc = func(user domain.User) (domain.UserId, error) {
			return 0, mockError // Fail the save
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.UserFunc = nil
			storage.SaveUserFunc = nil
		}()

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("storage.UpdatePassword error (during existing user update)", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock UpdatePassword error")
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return validConfirmationData, nil
		}
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			return domain.User{Id: 1, Email: email}, nil // Trigger existing user path
		}
		storage.UpdatePasswordFunc = func(creds domain.Credentials) error {
			return mockError // Fail the update
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.UserFunc = nil
			storage.UpdatePasswordFunc = nil
		}()

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("storage.DeleteConfirmationData error (at the end)", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock DeleteConfirmationData error")
		storage.ConfirmationDataFunc = func(email domain.Email) (domain.ConfirmationData, error) {
			return validConfirmationData, nil
		}
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			return domain.User{Id: 1, Email: email}, nil // Existing user path
		}
		storage.UpdatePasswordFunc = func(creds domain.Credentials) error {
			return nil // Update succeeds
		}
		storage.DeleteConfirmationDataFunc = func(email domain.Email) error {
			return mockError // Delete fails
		}
		defer func() { // Restore defaults
			storage.ConfirmationDataFunc = nil
			storage.UserFunc = nil
			storage.UpdatePasswordFunc = nil
			storage.DeleteConfirmationDataFunc = nil
		}()

		// Act
		err := service.CheckConfirmationCode(testEmail, confirmationCode)

		// Assert
		// The primary operation (update/create user) succeeded, but cleanup failed.
		// The service currently propagates this error.
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

}

func TestLogin(t *testing.T) {
	storage := &MockAuthStorage{}
	emailMock := &MockEmail{} // Renamed to avoid conflict
	jwt := &MockJwt{}
	service := NewAuth(storage, emailMock, jwt)

	creds := domain.Credentials{Email: "test@example.com", Password: "password"}
	lowerCaseEmail := strings.ToLower(creds.Email)

	correctPassHashBytes, _ := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	correctPassHash := string(correctPassHashBytes)
	correctUser := domain.User{Id: 1, Email: lowerCaseEmail, PassHash: correctPassHash, Admin: false}

	t.Run("Successful login", func(t *testing.T) {
		// Arrange
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			assert.Equal(t, lowerCaseEmail, email)
			return correctUser, nil
		}
		jwt.NewTokenFunc = func(user domain.User) (string, error) {
			assert.Equal(t, correctUser.Id, user.Id)
			assert.Equal(t, correctUser.Email, user.Email)
			assert.Equal(t, correctUser.PassHash, user.PassHash) // PassHash included in token generation
			assert.Equal(t, correctUser.Admin, user.Admin)
			return "success_token", nil
		}
		defer func() { // Restore defaults
			storage.UserFunc = nil
			jwt.NewTokenFunc = nil
		}()

		// Act
		token, err := service.Login(creds)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "success_token", token)
	})

	t.Run("email.IsCorrect error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock IsCorrectFunc error")
		emailMock.IsCorrectFunc = func(e domain.Email) error { return mockError }
		defer func() { emailMock.IsCorrectFunc = nil }() // Restore default

		// Act
		token, err := service.Login(domain.Credentials{Email: "invalid-email", Password: "password"})

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
		assert.Empty(t, token)
	})

	t.Run("storage.User not found error", func(t *testing.T) {
		// Arrange
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			return domain.User{}, &internal_errors.ErrorWithStatusCode{
				Message:    "User not found in storage",
				StatusCode: http.StatusNotFound,
			}
		}
		defer func() { storage.UserFunc = nil }() // Restore default

		// Act
		token, err := service.Login(creds)

		// Assert
		require.Error(t, err)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusUnauthorized, errWithStatus.StatusCode)
		assert.Equal(t, "Invalid credentials", errWithStatus.Message) // Generic message
		assert.Empty(t, token)
	})

	t.Run("storage.User general error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock UserFunc general error")
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			return domain.User{}, mockError
		}
		defer func() { storage.UserFunc = nil }() // Restore default

		// Act
		token, err := service.Login(creds)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError)) // Propagates the storage error
		assert.Empty(t, token)
	})

	t.Run("Incorrect password", func(t *testing.T) {
		// Arrange
		// Mock UserFunc returns the user but CompareHashAndPassword will fail
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			assert.Equal(t, lowerCaseEmail, email)
			return correctUser, nil // Return user with the CORRECT hash
		}
		defer func() { storage.UserFunc = nil }() // Restore default

		// Act
		// Use the WRONG password in credentials
		token, err := service.Login(domain.Credentials{Email: creds.Email, Password: "wrong_password"})

		// Assert
		require.Error(t, err)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusUnauthorized, errWithStatus.StatusCode)
		assert.Equal(t, "Invalid credentials", errWithStatus.Message)
		assert.Empty(t, token)
	})

	t.Run("jwt.NewToken error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock NewTokenFunc error")
		storage.UserFunc = func(email domain.Email) (domain.User, error) {
			return correctUser, nil // Login credentials are correct
		}
		jwt.NewTokenFunc = func(user domain.User) (string, error) {
			return "", mockError // JWT generation fails
		}
		defer func() { // Restore defaults
			storage.UserFunc = nil
			jwt.NewTokenFunc = nil
		}()

		// Act
		token, err := service.Login(creds)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError)) // Propagates the JWT error
		assert.Empty(t, token)
	})
}
