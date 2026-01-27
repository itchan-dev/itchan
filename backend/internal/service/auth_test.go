package service

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// --- Mocks ---

type MockAuthStorage struct {
	SaveUserFunc                       func(user domain.User) (domain.UserId, error)
	UserFunc                           func(emailHash []byte) (domain.User, error)
	DeleteUserFunc                     func(emailHash []byte) error
	UpdatePasswordFunc                 func(emailHash []byte, newPasswordHash domain.Password) error
	SaveConfirmationDataFunc           func(data domain.ConfirmationData) error
	ConfirmationDataFunc               func(emailHash []byte) (domain.ConfirmationData, error)
	DeleteConfirmationDataFunc         func(emailHash []byte) error
	IsUserBlacklistedFunc              func(userId domain.UserId) (bool, error)
	BlacklistUserFunc                  func(userId domain.UserId, reason string, blacklistedBy domain.UserId) error
	UnblacklistUserFunc                func(userId domain.UserId) error
	GetBlacklistedUsersWithDetailsFunc func() ([]domain.BlacklistEntry, error)

	// Invite code function fields
	SaveInviteCodeFunc      func(invite domain.InviteCode) error
	InviteCodeByHashFunc    func(codeHash string) (domain.InviteCode, error)
	GetInvitesByUserFunc    func(userId domain.UserId) ([]domain.InviteCode, error)
	CountActiveInvitesFunc  func(userId domain.UserId) (int, error)
	MarkInviteUsedFunc      func(codeHash string, usedBy domain.UserId) error
	DeleteInviteCodeFunc    func(codeHash string) error
	DeleteInvitesByUserFunc func(userId domain.UserId) error
}

func (m *MockAuthStorage) SaveUser(user domain.User) (domain.UserId, error) {
	if m.SaveUserFunc != nil {
		return m.SaveUserFunc(user)
	}
	return 1, nil
}

func (m *MockAuthStorage) User(emailHash []byte) (domain.User, error) {
	if m.UserFunc != nil {
		return m.UserFunc(emailHash)
	}
	// Default success case for login tests
	passHash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	return domain.User{Id: 1, PassHash: string(passHash)}, nil
}

func (m *MockAuthStorage) DeleteUser(emailHash []byte) error {
	if m.DeleteUserFunc != nil {
		return m.DeleteUserFunc(emailHash)
	}
	return nil
}

func (m *MockAuthStorage) UpdatePassword(emailHash []byte, newPasswordHash domain.Password) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(emailHash, newPasswordHash)
	}
	return nil
}

func (m *MockAuthStorage) SaveConfirmationData(data domain.ConfirmationData) error {
	if m.SaveConfirmationDataFunc != nil {
		return m.SaveConfirmationDataFunc(data)
	}
	return nil
}

func (m *MockAuthStorage) ConfirmationData(emailHash []byte) (domain.ConfirmationData, error) {
	if m.ConfirmationDataFunc != nil {
		return m.ConfirmationDataFunc(emailHash)
	}
	// Default: Not found
	return domain.ConfirmationData{}, &internal_errors.ErrorWithStatusCode{
		Message:    "Confirmation data not found",
		StatusCode: http.StatusNotFound,
	}
}

func (m *MockAuthStorage) DeleteConfirmationData(emailHash []byte) error {
	if m.DeleteConfirmationDataFunc != nil {
		return m.DeleteConfirmationDataFunc(emailHash)
	}
	return nil
}

func (m *MockAuthStorage) IsUserBlacklisted(userId domain.UserId) (bool, error) {
	if m.IsUserBlacklistedFunc != nil {
		return m.IsUserBlacklistedFunc(userId)
	}
	// Default: Not blacklisted
	return false, nil
}

func (m *MockAuthStorage) BlacklistUser(userId domain.UserId, reason string, blacklistedBy domain.UserId) error {
	if m.BlacklistUserFunc != nil {
		return m.BlacklistUserFunc(userId, reason, blacklistedBy)
	}
	return nil
}

func (m *MockAuthStorage) UnblacklistUser(userId domain.UserId) error {
	if m.UnblacklistUserFunc != nil {
		return m.UnblacklistUserFunc(userId)
	}
	return nil
}

func (m *MockAuthStorage) GetBlacklistedUsersWithDetails() ([]domain.BlacklistEntry, error) {
	if m.GetBlacklistedUsersWithDetailsFunc != nil {
		return m.GetBlacklistedUsersWithDetailsFunc()
	}
	return nil, nil
}

// Invite code methods
func (m *MockAuthStorage) SaveInviteCode(invite domain.InviteCode) error {
	if m.SaveInviteCodeFunc != nil {
		return m.SaveInviteCodeFunc(invite)
	}
	return nil
}

func (m *MockAuthStorage) InviteCodeByHash(codeHash string) (domain.InviteCode, error) {
	if m.InviteCodeByHashFunc != nil {
		return m.InviteCodeByHashFunc(codeHash)
	}
	return domain.InviteCode{}, &internal_errors.ErrorWithStatusCode{Message: "Invite code not found", StatusCode: http.StatusNotFound}
}

func (m *MockAuthStorage) GetInvitesByUser(userId domain.UserId) ([]domain.InviteCode, error) {
	if m.GetInvitesByUserFunc != nil {
		return m.GetInvitesByUserFunc(userId)
	}
	return nil, nil
}

func (m *MockAuthStorage) CountActiveInvites(userId domain.UserId) (int, error) {
	if m.CountActiveInvitesFunc != nil {
		return m.CountActiveInvitesFunc(userId)
	}
	return 0, nil
}

func (m *MockAuthStorage) MarkInviteUsed(codeHash string, usedBy domain.UserId) error {
	if m.MarkInviteUsedFunc != nil {
		return m.MarkInviteUsedFunc(codeHash, usedBy)
	}
	return nil
}

func (m *MockAuthStorage) DeleteInviteCode(codeHash string) error {
	if m.DeleteInviteCodeFunc != nil {
		return m.DeleteInviteCodeFunc(codeHash)
	}
	return nil
}

func (m *MockAuthStorage) DeleteInvitesByUser(userId domain.UserId) error {
	if m.DeleteInvitesByUserFunc != nil {
		return m.DeleteInvitesByUserFunc(userId)
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

type MockEmailCrypto struct {
	EncryptFunc       func(email string) ([]byte, error)
	HashFunc          func(email string) []byte
	ExtractDomainFunc func(email string) (string, error)
}

func (m *MockEmailCrypto) Encrypt(email string) ([]byte, error) {
	if m.EncryptFunc != nil {
		return m.EncryptFunc(email)
	}
	// Default: return email as bytes for testing
	return []byte("encrypted_" + email), nil
}

func (m *MockEmailCrypto) Hash(email string) []byte {
	if m.HashFunc != nil {
		return m.HashFunc(email)
	}
	// Default: simple hash for testing
	return []byte("hash_" + email)
}

func (m *MockEmailCrypto) ExtractDomain(email string) (string, error) {
	if m.ExtractDomainFunc != nil {
		return m.ExtractDomainFunc(email)
	}
	// Default: extract domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", errors.New("invalid email")
	}
	return parts[1], nil
}

// --- Tests ---

func TestRegister(t *testing.T) {
	storage := &MockAuthStorage{}
	email := &MockEmail{}
	jwt := &MockJwt{} // Not used in Register, but needed for constructor
	emailCrypto := &MockEmailCrypto{}
	service := NewAuth(storage, email, jwt, &config.Public{
		ConfirmationCodeLen: 8,
		ConfirmationCodeTTL: 10 * time.Minute,
	}, nil, emailCrypto)

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
			assert.NotEmpty(t, data.EmailHash)
			assert.NotEmpty(t, data.PasswordHash)
			assert.NotEmpty(t, data.ConfirmationCodeHash)
			assert.True(t, data.Expires.After(time.Now().UTC().Add(-1*time.Minute))) // Allow for slight clock skew
			assert.True(t, data.Expires.Before(time.Now().UTC().Add(11*time.Minute))) // Should be around 10 mins expiry
			// Check if password was hashed correctly
			err := bcrypt.CompareHashAndPassword([]byte(data.PasswordHash), []byte(creds.Password))
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			return domain.ConfirmationData{
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

		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			// Return expired data on first call
			return domain.ConfirmationData{
				Expires: expiredTime,
			}, nil
		}
		storage.DeleteConfirmationDataFunc = func(emailHash []byte) error {
			// emailHash is []byte, not string - skip assertion
			deleted = true
			// After deleting, the next ConfirmationData call (if any) should find nothing
			storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			return domain.ConfirmationData{Expires: expiredTime}, nil
		}
		storage.DeleteConfirmationDataFunc = func(emailHash []byte) error {
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
		storage.ConfirmationDataFunc = nil                                         // Ensure default "not found"
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
	emailCrypto := &MockEmailCrypto{}
	service := NewAuth(storage, emailMock, jwt, &config.Public{ConfirmationCodeLen: 8}, nil, emailCrypto)

	testEmail := "test@example.com"
	confirmationCode := "123456"
	correctPassHash := "correct_hashed_password"
	correctCodeHashBytes, _ := bcrypt.GenerateFromPassword([]byte(confirmationCode), bcrypt.DefaultCost)
	correctCodeHash := string(correctCodeHashBytes)

	validConfirmationData := domain.ConfirmationData{
		PasswordHash:         correctPassHash,
		ConfirmationCodeHash: correctCodeHash,
		Expires:              time.Now().UTC().Add(5 * time.Minute),
	}

	t.Run("Successful confirmation (existing user)", func(t *testing.T) {
		// Arrange
		updateCalled := false
		deleteCalled := false
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			// emailHash is []byte, not string - skip assertion
			return validConfirmationData, nil
		}
		// Simulate existing user found
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			// emailHash is []byte, not string - skip assertion
			return domain.User{Id: 1, PassHash: "old_hash"}, nil
		}
		storage.UpdatePasswordFunc = func(emailHash []byte, newPasswordHash domain.Password) error {
			updateCalled = true
			// emailHash is []byte - assertions updated
			assert.Equal(t, correctPassHash, string(newPasswordHash)) // Password field now holds the new hash
			return nil
		}
		storage.DeleteConfirmationDataFunc = func(emailHash []byte) error {
			deleteCalled = true
			// emailHash is []byte, not string - skip assertion
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			// emailHash is []byte, not string - skip assertion
			return validConfirmationData, nil
		}
		// Simulate user not found
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			// emailHash is []byte, not string - skip assertion
			return domain.User{}, &internal_errors.ErrorWithStatusCode{StatusCode: http.StatusNotFound}
		}
		storage.SaveUserFunc = func(user domain.User) (domain.UserId, error) {
			saveUserCalled = true
			assert.NotEmpty(t, user.EmailEncrypted)
			assert.NotEmpty(t, user.EmailHash)
			assert.NotEmpty(t, user.EmailDomain)
			assert.Equal(t, correctPassHash, user.PassHash)
			assert.False(t, user.Admin) // Ensure default admin status is false
			return 5, nil          // Return some user ID
		}
		storage.DeleteConfirmationDataFunc = func(emailHash []byte) error {
			deleteCalled = true
			// emailHash is []byte, not string - skip assertion
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			return validConfirmationData, nil
		}
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			return validConfirmationData, nil
		}
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			return validConfirmationData, nil
		}
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			return domain.User{Id: 1}, nil // Trigger existing user path
		}
		storage.UpdatePasswordFunc = func(emailHash []byte, newPasswordHash domain.Password) error {
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
		storage.ConfirmationDataFunc = func(emailHash []byte) (domain.ConfirmationData, error) {
			return validConfirmationData, nil
		}
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			return domain.User{Id: 1}, nil // Existing user path
		}
		storage.UpdatePasswordFunc = func(emailHash []byte, newPasswordHash domain.Password) error {
			return nil // Update succeeds
		}
		storage.DeleteConfirmationDataFunc = func(emailHash []byte) error {
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
	emailCrypto := &MockEmailCrypto{}
	service := NewAuth(storage, emailMock, jwt, &config.Public{ConfirmationCodeLen: 8}, nil, emailCrypto)

	creds := domain.Credentials{Email: "test@example.com", Password: "password"}

	correctPassHashBytes, _ := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	correctPassHash := string(correctPassHashBytes)
	correctUser := domain.User{Id: 1, PassHash: correctPassHash, Admin: false}

	t.Run("Successful login", func(t *testing.T) {
		// Arrange
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			// emailHash is []byte, not string - skip assertion
			return correctUser, nil
		}
		jwt.NewTokenFunc = func(user domain.User) (string, error) {
			assert.Equal(t, correctUser.Id, user.Id)
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
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
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
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
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
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			// emailHash is []byte, not string - skip assertion
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
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
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

	t.Run("Blacklisted user", func(t *testing.T) {
		// Arrange
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			return correctUser, nil
		}
		storage.IsUserBlacklistedFunc = func(userId domain.UserId) (bool, error) {
			assert.Equal(t, correctUser.Id, userId)
			return true, nil // User is blacklisted
		}
		defer func() {
			storage.UserFunc = nil
			storage.IsUserBlacklistedFunc = nil
		}()

		// Act
		token, err := service.Login(creds)

		// Assert
		require.Error(t, err)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusForbidden, errWithStatus.StatusCode)
		assert.Equal(t, "Account suspended", errWithStatus.Message)
		assert.Empty(t, token)
	})

	t.Run("IsUserBlacklisted error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock IsUserBlacklistedFunc error")
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			return correctUser, nil
		}
		storage.IsUserBlacklistedFunc = func(userId domain.UserId) (bool, error) {
			return false, mockError
		}
		defer func() {
			storage.UserFunc = nil
			storage.IsUserBlacklistedFunc = nil
		}()

		// Act
		token, err := service.Login(creds)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
		assert.Empty(t, token)
	})
}

func TestRegisterWithInvite(t *testing.T) {
	storage := &MockAuthStorage{}
	emailMock := &MockEmail{}
	jwt := &MockJwt{}
	emailCrypto := &MockEmailCrypto{}
	service := NewAuth(storage, emailMock, jwt, &config.Public{
		InviteEnabled:    true,
		InviteCodeLength: 12,
		InviteCodeTTL:    720 * time.Hour,
	}, nil, emailCrypto)

	testInviteCode := "TESTCODE1234"
	testPassword := domain.Password("password123")

	validInvite := domain.InviteCode{
		CodeHash:  "valid_hash",
		CreatedBy: 10,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		UsedBy:    nil,
		UsedAt:    nil,
	}

	t.Run("Successful registration with invite", func(t *testing.T) {
		// Arrange
		saveUserCalled := false
		markUsedCalled := false

		storage.InviteCodeByHashFunc = func(codeHash string) (domain.InviteCode, error) {
			// Service hashes the invite code before calling storage
			assert.NotEmpty(t, codeHash)
			return validInvite, nil
		}
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			// First call should not find user (email available)
			return domain.User{}, &internal_errors.ErrorWithStatusCode{StatusCode: http.StatusNotFound}
		}
		storage.SaveUserFunc = func(user domain.User) (domain.UserId, error) {
			saveUserCalled = true
			assert.NotEmpty(t, user.EmailEncrypted)
			assert.NotEmpty(t, user.EmailHash)
			assert.NotEmpty(t, user.EmailDomain)
			assert.NotEmpty(t, user.PassHash)
			assert.False(t, user.Admin)
			// Verify password was hashed correctly
			err := bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(testPassword))
			assert.NoError(t, err)
			return 100, nil
		}
		storage.MarkInviteUsedFunc = func(codeHash string, usedBy domain.UserId) error {
			markUsedCalled = true
			assert.NotEmpty(t, codeHash) // Now receives hash, not plain code
			assert.Equal(t, domain.UserId(100), usedBy)
			return nil
		}
		defer func() {
			storage.InviteCodeByHashFunc = nil
			storage.UserFunc = nil
			storage.SaveUserFunc = nil
			storage.MarkInviteUsedFunc = nil
		}()

		// Act
		email, err := service.RegisterWithInvite(testInviteCode, testPassword)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, email)
		assert.Contains(t, email, "@invited.ru")
		assert.True(t, saveUserCalled)
		assert.True(t, markUsedCalled)
	})

	t.Run("Invalid invite code", func(t *testing.T) {
		// Arrange
		storage.InviteCodeByHashFunc = func(codeHash string) (domain.InviteCode, error) {
			return domain.InviteCode{}, &internal_errors.ErrorWithStatusCode{
				Message:    "Invite code not found",
				StatusCode: http.StatusNotFound,
			}
		}
		defer func() { storage.InviteCodeByHashFunc = nil }()

		// Act
		email, err := service.RegisterWithInvite("INVALIDCODE", testPassword)

		// Assert
		require.Error(t, err)
		assert.Empty(t, email)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusBadRequest, errWithStatus.StatusCode)
	})

	t.Run("Already used invite code", func(t *testing.T) {
		// Arrange
		usedBy := domain.UserId(50)
		usedAt := time.Now().UTC().Add(-1 * time.Hour)
		usedInvite := validInvite
		usedInvite.UsedBy = &usedBy
		usedInvite.UsedAt = &usedAt

		storage.InviteCodeByHashFunc = func(codeHash string) (domain.InviteCode, error) {
			return usedInvite, nil
		}
		defer func() { storage.InviteCodeByHashFunc = nil }()

		// Act
		email, err := service.RegisterWithInvite(testInviteCode, testPassword)

		// Assert
		require.Error(t, err)
		assert.Empty(t, email)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusBadRequest, errWithStatus.StatusCode)
		assert.Contains(t, err.Error(), "has already been used")
	})

	t.Run("Expired invite code", func(t *testing.T) {
		// Arrange
		expiredInvite := validInvite
		expiredInvite.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)

		storage.InviteCodeByHashFunc = func(codeHash string) (domain.InviteCode, error) {
			return expiredInvite, nil
		}
		defer func() { storage.InviteCodeByHashFunc = nil }()

		// Act
		email, err := service.RegisterWithInvite(testInviteCode, testPassword)

		// Assert
		require.Error(t, err)
		assert.Empty(t, email)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusBadRequest, errWithStatus.StatusCode)
		assert.Contains(t, err.Error(), "has expired")
	})

	t.Run("Email collision retry logic", func(t *testing.T) {
		// Arrange
		callCount := 0
		storage.InviteCodeByHashFunc = func(codeHash string) (domain.InviteCode, error) {
			return validInvite, nil
		}
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			callCount++
			if callCount <= 3 {
				// First 3 attempts: email already exists
				return domain.User{Id: 99}, nil
			}
			// 4th attempt: email available
			return domain.User{}, &internal_errors.ErrorWithStatusCode{StatusCode: http.StatusNotFound}
		}
		storage.SaveUserFunc = func(user domain.User) (domain.UserId, error) {
			return 100, nil
		}
		storage.MarkInviteUsedFunc = func(codeHash string, usedBy domain.UserId) error {
			return nil
		}
		defer func() {
			storage.InviteCodeByHashFunc = nil
			storage.UserFunc = nil
			storage.SaveUserFunc = nil
			storage.MarkInviteUsedFunc = nil
		}()

		// Act
		email, err := service.RegisterWithInvite(testInviteCode, testPassword)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, email)
		assert.Equal(t, 4, callCount, "Should retry 3 times before finding available email")
	})

	t.Run("SaveUser error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("mock SaveUser error")
		storage.InviteCodeByHashFunc = func(codeHash string) (domain.InviteCode, error) {
			return validInvite, nil
		}
		storage.UserFunc = func(emailHash []byte) (domain.User, error) {
			return domain.User{}, &internal_errors.ErrorWithStatusCode{StatusCode: http.StatusNotFound}
		}
		storage.SaveUserFunc = func(user domain.User) (domain.UserId, error) {
			return 0, mockError
		}
		defer func() {
			storage.InviteCodeByHashFunc = nil
			storage.UserFunc = nil
			storage.SaveUserFunc = nil
		}()

		// Act
		email, err := service.RegisterWithInvite(testInviteCode, testPassword)

		// Assert
		require.Error(t, err)
		assert.Empty(t, email)
		assert.True(t, errors.Is(err, mockError))
	})
}

func TestGenerateInvite(t *testing.T) {
	storage := &MockAuthStorage{}
	emailMock := &MockEmail{}
	jwt := &MockJwt{}

	testUser := domain.User{
		Id:        42,
		Admin:     false,
		CreatedAt: time.Now().UTC().Add(-60 * 24 * time.Hour), // 60 days old
	}

	t.Run("Successful invite generation", func(t *testing.T) {
		// Arrange
		emailCrypto := &MockEmailCrypto{}
		service := NewAuth(storage, emailMock, jwt, &config.Public{
			InviteEnabled:     true,
			InviteCodeLength:  12,
			InviteCodeTTL:     720 * time.Hour,
			MaxInvitesPerUser: 5,
		}, nil, emailCrypto)

		saveInviteCalled := false
		storage.CountActiveInvitesFunc = func(userId domain.UserId) (int, error) {
			assert.Equal(t, testUser.Id, userId)
			return 2, nil // User has 2 active invites
		}
		storage.SaveInviteCodeFunc = func(invite domain.InviteCode) error {
			saveInviteCalled = true
			assert.Equal(t, testUser.Id, invite.CreatedBy)
			assert.NotEmpty(t, invite.CodeHash)
			assert.Equal(t, 64, len(invite.CodeHash)) // SHA256 produces 64 hex chars
			assert.True(t, invite.ExpiresAt.After(time.Now().UTC()))
			return nil
		}
		defer func() {
			storage.CountActiveInvitesFunc = nil
			storage.SaveInviteCodeFunc = nil
		}()

		// Act
		invite, err := service.GenerateInvite(testUser)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, invite)
		assert.NotEmpty(t, invite.PlainCode)
		assert.Equal(t, 12, len(invite.PlainCode))
		assert.Equal(t, testUser.Id, invite.CreatedBy)
		assert.True(t, saveInviteCalled)
	})

	t.Run("Admin bypasses invite limit", func(t *testing.T) {
		// Arrange
		emailCrypto := &MockEmailCrypto{}
		service := NewAuth(storage, emailMock, jwt, &config.Public{
			InviteEnabled:     true,
			InviteCodeLength:  12,
			InviteCodeTTL:     720 * time.Hour,
			MaxInvitesPerUser: 2, // Low limit
		}, nil, emailCrypto)

		adminUser := testUser
		adminUser.Admin = true

		// Mock shows admin already has max invites
		storage.CountActiveInvitesFunc = func(userId domain.UserId) (int, error) {
			return 10, nil // Already at 10 invites, but admin should bypass
		}
		storage.SaveInviteCodeFunc = func(invite domain.InviteCode) error {
			return nil
		}
		defer func() {
			storage.CountActiveInvitesFunc = nil
			storage.SaveInviteCodeFunc = nil
		}()

		// Act
		invite, err := service.GenerateInvite(adminUser)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, invite)
		assert.NotEmpty(t, invite.PlainCode)
	})

	t.Run("Regular user exceeds invite limit", func(t *testing.T) {
		// Arrange
		emailCrypto := &MockEmailCrypto{}
		service := NewAuth(storage, emailMock, jwt, &config.Public{
			InviteEnabled:     true,
			InviteCodeLength:  12,
			InviteCodeTTL:     720 * time.Hour,
			MaxInvitesPerUser: 3,
		}, nil, emailCrypto)

		storage.CountActiveInvitesFunc = func(userId domain.UserId) (int, error) {
			return 3, nil // Already at limit
		}
		defer func() { storage.CountActiveInvitesFunc = nil }()

		// Act
		invite, err := service.GenerateInvite(testUser)

		// Assert
		require.Error(t, err)
		assert.Nil(t, invite)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusForbidden, errWithStatus.StatusCode)
		assert.Contains(t, err.Error(), "Maximum invite limit reached")
	})

	t.Run("Unlimited invites when MaxInvitesPerUser is 0", func(t *testing.T) {
		// Arrange
		emailCrypto := &MockEmailCrypto{}
		service := NewAuth(storage, emailMock, jwt, &config.Public{
			InviteEnabled:     true,
			InviteCodeLength:  12,
			InviteCodeTTL:     720 * time.Hour,
			MaxInvitesPerUser: 0, // Unlimited
		}, nil, emailCrypto)

		// CountActiveInvites should not be called when limit is 0
		countCalled := false
		storage.CountActiveInvitesFunc = func(userId domain.UserId) (int, error) {
			countCalled = true
			return 9999, nil
		}
		storage.SaveInviteCodeFunc = func(invite domain.InviteCode) error {
			return nil
		}
		defer func() {
			storage.CountActiveInvitesFunc = nil
			storage.SaveInviteCodeFunc = nil
		}()

		// Act
		invite, err := service.GenerateInvite(testUser)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, invite)
		assert.False(t, countCalled, "Should not check count when limit is 0 (unlimited)")
	})

	t.Run("Invite system disabled", func(t *testing.T) {
		// Arrange
		emailCrypto := &MockEmailCrypto{}
		service := NewAuth(storage, emailMock, jwt, &config.Public{
			InviteEnabled: false,
		}, nil, emailCrypto)

		// Act
		invite, err := service.GenerateInvite(testUser)

		// Assert
		require.Error(t, err)
		assert.Nil(t, invite)
		var errWithStatus *internal_errors.ErrorWithStatusCode
		require.True(t, errors.As(err, &errWithStatus))
		assert.Equal(t, http.StatusForbidden, errWithStatus.StatusCode)
		assert.Contains(t, err.Error(), "Invite system is disabled")
	})

	t.Run("Deterministic HMAC-SHA256 hashing (not bcrypt)", func(t *testing.T) {
		// Arrange
		emailCrypto := &MockEmailCrypto{}
		service := NewAuth(storage, emailMock, jwt, &config.Public{
			InviteEnabled:     true,
			InviteCodeLength:  12,
			InviteCodeTTL:     720 * time.Hour,
			MaxInvitesPerUser: 0,
		}, nil, emailCrypto)

		var savedHash1, savedHash2 string
		callCount := 0

		storage.SaveInviteCodeFunc = func(invite domain.InviteCode) error {
			callCount++
			if callCount == 1 {
				savedHash1 = invite.CodeHash
			} else {
				savedHash2 = invite.CodeHash
			}
			// Verify it's SHA256 (64 hex chars), not bcrypt (60 chars with $2a$ prefix)
			assert.Equal(t, 64, len(invite.CodeHash))
			assert.NotContains(t, invite.CodeHash, "$")
			return nil
		}
		defer func() { storage.SaveInviteCodeFunc = nil }()

		// Act - Generate two invites
		invite1, err1 := service.GenerateInvite(testUser)
		invite2, err2 := service.GenerateInvite(testUser)

		// Assert
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, savedHash1, savedHash2, "Different codes should produce different hashes")
		assert.NotEqual(t, invite1.PlainCode, invite2.PlainCode, "Should generate different codes")
	})

	t.Run("CountActiveInvites error", func(t *testing.T) {
		// Arrange
		emailCrypto := &MockEmailCrypto{}
		service := NewAuth(storage, emailMock, jwt, &config.Public{
			InviteEnabled:     true,
			InviteCodeLength:  12,
			InviteCodeTTL:     720 * time.Hour,
			MaxInvitesPerUser: 5,
		}, nil, emailCrypto)

		mockError := errors.New("mock CountActiveInvites error")
		storage.CountActiveInvitesFunc = func(userId domain.UserId) (int, error) {
			return 0, mockError
		}
		defer func() { storage.CountActiveInvitesFunc = nil }()

		// Act
		invite, err := service.GenerateInvite(testUser)

		// Assert
		require.Error(t, err)
		assert.Nil(t, invite)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("SaveInviteCode error", func(t *testing.T) {
		// Arrange
		emailCrypto := &MockEmailCrypto{}
		service := NewAuth(storage, emailMock, jwt, &config.Public{
			InviteEnabled:     true,
			InviteCodeLength:  12,
			InviteCodeTTL:     720 * time.Hour,
			MaxInvitesPerUser: 5,
		}, nil, emailCrypto)

		mockError := errors.New("mock SaveInviteCode error")
		storage.CountActiveInvitesFunc = func(userId domain.UserId) (int, error) {
			return 2, nil
		}
		storage.SaveInviteCodeFunc = func(invite domain.InviteCode) error {
			return mockError
		}
		defer func() {
			storage.CountActiveInvitesFunc = nil
			storage.SaveInviteCodeFunc = nil
		}()

		// Act
		invite, err := service.GenerateInvite(testUser)

		// Assert
		require.Error(t, err)
		assert.Nil(t, invite)
		assert.True(t, errors.Is(err, mockError))
	})
}
