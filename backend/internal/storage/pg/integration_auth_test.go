package pg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
	_ "github.com/lib/pq"
)

func TestSaveUser(t *testing.T) {
	user := domain.User{Email: "saveuser@example.com", PassHash: "password"}
	id, err := storage.SaveUser(&user)
	require.NoError(t, err, "SaveUser should not return an error")
	assert.Greater(t, id, int64(0), "Expected ID > 0")

	_, err = storage.SaveUser(&user)
	assert.Error(t, err, "Saving user twice should return an error")
}

func TestGetUser(t *testing.T) {
	user := domain.User{Email: "getuser@example.com", PassHash: "password"}
	_, err := storage.SaveUser(&user)
	require.NoError(t, err, "SaveUser should not return an error")

	userFromDb, err := storage.User(user.Email)
	require.NoError(t, err, "User retrieval should not return an error")
	assert.Equal(t, user.Email, userFromDb.Email, "Unexpected user email")
	assert.Equal(t, user.PassHash, userFromDb.PassHash, "Unexpected user password hash")

	_, err = storage.User("nonexistent@example.com")
	require.Error(t, err, "Expected error for nonexistent user")
	e, ok := err.(*errors.ErrorWithStatusCode)
	require.True(t, ok, "Expected ErrorWithStatusCode")
	assert.Equal(t, 404, e.StatusCode, "Expected status code 404")
}

func TestDeleteUser(t *testing.T) {
	user := domain.User{Email: "deleteuser@example.com", PassHash: "password"}
	_, err := storage.SaveUser(&user)
	require.NoError(t, err, "SaveUser should not return an error")

	err = storage.DeleteUser(user.Email)
	require.NoError(t, err, "DeleteUser should not return an error")

	_, err = storage.User(user.Email)
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
