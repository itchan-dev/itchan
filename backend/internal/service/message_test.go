package service

import (
	"errors"
	"testing"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
)

// Mock structs
type MockMessageStorage struct {
	CreateMessageFunc func(board string, author *domain.User, text string, attachments *domain.Attachments, thread_id int64) (int64, error)
	GetMessageFunc    func(id int64) (*domain.Message, error)
	DeleteMessageFunc func(board string, id int64) error
}

func (m *MockMessageStorage) CreateMessage(board string, author *domain.User, text string, attachments *domain.Attachments, thread_id int64) (int64, error) {
	if m.CreateMessageFunc != nil {
		return m.CreateMessageFunc(board, author, text, attachments, thread_id)
	}
	return 1, nil
}

func (m *MockMessageStorage) GetMessage(id int64) (*domain.Message, error) {
	if m.GetMessageFunc != nil {
		return m.GetMessageFunc(id)
	}
	return &domain.Message{Id: id}, nil
}

func (m *MockMessageStorage) DeleteMessage(board string, id int64) error {
	if m.DeleteMessageFunc != nil {
		return m.DeleteMessageFunc(board, id)
	}
	return nil
}

type MockMessageValidator struct {
	TextFunc func(text string) error
}

func (m *MockMessageValidator) Text(text string) error {
	if m.TextFunc != nil {
		return m.TextFunc(text)
	}
	return nil
}

func TestMessageCreate(t *testing.T) {
	storage := &MockMessageStorage{}
	validator := &MockMessageValidator{}
	service := NewMessage(storage, validator)

	board := "test_board"
	author := &domain.User{Id: 1}
	text := "test_text"
	attachments := domain.Attachments{}
	thread_id := int64(1)

	// Test successful creation
	createdId, err := service.Create(board, author, text, &attachments, thread_id)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if createdId != 1 {
		t.Errorf("Unexpected id: got %d, expected %d", createdId, 1)
	}

	// Test storage error
	mockError := errors.New("Mock CreateMessageFunc")
	storage.CreateMessageFunc = func(board string, author *domain.User, text string, attachments *domain.Attachments, thread_id int64) (int64, error) {
		return 0, mockError
	}
	_, err = service.Create(board, author, text, &attachments, thread_id)
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Expected %v, got: %v", mockError, err)
	}

	// Test validation error
	validator.TextFunc = func(text string) error {
		return &internal_errors.ErrorWithStatusCode{Message: "Invalid text", StatusCode: 400}
	}
	_, err = service.Create(board, author, text, &attachments, thread_id)
	if err == nil || err.Error() != "Invalid text" {
		t.Errorf("Expected validation error 'Invalid text', got: %v", err)
	}
}
func TestMessageGet(t *testing.T) {
	storage := &MockMessageStorage{}
	validator := &MockMessageValidator{} // Not used in Get, but needed for constructor
	service := NewMessage(storage, validator)

	id := int64(1)

	// Test successful get
	expectedMessage := &domain.Message{Id: id, Text: "test_text"}
	storage.GetMessageFunc = func(i int64) (*domain.Message, error) {
		if i != id {
			t.Errorf("Unexpected id: got %d, expected %d", i, id)
		}
		return expectedMessage, nil
	}

	message, err := service.Get(id)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if message.Id != expectedMessage.Id || message.Text != expectedMessage.Text {
		t.Errorf("Unexpected message: got %+v, expected %+v", message, expectedMessage)
	}

	// Test storage error
	mockError := errors.New("Mock GetMessageFunc")
	storage.GetMessageFunc = func(id int64) (*domain.Message, error) { return nil, mockError }
	_, err = service.Get(id)
	if err == nil || !errors.Is(err, mockError) {
		t.Errorf("Unexpected message: got %+v, expected %+v", err, mockError)
	}
}

func TestMessageDelete(t *testing.T) {
	storage := &MockMessageStorage{}
	validator := &MockMessageValidator{} // Not used in Delete, but needed for constructor
	service := NewMessage(storage, validator)

	board := "test_board"
	id := int64(1)

	// Test successful delete
	storage.DeleteMessageFunc = func(b string, i int64) error {
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
