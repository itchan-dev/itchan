package service

import (
	"errors"
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
)

// MockBoardStorage mocks the BoardStorage interface.
type MockBoardStorage struct {
	createBoardFunc func(name, shortName string) error
	getBoardFunc    func(shortName string, page int) (*domain.Board, error)
	deleteBoardFunc func(shortName string) error
}

func (m *MockBoardStorage) CreateBoard(name, shortName string) error {
	if m.createBoardFunc != nil {
		return m.createBoardFunc(name, shortName)
	}
	return nil
}

func (m *MockBoardStorage) GetBoard(shortName string, page int) (*domain.Board, error) {
	if m.getBoardFunc != nil {
		return m.getBoardFunc(shortName, page)
	}
	return nil, nil
}

func (m *MockBoardStorage) DeleteBoard(shortName string) error {
	if m.deleteBoardFunc != nil {
		return m.deleteBoardFunc(shortName)
	}
	return nil
}

// MockBoardValidator mocks the BoardValidator interface.
type MockBoardValidator struct {
	nameFunc      func(name string) error
	shortNameFunc func(shortName string) error
}

func (m *MockBoardValidator) Name(name string) error {
	if m.nameFunc != nil {
		return m.nameFunc(name)
	}
	return nil
}

func (m *MockBoardValidator) ShortName(shortName string) error {
	if m.shortNameFunc != nil {
		return m.shortNameFunc(shortName)
	}
	return nil
}

func TestBoardCreate(t *testing.T) {
	testCases := []struct {
		name        string
		nameInput   string
		shortInput  string
		mockError   error
		expectError bool
	}{
		{name: "Successful Creation", nameInput: "test", shortInput: "t", mockError: nil, expectError: false},
		{name: "Invalid Name", nameInput: "", shortInput: "t", mockError: errors.New("invalid name"), expectError: true},
		{name: "Invalid Short Name", nameInput: "test", shortInput: "", mockError: errors.New("invalid short name"), expectError: true},
		{name: "Storage Error", nameInput: "test", shortInput: "t", mockError: errors.New("storage error"), expectError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockStorage := &MockBoardStorage{
				createBoardFunc: func(name, shortName string) error {
					return tc.mockError
				},
			}
			mockValidator := &MockBoardValidator{
				nameFunc: func(name string) error {
					if tc.nameInput == "" {
						return errors.New("invalid name")
					}
					return nil
				},
				shortNameFunc: func(shortName string) error {
					if tc.shortInput == "" {
						return errors.New("invalid short name")
					}
					return nil
				},
			}

			s := NewBoard(mockStorage, mockValidator)
			err := s.Create(tc.nameInput, tc.shortInput)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBoardGet(t *testing.T) {
	mockStorage := &MockBoardStorage{
		getBoardFunc: func(shortName string, page int) (*domain.Board, error) {
			if shortName == "invalid" {
				return nil, errors.New("board not found")
			}
			return &domain.Board{ShortName: shortName}, nil
		},
	}

	mockValidator := &MockBoardValidator{
		shortNameFunc: func(shortName string) error {
			if shortName == "invalid_short_name" {
				return errors.New("invalid short name")
			}
			return nil
		},
	}

	s := NewBoard(mockStorage, mockValidator)

	t.Run("ValidShortName", func(t *testing.T) {
		board, err := s.Get("test", 1)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if board == nil || board.ShortName != "test" {
			t.Fatalf("Expected board 'test', but got: %v", board)

		}
	})

	t.Run("InvalidShortName", func(t *testing.T) {
		_, err := s.Get("invalid_short_name", 1)
		if err == nil {
			t.Errorf("Expected error for invalid short name, but got nil")

		}
	})

	t.Run("BoardNotFound", func(t *testing.T) {

		_, err := s.Get("invalid", 1)
		if err == nil {
			t.Fatalf("Expected 'board not found' error, but got nil")

		}
	})

	t.Run("PageLessThanOne", func(t *testing.T) {
		board, err := s.Get("test", 0) // Simulate page less than 1
		if err != nil {
			t.Fatalf("Unexpected error: %v", err) // Check for unexpected errors

		}
		if board == nil || board.ShortName != "test" {
			t.Fatalf("Expected board 'test', but got: %v", board) // Check correct board

		}

	})

}

func TestBoardDelete(t *testing.T) {
	// Test cases for Delete
	testCases := []struct {
		name        string
		shortName   string
		mockError   error
		expectError bool
	}{
		{name: "Successful Deletion", shortName: "test", mockError: nil, expectError: false},
		{name: "Board Not Found", shortName: "nonexistent", mockError: errors.New("board not found"), expectError: true},
		{name: "Invalid Short Name", shortName: "", mockError: errors.New("invalid short name"), expectError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockStorage := &MockBoardStorage{
				deleteBoardFunc: func(shortName string) error { return tc.mockError },
			}
			mockValidator := &MockBoardValidator{
				shortNameFunc: func(shortName string) error {
					if shortName == "" {
						return errors.New("invalid short name")
					}
					return nil
				},
			}

			s := NewBoard(mockStorage, mockValidator)

			err := s.Delete(tc.shortName)
			if (err != nil) != tc.expectError {
				t.Errorf("Expected error: %v, got: %v", tc.expectError, err)
			}
		})
	}
}
