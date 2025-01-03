package pg

import (
	"testing"

	// Assuming this is your internal errors package
	// Assuming this is your domain package
	"github.com/itchan-dev/itchan/backend/internal/errors"
	_ "github.com/lib/pq"
)

func TestSaveUser(t *testing.T) {
	id, err := storage.SaveUser("test@example.com", []byte("password"))
	if err != nil {
		t.Fatalf("SaveUser failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("Expected ID > 0, got %d", id)
	}
	if _, err := storage.SaveUser("test@example.com", []byte("password")); err == nil {
		t.Errorf("Saving user twice should be error")
	}
}

func TestUser(t *testing.T) {
	_, err := storage.SaveUser("testuser@example.com", []byte("password"))
	if err != nil {
		t.Fatal(err)
	}

	user, err := storage.User("testuser@example.com")
	if err != nil {
		t.Fatalf("User retrieval failed: %v", err)
	}
	if user.Email != "testuser@example.com" || string(user.PassHash) != "password" { // Fixed comparison
		t.Errorf("Unexpected user data: %+v", user)
	}

	_, err = storage.User("nonexistent@example.com")
	if err == nil {
		t.Fatal("Expected error for nonexistent user, got nil")
	}
	e, ok := err.(*errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %T", err)
	}
}

func TestDeleteUser(t *testing.T) {
	username := "deleteuser@example.com"
	_, err := storage.SaveUser(username, []byte("password"))
	if err != nil {
		t.Fatal(err)
	}

	if err := storage.DeleteUser(username); err != nil {
		t.Errorf("Cant delete user")
	}
	_, err = storage.User(username)
	if err == nil {
		t.Fatal("Expected error for nonexistent user, got nil")
	}
	e, ok := err.(*errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %T", err)
	}

	err = storage.DeleteUser("nonexistent@example.com")
	if err == nil {
		t.Errorf("Delete  nonexisting user should raise error")
	}
	e, ok = err.(*errors.ErrorWithStatusCode)
	if !ok || e.StatusCode != 404 {
		t.Errorf("Expected ErrorWithStatusCode 404, got %T", err)
	}
}
