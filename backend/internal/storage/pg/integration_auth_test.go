package pg

import (
	"net/http"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// User CRUD Tests
// =========================================================================

func TestSaveUser(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	t.Run("successfully save user", func(t *testing.T) {
		user := domain.User{
			EmailEncrypted: []byte("encrypted_test@example.com"),
			EmailDomain:    "example.com",
			EmailHash:      []byte("hash_test@example.com"),
			PassHash:       "test_pass_hash",
			Admin:          false,
		}

		userId, err := storage.saveUser(tx, user)
		require.NoError(t, err)
		assert.Greater(t, userId, domain.UserId(0))

		// Verify user was saved correctly
		savedUser, err := storage.user(tx, user.EmailHash)
		require.NoError(t, err)
		assert.Equal(t, userId, savedUser.Id)
		assert.Equal(t, user.EmailEncrypted, savedUser.EmailEncrypted)
		assert.Equal(t, user.EmailDomain, savedUser.EmailDomain)
		assert.Equal(t, user.EmailHash, savedUser.EmailHash)
		assert.Equal(t, user.PassHash, savedUser.PassHash)
		assert.Equal(t, user.Admin, savedUser.Admin)
		assert.NotZero(t, savedUser.CreatedAt)
	})

	t.Run("save admin user", func(t *testing.T) {
		user := domain.User{
			EmailEncrypted: []byte("encrypted_admin@example.com"),
			EmailDomain:    "example.com",
			EmailHash:      []byte("hash_admin@example.com"),
			PassHash:       "admin_pass_hash",
			Admin:          true,
		}

		_, err := storage.saveUser(tx, user)
		require.NoError(t, err)

		savedUser, err := storage.user(tx, user.EmailHash)
		require.NoError(t, err)
		assert.True(t, savedUser.Admin)
	})

	t.Run("duplicate email hash should fail", func(t *testing.T) {
		user := domain.User{
			EmailEncrypted: []byte("encrypted_duplicate@example.com"),
			EmailDomain:    "example.com",
			EmailHash:      []byte("hash_duplicate@example.com"),
			PassHash:       "test_pass_hash",
			Admin:          false,
		}

		// Save first time
		_, err := storage.saveUser(tx, user)
		require.NoError(t, err)

		// Try to save again with same email hash
		_, err = storage.saveUser(tx, user)
		require.Error(t, err)
	})
}

func TestUserByEmailHash(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	user := domain.User{
		EmailEncrypted: []byte("encrypted_find@example.com"),
		EmailDomain:    "example.com",
		EmailHash:      []byte("hash_find@example.com"),
		PassHash:       "test_pass_hash",
		Admin:          false,
	}

	userId, err := storage.saveUser(tx, user)
	require.NoError(t, err)

	t.Run("find existing user", func(t *testing.T) {
		foundUser, err := storage.user(tx, user.EmailHash)
		require.NoError(t, err)
		assert.Equal(t, userId, foundUser.Id)
		assert.Equal(t, user.EmailEncrypted, foundUser.EmailEncrypted)
		assert.Equal(t, user.PassHash, foundUser.PassHash)
	})

	t.Run("user not found returns 404 error", func(t *testing.T) {
		nonExistentHash := []byte("hash_nonexistent@example.com")
		_, err := storage.user(tx, nonExistentHash)
		requireNotFoundError(t, err)
	})
}

func TestUpdatePassword(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	user := domain.User{
		EmailEncrypted: []byte("encrypted_update@example.com"),
		EmailDomain:    "example.com",
		EmailHash:      []byte("hash_update@example.com"),
		PassHash:       "old_pass_hash",
		Admin:          false,
	}

	_, err := storage.saveUser(tx, user)
	require.NoError(t, err)

	t.Run("successfully update password", func(t *testing.T) {
		newPassword := domain.Password("new_pass_hash")
		err := storage.updatePassword(tx, user.EmailHash, newPassword)
		require.NoError(t, err)

		// Verify password was updated
		updatedUser, err := storage.user(tx, user.EmailHash)
		require.NoError(t, err)
		assert.Equal(t, string(newPassword), updatedUser.PassHash)
	})

	t.Run("update password for non-existent user returns 404", func(t *testing.T) {
		nonExistentHash := []byte("hash_nonexistent@example.com")
		err := storage.updatePassword(tx, nonExistentHash, "any_password")
		require.Error(t, err)

		var statusErr *internal_errors.ErrorWithStatusCode
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusNotFound, statusErr.StatusCode)
	})
}

func TestDeleteUser(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	user := domain.User{
		EmailEncrypted: []byte("encrypted_delete@example.com"),
		EmailDomain:    "example.com",
		EmailHash:      []byte("hash_delete@example.com"),
		PassHash:       "test_pass_hash",
		Admin:          false,
	}

	_, err := storage.saveUser(tx, user)
	require.NoError(t, err)

	t.Run("successfully delete user", func(t *testing.T) {
		err := storage.deleteUser(tx, user.EmailHash)
		require.NoError(t, err)

		// Verify user no longer exists
		_, err = storage.user(tx, user.EmailHash)
		requireNotFoundError(t, err)
	})

	t.Run("delete non-existent user returns 404", func(t *testing.T) {
		nonExistentHash := []byte("hash_nonexistent@example.com")
		err := storage.deleteUser(tx, nonExistentHash)
		require.Error(t, err)

		var statusErr *internal_errors.ErrorWithStatusCode
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusNotFound, statusErr.StatusCode)
	})
}

// =========================================================================
// Confirmation Data Tests
// =========================================================================

func TestSaveConfirmationData(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	emailHash := []byte("hash_confirm@example.com")
	data := domain.ConfirmationData{
		EmailHash:            emailHash,
		PasswordHash:         "test_pass_hash",
		ConfirmationCodeHash: "test_code_hash",
		Expires:              time.Now().UTC().Add(10 * time.Minute),
	}

	t.Run("successfully save confirmation data", func(t *testing.T) {
		err := storage.saveConfirmationData(tx, data)
		require.NoError(t, err)

		// Verify data was saved
		saved, err := storage.confirmationData(tx, emailHash)
		require.NoError(t, err)
		assert.Equal(t, data.EmailHash, saved.EmailHash)
		assert.Equal(t, data.PasswordHash, saved.PasswordHash)
		assert.Equal(t, data.ConfirmationCodeHash, saved.ConfirmationCodeHash)
		// Compare times with tolerance
		assert.WithinDuration(t, data.Expires, saved.Expires, time.Second)
	})

	t.Run("duplicate email hash should fail", func(t *testing.T) {
		duplicateHash := []byte("hash_duplicate_confirm@example.com")
		data1 := domain.ConfirmationData{
			EmailHash:            duplicateHash,
			PasswordHash:         "first_pass_hash",
			ConfirmationCodeHash: "first_code_hash",
			Expires:              time.Now().UTC().Add(10 * time.Minute),
		}

		// First save
		err := storage.saveConfirmationData(tx, data1)
		require.NoError(t, err)

		// Try to save again with same email hash
		data2 := domain.ConfirmationData{
			EmailHash:            duplicateHash,
			PasswordHash:         "another_pass_hash",
			ConfirmationCodeHash: "another_code_hash",
			Expires:              time.Now().UTC().Add(10 * time.Minute),
		}
		err = storage.saveConfirmationData(tx, data2)
		require.Error(t, err)
	})
}

func TestConfirmationData(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	emailHash := []byte("hash_find_confirm@example.com")
	data := domain.ConfirmationData{
		EmailHash:            emailHash,
		PasswordHash:         "test_pass_hash",
		ConfirmationCodeHash: "test_code_hash",
		Expires:              time.Now().UTC().Add(10 * time.Minute),
	}

	err := storage.saveConfirmationData(tx, data)
	require.NoError(t, err)

	t.Run("find existing confirmation data", func(t *testing.T) {
		found, err := storage.confirmationData(tx, emailHash)
		require.NoError(t, err)
		assert.Equal(t, data.PasswordHash, found.PasswordHash)
		assert.Equal(t, data.ConfirmationCodeHash, found.ConfirmationCodeHash)
	})

	t.Run("confirmation data not found returns 404", func(t *testing.T) {
		nonExistentHash := []byte("hash_nonexistent_confirm@example.com")
		_, err := storage.confirmationData(tx, nonExistentHash)
		requireNotFoundError(t, err)
	})
}

func TestDeleteConfirmationData(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	emailHash := []byte("hash_delete_confirm@example.com")
	data := domain.ConfirmationData{
		EmailHash:            emailHash,
		PasswordHash:         "test_pass_hash",
		ConfirmationCodeHash: "test_code_hash",
		Expires:              time.Now().UTC().Add(10 * time.Minute),
	}

	err := storage.saveConfirmationData(tx, data)
	require.NoError(t, err)

	t.Run("successfully delete confirmation data", func(t *testing.T) {
		err := storage.deleteConfirmationData(tx, emailHash)
		require.NoError(t, err)

		// Verify data no longer exists
		_, err = storage.confirmationData(tx, emailHash)
		requireNotFoundError(t, err)
	})

	t.Run("delete non-existent confirmation data returns 404", func(t *testing.T) {
		nonExistentHash := []byte("hash_nonexistent_confirm@example.com")
		err := storage.deleteConfirmationData(tx, nonExistentHash)
		require.Error(t, err)

		var statusErr *internal_errors.ErrorWithStatusCode
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusNotFound, statusErr.StatusCode)
	})
}

func TestConfirmationDataIndependence(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	// Create user
	user := domain.User{
		EmailEncrypted: []byte("encrypted_independent@example.com"),
		EmailDomain:    "example.com",
		EmailHash:      []byte("hash_independent@example.com"),
		PassHash:       "test_pass_hash",
		Admin:          false,
	}

	_, err := storage.saveUser(tx, user)
	require.NoError(t, err)

	// Save confirmation data
	data := domain.ConfirmationData{
		EmailHash:            user.EmailHash,
		PasswordHash:         "test_pass_hash",
		ConfirmationCodeHash: "test_code_hash",
		Expires:              time.Now().UTC().Add(10 * time.Minute),
	}
	err = storage.saveConfirmationData(tx, data)
	require.NoError(t, err)

	t.Run("deleting user does not cascade to confirmation data", func(t *testing.T) {
		// Note: confirmation_data table has no FK to users, so it persists independently
		// This is by design - confirmation data can exist before user is created

		// Delete user
		err := storage.deleteUser(tx, user.EmailHash)
		require.NoError(t, err)

		// Verify confirmation data still exists (no cascade)
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM confirmation_data WHERE email_hash = $1", user.EmailHash).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "confirmation_data should persist after user deletion")
	})
}

// =========================================================================
// Invite Code Tests
// =========================================================================

func TestSaveInviteCode(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	creatorId := createTestUser(t, tx, "creator@test.com")

	t.Run("successfully save invite code", func(t *testing.T) {
		invite := domain.InviteCode{
			CodeHash:  "test_hash_123",
			CreatedBy: creatorId,
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			UsedBy:    nil,
			UsedAt:    nil,
		}

		err := storage.saveInviteCode(tx, invite)
		require.NoError(t, err)

		// Verify invite was saved
		saved, err := storage.inviteCodeByHash(tx, invite.CodeHash)
		require.NoError(t, err)
		assert.Equal(t, invite.CodeHash, saved.CodeHash)
		assert.Equal(t, invite.CreatedBy, saved.CreatedBy)
		assert.Nil(t, saved.UsedBy)
		assert.Nil(t, saved.UsedAt)
	})

	t.Run("duplicate code hash should fail", func(t *testing.T) {
		codeHash := "duplicate_hash_456"
		invite := domain.InviteCode{
			CodeHash:  codeHash,
			CreatedBy: creatorId,
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		}

		// Save first time
		err := storage.saveInviteCode(tx, invite)
		require.NoError(t, err)

		// Try to save again with same hash
		err = storage.saveInviteCode(tx, invite)
		require.Error(t, err)
	})
}

func TestInviteCodeByHash(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	creatorId := createTestUser(t, tx, "creator@test.com")
	codeHash := "find_hash_789"

	invite := domain.InviteCode{
		CodeHash:  codeHash,
		CreatedBy: creatorId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	err := storage.saveInviteCode(tx, invite)
	require.NoError(t, err)

	t.Run("find existing invite code", func(t *testing.T) {
		found, err := storage.inviteCodeByHash(tx, codeHash)
		require.NoError(t, err)
		assert.Equal(t, invite.CodeHash, found.CodeHash)
		assert.Equal(t, invite.CreatedBy, found.CreatedBy)
	})

	t.Run("invite code not found returns 404", func(t *testing.T) {
		_, err := storage.inviteCodeByHash(tx, "nonexistent_hash")
		requireNotFoundError(t, err)
	})
}

func TestGetInvitesByUser(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	user1Id := createTestUser(t, tx, "user1@test.com")
	user2Id := createTestUser(t, tx, "user2@test.com")

	// Create invites for user1
	invite1 := domain.InviteCode{
		CodeHash:  "user1_invite_1",
		CreatedBy: user1Id,
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	invite2 := domain.InviteCode{
		CodeHash:  "user1_invite_2",
		CreatedBy: user1Id,
		CreatedAt: time.Now().UTC().Add(-1 * time.Hour),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	// Create invite for user2
	invite3 := domain.InviteCode{
		CodeHash:  "user2_invite_1",
		CreatedBy: user2Id,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	err := storage.saveInviteCode(tx, invite1)
	require.NoError(t, err)
	err = storage.saveInviteCode(tx, invite2)
	require.NoError(t, err)
	err = storage.saveInviteCode(tx, invite3)
	require.NoError(t, err)

	t.Run("get invites for user1", func(t *testing.T) {
		invites, err := storage.getInvitesByUser(tx, user1Id)
		require.NoError(t, err)
		assert.Len(t, invites, 2)
		// Should be ordered by created_at DESC (most recent first)
		assert.Equal(t, "user1_invite_2", invites[0].CodeHash)
		assert.Equal(t, "user1_invite_1", invites[1].CodeHash)
	})

	t.Run("get invites for user2", func(t *testing.T) {
		invites, err := storage.getInvitesByUser(tx, user2Id)
		require.NoError(t, err)
		assert.Len(t, invites, 1)
		assert.Equal(t, "user2_invite_1", invites[0].CodeHash)
	})

	t.Run("get invites for user with no invites", func(t *testing.T) {
		user3Id := createTestUser(t, tx, "user3@test.com")
		invites, err := storage.getInvitesByUser(tx, user3Id)
		require.NoError(t, err)
		assert.Len(t, invites, 0)
	})
}

func TestCountActiveInvites(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	userId := createTestUser(t, tx, "user@test.com")
	usedById := createTestUser(t, tx, "used_by@test.com")

	// Create unused, unexpired invite (active)
	activeInvite := domain.InviteCode{
		CodeHash:  "active_invite",
		CreatedBy: userId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	err := storage.saveInviteCode(tx, activeInvite)
	require.NoError(t, err)

	// Create used invite (not active)
	usedInvite := domain.InviteCode{
		CodeHash:  "used_invite",
		CreatedBy: userId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	err = storage.saveInviteCode(tx, usedInvite)
	require.NoError(t, err)
	err = storage.markInviteUsed(tx, usedInvite.CodeHash, usedById)
	require.NoError(t, err)

	// Create expired invite (not active)
	expiredInvite := domain.InviteCode{
		CodeHash:  "expired_invite",
		CreatedBy: userId,
		CreatedAt: time.Now().UTC().Add(-48 * time.Hour),
		ExpiresAt: time.Now().UTC().Add(-24 * time.Hour),
	}
	err = storage.saveInviteCode(tx, expiredInvite)
	require.NoError(t, err)

	t.Run("count only active invites", func(t *testing.T) {
		count, err := storage.countActiveInvites(tx, userId)
		require.NoError(t, err)
		assert.Equal(t, 1, count) // Only the active invite
	})

	t.Run("count for user with no active invites", func(t *testing.T) {
		user2Id := createTestUser(t, tx, "user2@test.com")
		count, err := storage.countActiveInvites(tx, user2Id)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestMarkInviteUsed(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	creatorId := createTestUser(t, tx, "creator@test.com")
	userId := createTestUser(t, tx, "user@test.com")

	codeHash := "mark_used_hash"
	invite := domain.InviteCode{
		CodeHash:  codeHash,
		CreatedBy: creatorId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	err := storage.saveInviteCode(tx, invite)
	require.NoError(t, err)

	t.Run("successfully mark invite as used", func(t *testing.T) {
		err := storage.markInviteUsed(tx, codeHash, userId)
		require.NoError(t, err)

		// Verify invite was marked as used
		marked, err := storage.inviteCodeByHash(tx, codeHash)
		require.NoError(t, err)
		require.NotNil(t, marked.UsedBy)
		assert.Equal(t, userId, *marked.UsedBy)
		require.NotNil(t, marked.UsedAt)
		assert.WithinDuration(t, time.Now().UTC(), *marked.UsedAt, 5*time.Second)
	})

	t.Run("marking already used invite returns conflict error", func(t *testing.T) {
		// Try to mark the same invite again
		anotherUserId := createTestUser(t, tx, "another@test.com")
		err := storage.markInviteUsed(tx, codeHash, anotherUserId)
		require.Error(t, err)

		var statusErr *internal_errors.ErrorWithStatusCode
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusConflict, statusErr.StatusCode)
	})

	t.Run("marking non-existent invite returns conflict error", func(t *testing.T) {
		err := storage.markInviteUsed(tx, "nonexistent_hash", userId)
		require.Error(t, err)

		var statusErr *internal_errors.ErrorWithStatusCode
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusConflict, statusErr.StatusCode)
	})
}

func TestDeleteInviteCode(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	creatorId := createTestUser(t, tx, "creator@test.com")
	codeHash := "delete_hash"

	invite := domain.InviteCode{
		CodeHash:  codeHash,
		CreatedBy: creatorId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	err := storage.saveInviteCode(tx, invite)
	require.NoError(t, err)

	t.Run("successfully delete invite code", func(t *testing.T) {
		err := storage.deleteInviteCode(tx, codeHash)
		require.NoError(t, err)

		// Verify invite no longer exists
		_, err = storage.inviteCodeByHash(tx, codeHash)
		requireNotFoundError(t, err)
	})

	t.Run("delete non-existent invite returns 404", func(t *testing.T) {
		err := storage.deleteInviteCode(tx, "nonexistent_hash")
		require.Error(t, err)

		var statusErr *internal_errors.ErrorWithStatusCode
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusNotFound, statusErr.StatusCode)
	})
}

func TestDeleteInvitesByUser(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	userId := createTestUser(t, tx, "user@test.com")
	usedById := createTestUser(t, tx, "used_by@test.com")

	// Create unused invites
	unusedInvite1 := domain.InviteCode{
		CodeHash:  "unused1",
		CreatedBy: userId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	unusedInvite2 := domain.InviteCode{
		CodeHash:  "unused2",
		CreatedBy: userId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	// Create used invite
	usedInvite := domain.InviteCode{
		CodeHash:  "used",
		CreatedBy: userId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	err := storage.saveInviteCode(tx, unusedInvite1)
	require.NoError(t, err)
	err = storage.saveInviteCode(tx, unusedInvite2)
	require.NoError(t, err)
	err = storage.saveInviteCode(tx, usedInvite)
	require.NoError(t, err)
	err = storage.markInviteUsed(tx, usedInvite.CodeHash, usedById)
	require.NoError(t, err)

	t.Run("delete only unused invites for user", func(t *testing.T) {
		err := storage.deleteInvitesByUser(tx, userId)
		require.NoError(t, err)

		// Verify unused invites were deleted
		_, err = storage.inviteCodeByHash(tx, unusedInvite1.CodeHash)
		requireNotFoundError(t, err)
		_, err = storage.inviteCodeByHash(tx, unusedInvite2.CodeHash)
		requireNotFoundError(t, err)

		// Verify used invite still exists
		found, err := storage.inviteCodeByHash(tx, usedInvite.CodeHash)
		require.NoError(t, err)
		assert.Equal(t, usedInvite.CodeHash, found.CodeHash)
	})

	t.Run("delete invites for user with no unused invites", func(t *testing.T) {
		user2Id := createTestUser(t, tx, "user2@test.com")
		err := storage.deleteInvitesByUser(tx, user2Id)
		require.NoError(t, err) // Should not error
	})
}

func TestInviteCodeCascadeDelete(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	// Create creator user
	creator := domain.User{
		EmailEncrypted: []byte("encrypted_creator@example.com"),
		EmailDomain:    "example.com",
		EmailHash:      []byte("hash_creator@example.com"),
		PassHash:       "test_pass_hash",
		Admin:          false,
	}

	creatorId, err := storage.saveUser(tx, creator)
	require.NoError(t, err)

	// Create invite codes
	invite := domain.InviteCode{
		CodeHash:  "cascade_test_hash",
		CreatedBy: creatorId,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	err = storage.saveInviteCode(tx, invite)
	require.NoError(t, err)

	t.Run("deleting creator cascades to invite codes", func(t *testing.T) {
		// Delete creator user
		err := storage.deleteUser(tx, creator.EmailHash)
		require.NoError(t, err)

		// Verify invite code was also deleted
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM invite_codes WHERE created_by = $1", creatorId).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
