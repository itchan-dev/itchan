package service

import (
	"errors"
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

// MockBoardStorage mocks the BoardStorage interface.
type MockBoardStorage struct {
	createBoardFunc func(creationData domain.BoardCreationData) error
	getBoardFunc    func(shortName domain.BoardShortName, page int) (domain.Board, error)
	deleteBoardFunc func(shortName domain.BoardShortName) error
	getBoardsFunc   func(user domain.User) ([]domain.Board, error)
}

func (m *MockBoardStorage) CreateBoard(creationData domain.BoardCreationData) error {
	if m.createBoardFunc != nil {
		return m.createBoardFunc(creationData)
	}
	return nil // Default success
}

func (m *MockBoardStorage) GetBoard(shortName domain.BoardShortName, page int) (domain.Board, error) {
	if m.getBoardFunc != nil {
		return m.getBoardFunc(shortName, page)
	}
	// Default success case returns a board with the requested shortName
	return domain.Board{BoardMetadata: domain.BoardMetadata{ShortName: shortName}}, nil
}

func (m *MockBoardStorage) DeleteBoard(shortName domain.BoardShortName) error {
	if m.deleteBoardFunc != nil {
		return m.deleteBoardFunc(shortName)
	}
	return nil // Default success
}

func (m *MockBoardStorage) GetBoardsByUser(user domain.User) ([]domain.Board, error) {
	if m.getBoardsFunc != nil {
		return m.getBoardsFunc(user)
	}
	// Default success returns an empty list
	return []domain.Board{}, nil
}

// MockBoardValidator mocks the BoardValidator interface.
type MockBoardValidator struct {
	nameFunc      func(name domain.BoardName) error
	shortNameFunc func(shortName domain.BoardShortName) error
}

func (m *MockBoardValidator) Name(name domain.BoardName) error {
	if m.nameFunc != nil {
		return m.nameFunc(name)
	}
	return nil // Default valid
}

func (m *MockBoardValidator) ShortName(shortName domain.BoardShortName) error {
	if m.shortNameFunc != nil {
		return m.shortNameFunc(shortName)
	}
	return nil // Default valid
}

// --- Tests ---

func TestBoardCreate(t *testing.T) {
	validName := "Test Board"
	validShortName := "tst"
	validEmails := &domain.Emails{"test@example.com"}
	validCreationData := domain.BoardCreationData{Name: validName, ShortName: validShortName, AllowedEmails: validEmails}

	t.Run("Successful Creation", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockValidator := &MockBoardValidator{}
		mockMediaStorage := &SharedMockMediaStorage{}
		storageCalled := false

		mockStorage.createBoardFunc = func(creationData domain.BoardCreationData) error {
			storageCalled = true
			assert.Equal(t, validCreationData, creationData)
			return nil
		}
		mockValidator.nameFunc = func(name domain.BoardName) error {
			assert.Equal(t, validName, name)
			return nil
		}
		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			assert.Equal(t, validShortName, shortName)
			return nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		err := service.Create(validCreationData)

		// Assert
		require.NoError(t, err)
		assert.True(t, storageCalled, "Storage CreateBoard should be called")
	})

	t.Run("Invalid Name", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{} // Storage should not be called
		mockValidator := &MockBoardValidator{}
		mockMediaStorage := &SharedMockMediaStorage{}
		validationError := errors.New("invalid name format")
		invalidData := domain.BoardCreationData{Name: "Invalid Name!", ShortName: validShortName}

		mockValidator.nameFunc = func(name domain.BoardName) error {
			return validationError
		}
		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			// This should not be called if Name validation fails first
			t.Fatal("ShortName validation should not be called when Name validation fails")
			return nil
		}
		mockStorage.createBoardFunc = func(creationData domain.BoardCreationData) error {
			t.Fatal("Storage CreateBoard should not be called when validation fails")
			return nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		err := service.Create(invalidData)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, validationError))
	})

	t.Run("Invalid Short Name", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{} // Storage should not be called
		mockValidator := &MockBoardValidator{}
		mockMediaStorage := &SharedMockMediaStorage{}
		validationError := errors.New("invalid short name format")
		invalidData := domain.BoardCreationData{Name: validName, ShortName: ""}

		mockValidator.nameFunc = func(name domain.BoardName) error {
			assert.Equal(t, validName, name)
			return nil // Name is valid
		}
		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			return validationError
		}
		mockStorage.createBoardFunc = func(creationData domain.BoardCreationData) error {
			t.Fatal("Storage CreateBoard should not be called when validation fails")
			return nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		err := service.Create(invalidData)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, validationError))
	})

	t.Run("Storage Error", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockValidator := &MockBoardValidator{}
		mockMediaStorage := &SharedMockMediaStorage{}
		storageError := errors.New("database connection failed")
		storageCalled := false

		// Assume validation passes
		mockValidator.nameFunc = func(name domain.BoardName) error { return nil }
		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error { return nil }

		mockStorage.createBoardFunc = func(creationData domain.BoardCreationData) error {
			storageCalled = true
			assert.Equal(t, validCreationData, creationData)
			return storageError
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		err := service.Create(validCreationData)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, storageCalled, "Storage CreateBoard should be called")
	})
}

func TestBoardGet(t *testing.T) {
	validShortName := domain.BoardShortName("test")
	expectedBoard := domain.Board{BoardMetadata: domain.BoardMetadata{ShortName: validShortName}}

	t.Run("Successful Get", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{}
		mockValidator := &MockBoardValidator{}
		storageCalled := false
		requestedPage := 2

		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			assert.Equal(t, validShortName, shortName)
			return nil
		}
		mockStorage.getBoardFunc = func(shortName domain.BoardShortName, page int) (domain.Board, error) {
			storageCalled = true
			assert.Equal(t, validShortName, shortName)
			assert.Equal(t, requestedPage, page) // Page should be passed directly
			return expectedBoard, nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		board, err := service.Get(validShortName, requestedPage)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedBoard, board)
		assert.True(t, storageCalled, "Storage GetBoard should be called")
	})

	t.Run("Invalid Short Name", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{} // Storage should not be called
		mockValidator := &MockBoardValidator{}
		validationError := errors.New("invalid short name format")
		invalidShortName := domain.BoardShortName("")

		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			return validationError
		}
		mockStorage.getBoardFunc = func(shortName domain.BoardShortName, page int) (domain.Board, error) {
			t.Fatal("Storage GetBoard should not be called when validation fails")
			return domain.Board{}, nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		_, err := service.Get(invalidShortName, 1)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, validationError))
	})

	t.Run("Storage Error (Board Not Found)", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{}
		mockValidator := &MockBoardValidator{}
		storageError := errors.New("board not found")
		storageCalled := false
		requestedPage := 1

		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			assert.Equal(t, validShortName, shortName)
			return nil // Assume validation passes
		}
		mockStorage.getBoardFunc = func(shortName domain.BoardShortName, page int) (domain.Board, error) {
			storageCalled = true
			assert.Equal(t, validShortName, shortName)
			assert.Equal(t, requestedPage, page)
			return domain.Board{}, storageError
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		_, err := service.Get(validShortName, requestedPage)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, storageCalled, "Storage GetBoard should be called")
	})

	t.Run("Page Less Than One Corrected", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{}
		mockValidator := &MockBoardValidator{}
		storageCalled := false
		requestedPage := 0 // Service should correct this to 1

		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			assert.Equal(t, validShortName, shortName)
			return nil
		}
		mockStorage.getBoardFunc = func(shortName domain.BoardShortName, page int) (domain.Board, error) {
			storageCalled = true
			assert.Equal(t, validShortName, shortName)
			// IMPORTANT: Service logic corrects page to 1 before calling storage
			assert.Equal(t, 1, page, "Page passed to storage should be corrected to 1")
			return expectedBoard, nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		board, err := service.Get(validShortName, requestedPage)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedBoard, board)
		assert.True(t, storageCalled, "Storage GetBoard should be called")
	})
}

func TestBoardGetAll(t *testing.T) {
	testUser := domain.User{Id: 1}
	expectedBoards := []domain.Board{
		{BoardMetadata: domain.BoardMetadata{ShortName: "b", Name: "Board B"}},
		{BoardMetadata: domain.BoardMetadata{ShortName: "a", Name: "Board A"}},
	}

	t.Run("Successful GetAll", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{}
		mockValidator := &MockBoardValidator{} // Validator not used in GetAll
		storageCalled := false

		mockStorage.getBoardsFunc = func(user domain.User) ([]domain.Board, error) {
			storageCalled = true
			assert.Equal(t, testUser, user)
			return expectedBoards, nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		boards, err := service.GetAll(testUser)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedBoards, boards)
		assert.True(t, storageCalled, "Storage GetBoards should be called")
	})

	t.Run("Storage Error on GetAll", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{}
		mockValidator := &MockBoardValidator{}
		storageError := errors.New("failed to retrieve boards")
		storageCalled := false

		mockStorage.getBoardsFunc = func(user domain.User) ([]domain.Board, error) {
			storageCalled = true
			assert.Equal(t, testUser, user)
			return nil, storageError
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		_, err := service.GetAll(testUser)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, storageCalled, "Storage GetBoards should be called")
	})
}

func TestBoardDelete(t *testing.T) {
	validShortName := domain.BoardShortName("test")

	t.Run("Successful Deletion", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{}
		mockValidator := &MockBoardValidator{}
		storageCalled := false

		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			assert.Equal(t, validShortName, shortName)
			return nil
		}
		mockStorage.deleteBoardFunc = func(shortName domain.BoardShortName) error {
			storageCalled = true
			assert.Equal(t, validShortName, shortName)
			return nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		err := service.Delete(validShortName)

		// Assert
		require.NoError(t, err)
		assert.True(t, storageCalled, "Storage DeleteBoard should be called")
	})

	t.Run("Invalid Short Name", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{} // Storage should not be called
		mockValidator := &MockBoardValidator{}
		validationError := errors.New("invalid short name format")
		invalidShortName := domain.BoardShortName("")

		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			return validationError
		}
		mockStorage.deleteBoardFunc = func(shortName domain.BoardShortName) error {
			t.Fatal("Storage DeleteBoard should not be called when validation fails")
			return nil
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		err := service.Delete(invalidShortName)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, validationError))
	})

	t.Run("Storage Error (Board Not Found)", func(t *testing.T) {
		// Arrange
		mockStorage := &MockBoardStorage{}
		mockMediaStorage := &SharedMockMediaStorage{}
		mockValidator := &MockBoardValidator{}
		storageError := errors.New("board not found")
		storageCalled := false
		nonExistentShortName := domain.BoardShortName("nonex")

		mockValidator.shortNameFunc = func(shortName domain.BoardShortName) error {
			assert.Equal(t, nonExistentShortName, shortName)
			return nil // Assume validation passes
		}
		mockStorage.deleteBoardFunc = func(shortName domain.BoardShortName) error {
			storageCalled = true
			assert.Equal(t, nonExistentShortName, shortName)
			return storageError
		}

		service := NewBoard(mockStorage, mockValidator, mockMediaStorage)

		// Act
		err := service.Delete(nonExistentShortName)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, storageCalled, "Storage DeleteBoard should be called")
	})
}
