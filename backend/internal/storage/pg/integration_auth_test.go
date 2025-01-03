package pg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Assuming this is your internal errors package
	// Assuming this is your domain package
	"github.com/itchan-dev/itchan/backend/internal/errors"
	_ "github.com/lib/pq"
)

func TestSaveUser(t *testing.T) {
	id, err := storage.SaveUser("test@example.com", []byte("password"))
	require.NoError(t, err, "SaveUser should not return an error")
	assert.Greater(t, id, int64(0), "Expected ID > 0")

	_, err = storage.SaveUser("test@example.com", []byte("password"))
	assert.Error(t, err, "Saving user twice should return an error")
}

func TestUser(t *testing.T) {
	_, err := storage.SaveUser("testuser@example.com", []byte("password"))
	require.NoError(t, err, "SaveUser should not return an error")

	user, err := storage.User("testuser@example.com")
	require.NoError(t, err, "User retrieval should not return an error")
	assert.Equal(t, "testuser@example.com", user.Email, "Unexpected user email")
	assert.Equal(t, "password", string(user.PassHash), "Unexpected user password hash")

	_, err = storage.User("nonexistent@example.com")
	require.Error(t, err, "Expected error for nonexistent user")
	e, ok := err.(*errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")
}

func TestDeleteUser(t *testing.T) {
	username := "deleteuser@example.com"
	_, err := storage.SaveUser(username, []byte("password"))
	require.NoError(t, err, "SaveUser should not return an error")

	err = storage.DeleteUser(username)
	require.NoError(t, err, "DeleteUser should not return an error")

	_, err = storage.User(username)
	require.Error(t, err, "Expected error for deleted user")
	e, ok := err.(*errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")

	err = storage.DeleteUser("nonexistent@example.com")
	require.Error(t, err, "DeleteUser should return an error for nonexistent user")
	e, ok = err.(*errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")
}
