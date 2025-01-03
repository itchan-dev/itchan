package service

import (
	"errors"
	"testing"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
)

// Mock ThreadStorage for testing
type MockThreadStorage struct {
	CreateThreadFunc func(title, board string, msg *domain.Message) (int64, error)
	GetThreadFunc    func(id int64) (*domain.Thread, error)
	DeleteThreadFunc func(board string, id int64) error
}

func (m *MockThreadStorage) CreateThread(title, board string, msg *domain.Message) (int64, error) {
	if m.CreateThreadFunc != nil {
		return m.CreateThreadFunc(title, board, msg)
	}
	return 1, nil
}

func (m *MockThreadStorage) GetThread(id int64) (*domain.Thread, error) {
	if m.GetThreadFunc != nil {
		return m.GetThreadFunc(id)
	}
	return &domain.Thread{Messages: []*domain.Message{&domain.Message{Id: id}}}, nil
}

func (m *MockThreadStorage) DeleteThread(board string, id int64) error {
	if m.DeleteThreadFunc != nil {
		return m.DeleteThreadFunc(board, id)
	}
	return nil
}

// Mock ThreadValidator for testing
type MockThreadValidator struct {
	TitleFunc func(title string) error
}

func (m *MockThreadValidator) Title(title string) error {
	if m.TitleFunc != nil {
		return m.TitleFunc(title)
	}
	return nil
}

func TestThreadCreate(t *testing.T) {
	storage := &MockThreadStorage{}
	validator := &MockThreadValidator{}
	service := NewThread(storage, validator)

	title := "test title"
	board := "test board"
	msg := &domain.Message{}

	// Test successful creation
	storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
		if tt != title || b != board {
			t.Errorf("Unexpected title or board: got %s, %s, expected %s, %s", tt, b, title, board)
		}
		return 1, nil
	}
	_, err := service.Create(title, board, msg)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test validation error
	validator.TitleFunc = func(t string) error {
		return &internal_errors.ErrorWithStatusCode{Message: "Invalid title", StatusCode: 400}
	}
	_, err = service.Create(title, board, msg)
	if err == nil || err.Error() != "Invalid title" {
		t.Errorf("Expected validation error, got %v", err)
	}
}

func TestThreadGet(t *testing.T) {
	storage := &MockThreadStorage{}
	validator := &MockThreadValidator{}
	service := NewThread(storage, validator)

	id := int64(1)

	// Test successful get
	expectedThread := &domain.Thread{Title: "test title", Messages: []*domain.Message{{Id: id}}}
	storage.GetThreadFunc = func(i int64) (*domain.Thread, error) {
		if i != id {
			t.Errorf("Unexpected id: got %d, expected %d", i, id)
		}
		return expectedThread, nil
	}
	thread, err := service.Get(id)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if thread.Id() != expectedThread.Id() || thread.Title != expectedThread.Title {
		t.Errorf("Unexpected thread: got %+v, expected %+v", thread, expectedThread)
	}

	// Test storage error
	var mockError error = errors.New("Mock GetThreadFunc")
	storage.GetThreadFunc = func(i int64) (*domain.Thread, error) { return nil, mockError }
	_, err = service.Get(id)
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Unexpected message: got %+v, expected %+v", err, mockError)
	}
}

func TestThreadDelete(t *testing.T) {
	storage := &MockThreadStorage{}
	validator := &MockThreadValidator{}
	service := NewThread(storage, validator)

	board := "test board"
	id := int64(1)

	// Test successful deletion
	storage.DeleteThreadFunc = func(b string, i int64) error {
		if b != board || i != id {
			t.Errorf("Unexpected board or id: got %s, %d, expected %s, %d", b, i, board, id)
		}
		return nil
	}
	err := service.Delete(board, id)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

}
