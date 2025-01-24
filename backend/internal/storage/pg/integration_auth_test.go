package pg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	requireNotFoundError(t, err)
}

func TestUpdatePassword(t *testing.T) {
	user := domain.User{Email: "updatepassword@example.com", PassHash: "password"}
	_, err := storage.SaveUser(&user)
	require.NoError(t, err, "SaveUser should not return an error")

	err = storage.UpdatePassword(user.Email, "new_password")
	require.NoError(t, err, "UpdatePassword should not return an error")

	updatedUser, err := storage.User(user.Email)
	require.NoError(t, err, "SaveUser should not return an error")
	require.Equal(t, updatedUser.PassHash, "new_password")

	err = storage.UpdatePassword("nonexisting@example.com", "new_password")
	requireNotFoundError(t, err)
}

func TestDeleteUser(t *testing.T) {
	user := domain.User{Email: "deleteuser@example.com", PassHash: "password"}
	_, err := storage.SaveUser(&user)
	require.NoError(t, err, "SaveUser should not return an error")

	err = storage.DeleteUser(user.Email)
	require.NoError(t, err, "DeleteUser should not return an error")

	_, err = storage.User(user.Email)
	requireNotFoundError(t, err)

	err = storage.DeleteUser("nonexistent@example.com")
	requireNotFoundError(t, err)
}

func TestSaveConfirmationData(t *testing.T) {
	now := time.Now().UTC().Round(time.Second)
	data := domain.ConfirmationData{Email: "saveuser@example.com", NewPassHash: "password", ConfirmationCodeHash: "cofirm", Expires: now}
	err := storage.SaveConfirmationData(&data)
	require.NoError(t, err, "SaveConfirmationData should not return an error")

	err = storage.SaveConfirmationData(&data)
	assert.Error(t, err, "Saving user twice should return an error")

	dataFromPg, err := storage.ConfirmationData(data.Email)
	require.Equal(t, data, *dataFromPg)
}

func TestGetConfirmationData(t *testing.T) {
	now := time.Now().UTC().Round(time.Second)
	data := domain.ConfirmationData{Email: "getconfirmationdata@example.com", NewPassHash: "password", ConfirmationCodeHash: "cofirm", Expires: now}
	err := storage.SaveConfirmationData(&data)
	require.NoError(t, err, "SaveConfirmationData should not return an error")

	dataFromPg, err := storage.ConfirmationData(data.Email)
	require.NoError(t, err, "ConfirmationData should not return an error")
	require.Equal(t, data, *dataFromPg)

	_, err = storage.ConfirmationData("nonexistent@example.com")
	requireNotFoundError(t, err)
}

func TestConfirmationData(t *testing.T) {
	now := time.Now().UTC().Round(time.Second)
	data := domain.ConfirmationData{Email: "deleteconfirmationdata@example.com", NewPassHash: "password", ConfirmationCodeHash: "cofirm", Expires: now}
	err := storage.SaveConfirmationData(&data)
	require.NoError(t, err, "SaveConfirmationData should not return an error")

	err = storage.DeleteConfirmationData(data.Email)
	require.NoError(t, err, "DeleteConfirmationData should not return an error")

	_, err = storage.ConfirmationData(data.Email)
	requireNotFoundError(t, err)

	err = storage.DeleteConfirmationData("nonexistent@example.com")
	requireNotFoundError(t, err)
}
