package service

import (
	"errors"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type MockAuthStorage struct {
	SaveUserFunc               func(user *domain.User) (int64, error)
	UserFunc                   func(email string) (*domain.User, error)
	DeleteUserFunc             func(email string) error
	UpdatePasswordFunc         func(email, passHash string) error
	SaveConfirmationDataFunc   func(newPassword *domain.ConfirmationData) error
	ConfirmationDataFunc       func(email string) (*domain.ConfirmationData, error)
	DeleteConfirmationDataFunc func(email string) error
}

func (m *MockAuthStorage) SaveUser(user *domain.User) (int64, error) {
	if m.SaveUserFunc != nil {
		return m.SaveUserFunc(user)
	}
	return 1, nil
}

func (m *MockAuthStorage) User(email string) (*domain.User, error) {
	if m.UserFunc != nil {
		return m.UserFunc(email)
	}
	passHash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	return &domain.User{Id: 1, Email: email, PassHash: string(passHash)}, nil
}

func (m *MockAuthStorage) DeleteUser(email string) error {
	if m.DeleteUserFunc != nil {
		return m.DeleteUserFunc(email)
	}
	return nil
}

func (m *MockAuthStorage) UpdatePassword(email, passHash string) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(email, passHash)
	}
	return nil
}

func (m *MockAuthStorage) SaveConfirmationData(data *domain.ConfirmationData) error {
	if m.SaveConfirmationDataFunc != nil {
		return m.SaveConfirmationDataFunc(data)
	}
	return nil
}

func (m *MockAuthStorage) ConfirmationData(email string) (*domain.ConfirmationData, error) {
	if m.ConfirmationDataFunc != nil {
		return m.ConfirmationDataFunc(email)
	}
	return nil, nil
}

func (m *MockAuthStorage) DeleteConfirmationData(email string) error {
	if m.DeleteConfirmationDataFunc != nil {
		return m.DeleteConfirmationDataFunc(email)
	}
	return nil
}

type MockEmail struct {
	SendFunc      func(recipientEmail, subject, body string) error
	IsCorrectFunc func(email string) error
}

func (m *MockEmail) Send(recipientEmail, subject, body string) error {
	if m.SendFunc != nil {
		return m.SendFunc(recipientEmail, subject, body)
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

func TestRegister(t *testing.T) {
	storage := &MockAuthStorage{}
	email := &MockEmail{}
	jwt := &MockJwt{}
	service := NewAuth(storage, email, jwt)

	t.Run("Successful registration", func(t *testing.T) {
		email.IsCorrectFunc = func(e string) error { return nil }
		email.SendFunc = func(recipientEmail, subject, body string) error { return nil }
		storage.SaveConfirmationDataFunc = func(data *domain.ConfirmationData) error {
			assert.NotEmpty(t, data.Email)
			assert.NotEmpty(t, data.NewPassHash)
			assert.NotEmpty(t, data.ConfirmationCodeHash)
			assert.True(t, data.Expires.After(time.Now()))
			return nil
		}

		err := service.Register("test@example.com", "password")
		require.NoError(t, err)
	})

	t.Run("email.IsCorrect error", func(t *testing.T) {
		mockError := errors.New("Mock IsCorrectFunc")
		email.IsCorrectFunc = func(e string) error { return mockError }

		err := service.Register("invalid_email", "password")
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("email.Send error", func(t *testing.T) {
		email.IsCorrectFunc = func(e string) error { return nil }
		mockError := errors.New("Mock SendFunc")
		email.SendFunc = func(recipientEmail, subject, body string) error { return mockError }

		err := service.Register("test@example.com", "password")
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("storage.SaveConfirmationData error", func(t *testing.T) {
		email.IsCorrectFunc = func(e string) error { return nil }
		email.SendFunc = func(recipientEmail, subject, body string) error { return nil }
		mockError := errors.New("Mock SaveConfirmationDataFunc")
		storage.SaveConfirmationDataFunc = func(data *domain.ConfirmationData) error { return mockError }

		err := service.Register("test@example.com", "password")
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})
}

func TestCheckConfirmationCode(t *testing.T) {
	storage := &MockAuthStorage{}
	email := &MockEmail{}
	jwt := &MockJwt{}
	service := NewAuth(storage, email, jwt)

	t.Run("Successful confirmation", func(t *testing.T) {
		confirmationCode := "123456"
		confirmationCodeHash, _ := bcrypt.GenerateFromPassword([]byte(confirmationCode), bcrypt.DefaultCost)
		passHash := "hashed_password"

		email.IsCorrectFunc = func(e string) error { return nil }
		storage.ConfirmationDataFunc = func(email string) (*domain.ConfirmationData, error) {
			return &domain.ConfirmationData{
				Email:                email,
				NewPassHash:          passHash,
				ConfirmationCodeHash: string(confirmationCodeHash),
				Expires:              time.Now().Add(5 * time.Minute),
			}, nil
		}
		storage.UpdatePasswordFunc = func(email, hash string) error {
			assert.Equal(t, passHash, hash)
			return nil
		}

		err := service.CheckConfirmationCode("test@example.com", confirmationCode)
		require.NoError(t, err)
	})

	t.Run("email.IsCorrect error", func(t *testing.T) {
		mockError := errors.New("Mock IsCorrectFunc")
		email.IsCorrectFunc = func(e string) error { return mockError }

		err := service.CheckConfirmationCode("invalid_email", "123456")
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("storage.ConfirmationData error", func(t *testing.T) {
		email.IsCorrectFunc = func(e string) error { return nil }
		mockError := errors.New("Mock ConfirmationDataFunc")
		storage.ConfirmationDataFunc = func(email string) (*domain.ConfirmationData, error) {
			return nil, mockError
		}

		err := service.CheckConfirmationCode("test@example.com", "123456")
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("Confirmation time expired", func(t *testing.T) {
		email.IsCorrectFunc = func(e string) error { return nil }
		storage.ConfirmationDataFunc = func(email string) (*domain.ConfirmationData, error) {
			return &domain.ConfirmationData{
				Email:   email,
				Expires: time.Now().Add(-5 * time.Minute),
			}, nil
		}

		err := service.CheckConfirmationCode("test@example.com", "123456")
		require.Error(t, err)
		assert.Equal(t, "Confirmation time expired", err.Error())
	})

	t.Run("Wrong confirmation code", func(t *testing.T) {
		confirmationCode := "123456"
		confirmationCodeHash, _ := bcrypt.GenerateFromPassword([]byte("different_code"), bcrypt.DefaultCost)

		email.IsCorrectFunc = func(e string) error { return nil }
		storage.ConfirmationDataFunc = func(email string) (*domain.ConfirmationData, error) {
			return &domain.ConfirmationData{
				Email:                email,
				ConfirmationCodeHash: string(confirmationCodeHash),
				Expires:              time.Now().Add(5 * time.Minute),
			}, nil
		}

		err := service.CheckConfirmationCode("test@example.com", confirmationCode)
		require.Error(t, err)
		assert.Equal(t, "Wrong confirmation code", err.Error())
	})

	t.Run("UpdatePassword error", func(t *testing.T) {
		confirmationCode := "123456"
		confirmationCodeHash, _ := bcrypt.GenerateFromPassword([]byte(confirmationCode), bcrypt.DefaultCost)

		email.IsCorrectFunc = func(e string) error { return nil }
		storage.ConfirmationDataFunc = func(email string) (*domain.ConfirmationData, error) {
			return &domain.ConfirmationData{
				Email:                email,
				ConfirmationCodeHash: string(confirmationCodeHash),
				Expires:              time.Now().Add(5 * time.Minute),
			}, nil
		}
		mockError := errors.New("Mock UpdatePasswordFunc")
		storage.UpdatePasswordFunc = func(email, hash string) error { return mockError }

		err := service.CheckConfirmationCode("test@example.com", confirmationCode)
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})
}

func TestLogin(t *testing.T) {
	storage := &MockAuthStorage{}
	email := &MockEmail{}
	jwt := &MockJwt{}
	service := NewAuth(storage, email, jwt)

	email.IsCorrectFunc = func(e string) error { return nil }

	t.Run("Successful login", func(t *testing.T) {
		token, err := service.Login("test@example.com", "password")
		require.NoError(t, err)
		assert.Equal(t, "test_token", token)
	})

	t.Run("jwt new token error", func(t *testing.T) {
		mockError := errors.New("Mock NewTokenFunc")
		jwt.NewTokenFunc = func(user *domain.User) (string, error) { return "", mockError }
		token, err := service.Login("test@example.com", "password")
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
		assert.Equal(t, "", token)
	})

	t.Run("Incorrect password", func(t *testing.T) {
		incorrectHash := "$2a$10$invalid_hash_here_for_testing"
		storage.UserFunc = func(email string) (*domain.User, error) {
			return &domain.User{Id: 1, Email: email, PassHash: incorrectHash}, nil
		}
		_, err := service.Login("test@example.com", "wrong_password")
		require.Error(t, err)
		assert.Equal(t, "Wrong password", err.Error())
	})

	t.Run("storage.User error", func(t *testing.T) {
		mockError := errors.New("Mock UserFunc")
		storage.UserFunc = func(email string) (*domain.User, error) { return nil, mockError }
		_, err := service.Login("test@example.com", "password")
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("email validation error", func(t *testing.T) {
		mockError := errors.New("Mock IsCorrectFunc")
		email.IsCorrectFunc = func(e string) error { return mockError }
		_, err := service.Login("invalid_email", "password")
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})
}
