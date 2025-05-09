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
	user := domain.User{Email: "saveuser@example.com", PassHash: "password", Admin: false}
	id, err := storage.SaveUser(user)
	require.NoError(t, err, "SaveUser should not return an error")
	assert.Greater(t, id, int64(0), "Expected ID > 0")

	// Try saving the same user again (should fail due to unique constraint)
	_, err = storage.SaveUser(user)
	assert.Error(t, err, "Saving user twice should return an error")
}

func TestGetUser(t *testing.T) {
	user := domain.User{Email: "getuser@example.com", PassHash: "password", Admin: false}
	id, err := storage.SaveUser(user)
	require.NoError(t, err, "SaveUser should not return an error")
	user.Id = id // Assign the generated ID back for comparison

	userFromDb, err := storage.User(user.Email)
	require.NoError(t, err, "User retrieval should not return an error")
	assert.Equal(t, user.Id, userFromDb.Id, "Unexpected user ID")
	assert.Equal(t, user.Email, userFromDb.Email, "Unexpected user email")
	assert.Equal(t, user.PassHash, userFromDb.PassHash, "Unexpected user password hash")
	assert.Equal(t, user.Admin, userFromDb.Admin, "Unexpected user admin status")

	_, err = storage.User("nonexistent@example.com")
	requireNotFoundError(t, err)
}

func TestUpdatePassword(t *testing.T) {
	user := domain.User{Email: "updatepassword@example.com", PassHash: "password", Admin: false}
	_, err := storage.SaveUser(user)
	require.NoError(t, err, "SaveUser should not return an error")

	newPassword := "new_password"
	creds := domain.Credentials{Email: user.Email, Password: newPassword}
	err = storage.UpdatePassword(creds)
	require.NoError(t, err, "UpdatePassword should not return an error")

	updatedUser, err := storage.User(user.Email)
	require.NoError(t, err, "Retrieving updated user should not return an error")
	require.Equal(t, newPassword, updatedUser.PassHash, "Password was not updated correctly")

	// Test updating password for non-existent user
	nonExistentCreds := domain.Credentials{Email: "nonexisting@example.com", Password: "new_password"}
	err = storage.UpdatePassword(nonExistentCreds)
	requireNotFoundError(t, err)
}

func TestDeleteUser(t *testing.T) {
	user := domain.User{Email: "deleteuser@example.com", PassHash: "password", Admin: false}
	_, err := storage.SaveUser(user)
	require.NoError(t, err, "SaveUser should not return an error")

	err = storage.DeleteUser(user.Email)
	require.NoError(t, err, "DeleteUser should not return an error")

	// Verify user is deleted
	_, err = storage.User(user.Email)
	requireNotFoundError(t, err)

	// Test deleting non-existent user
	err = storage.DeleteUser("nonexistent@example.com")
	requireNotFoundError(t, err)
}

func TestSaveConfirmationData(t *testing.T) {
	// Use UTC and round to second precision as done in the storage layer query
	now := time.Now().UTC().Round(time.Second)
	data := domain.ConfirmationData{
		Email:                "saveconfirmation@example.com",
		NewPassHash:          "new_password_hash",
		ConfirmationCodeHash: "confirm_hash",
		Expires:              now,
	}
	err := storage.SaveConfirmationData(data)
	require.NoError(t, err, "SaveConfirmationData should not return an error")

	// Verify data was saved correctly
	dataFromPg, err := storage.ConfirmationData(data.Email)
	require.NoError(t, err, "ConfirmationData retrieval should not return an error")
	require.Equal(t, data.Email, dataFromPg.Email)
	require.Equal(t, data.NewPassHash, dataFromPg.NewPassHash)
	require.Equal(t, data.ConfirmationCodeHash, dataFromPg.ConfirmationCodeHash)
	// Compare time values directly after ensuring both are UTC and rounded
	require.True(t, data.Expires.Equal(dataFromPg.Expires), "Expected Expires %v, got %v", data.Expires, dataFromPg.Expires)

	// Try saving the same data again (should fail due to unique constraint on email)
	err = storage.SaveConfirmationData(data)
	assert.Error(t, err, "Saving confirmation data twice for the same email should return an error")

	// Cleanup (or the next test using the same email might fail)
	err = storage.DeleteConfirmationData(data.Email)
	require.NoError(t, err, "Cleanup failed")
}

func TestGetConfirmationData(t *testing.T) {
	now := time.Now().UTC().Round(time.Second)
	data := domain.ConfirmationData{
		Email:                "getconfirmationdata@example.com",
		NewPassHash:          "password",
		ConfirmationCodeHash: "confirm",
		Expires:              now,
	}
	err := storage.SaveConfirmationData(data)
	require.NoError(t, err, "SaveConfirmationData should not return an error")

	dataFromPg, err := storage.ConfirmationData(data.Email)
	require.NoError(t, err, "ConfirmationData retrieval should not return an error")
	// Compare fields individually for clarity and better time comparison
	require.Equal(t, data.Email, dataFromPg.Email)
	require.Equal(t, data.NewPassHash, dataFromPg.NewPassHash)
	require.Equal(t, data.ConfirmationCodeHash, dataFromPg.ConfirmationCodeHash)
	require.True(t, data.Expires.Equal(dataFromPg.Expires), "Expected Expires %v, got %v", data.Expires, dataFromPg.Expires)

	// Test getting non-existent data
	_, err = storage.ConfirmationData("nonexistent@example.com")
	requireNotFoundError(t, err)

	// Cleanup
	err = storage.DeleteConfirmationData(data.Email)
	require.NoError(t, err, "Cleanup failed")
}

func TestDeleteConfirmationData(t *testing.T) {
	now := time.Now().UTC().Round(time.Second)
	data := domain.ConfirmationData{
		Email:                "deleteconfirmationdata@example.com",
		NewPassHash:          "password",
		ConfirmationCodeHash: "confirm",
		Expires:              now,
	}
	err := storage.SaveConfirmationData(data)
	require.NoError(t, err, "SaveConfirmationData should not return an error")

	err = storage.DeleteConfirmationData(data.Email)
	require.NoError(t, err, "DeleteConfirmationData should not return an error")

	// Verify data is deleted
	_, err = storage.ConfirmationData(data.Email)
	requireNotFoundError(t, err)

	// Test deleting non-existent data
	err = storage.DeleteConfirmationData("nonexistent@example.com")
	requireNotFoundError(t, err)
}
