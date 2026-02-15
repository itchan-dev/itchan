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

func TestBlacklistUser(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	// Create test users
	adminId := createTestUser(t, tx, "admin@test.com")
	targetUserId := createTestUser(t, tx, "target@test.com")

	t.Run("successfully blacklist user", func(t *testing.T) {
		err := storage.blacklistUser(tx, targetUserId, "Spam violation", adminId)
		require.NoError(t, err)

		// Verify user is blacklisted
		isBlacklisted, err := storage.isUserBlacklisted(tx, targetUserId)
		require.NoError(t, err)
		assert.True(t, isBlacklisted)
	})

	t.Run("prevent admin from blacklisting themselves", func(t *testing.T) {
		err := storage.blacklistUser(tx, adminId, "Test", adminId)
		require.Error(t, err)

		var statusErr *internal_errors.ErrorWithStatusCode
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusBadRequest, statusErr.StatusCode)
		assert.Contains(t, statusErr.Message, "Cannot blacklist yourself")
	})

	t.Run("idempotent blacklist operation", func(t *testing.T) {
		userId := createTestUser(t, tx, "duplicate@test.com")

		// First blacklist
		err := storage.blacklistUser(tx, userId, "Reason 1", adminId)
		require.NoError(t, err)

		// Second blacklist should update, not fail
		err = storage.blacklistUser(tx, userId, "Reason 2", adminId)
		require.NoError(t, err)

		// Verify still blacklisted
		isBlacklisted, err := storage.isUserBlacklisted(tx, userId)
		require.NoError(t, err)
		assert.True(t, isBlacklisted)
	})
}

func TestUnblacklistUser(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	adminId := createTestUser(t, tx, "admin@test.com")
	blacklistedUserId := createTestUser(t, tx, "blacklisted@test.com")

	t.Run("successfully unblacklist user", func(t *testing.T) {
		// First blacklist the user
		err := storage.blacklistUser(tx, blacklistedUserId, "Test", adminId)
		require.NoError(t, err)

		// Verify blacklisted
		isBlacklisted, err := storage.isUserBlacklisted(tx, blacklistedUserId)
		require.NoError(t, err)
		assert.True(t, isBlacklisted)

		// Unblacklist
		err = storage.unblacklistUser(tx, blacklistedUserId)
		require.NoError(t, err)

		// Verify no longer blacklisted
		isBlacklisted, err = storage.isUserBlacklisted(tx, blacklistedUserId)
		require.NoError(t, err)
		assert.False(t, isBlacklisted)
	})

	t.Run("unblacklist non-blacklisted user returns error", func(t *testing.T) {
		nonBlacklistedUserId := createTestUser(t, tx, "normal@test.com")

		err := storage.unblacklistUser(tx, nonBlacklistedUserId)
		require.Error(t, err)

		var statusErr *internal_errors.ErrorWithStatusCode
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusNotFound, statusErr.StatusCode)
		assert.Contains(t, statusErr.Message, "not blacklisted")
	})
}

func TestIsUserBlacklisted(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	adminId := createTestUser(t, tx, "admin@test.com")
	blacklistedUserId := createTestUser(t, tx, "blacklisted@test.com")
	normalUserId := createTestUser(t, tx, "normal@test.com")

	// Blacklist one user
	err := storage.blacklistUser(tx, blacklistedUserId, "Test", adminId)
	require.NoError(t, err)

	t.Run("blacklisted user returns true", func(t *testing.T) {
		isBlacklisted, err := storage.isUserBlacklisted(tx, blacklistedUserId)
		require.NoError(t, err)
		assert.True(t, isBlacklisted)
	})

	t.Run("normal user returns false", func(t *testing.T) {
		isBlacklisted, err := storage.isUserBlacklisted(tx, normalUserId)
		require.NoError(t, err)
		assert.False(t, isBlacklisted)
	})

	t.Run("non-existent user returns false", func(t *testing.T) {
		isBlacklisted, err := storage.isUserBlacklisted(tx, 99999)
		require.NoError(t, err)
		assert.False(t, isBlacklisted)
	})
}

func TestGetRecentlyBlacklistedUsers(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	adminId := createTestUser(t, tx, "admin@test.com")

	// Create users blacklisted at different times
	user1Id := createTestUser(t, tx, "user1@test.com")
	user2Id := createTestUser(t, tx, "user2@test.com")
	user3Id := createTestUser(t, tx, "user3@test.com")

	// Blacklist users
	err := storage.blacklistUser(tx, user1Id, "Test", adminId)
	require.NoError(t, err)
	err = storage.blacklistUser(tx, user2Id, "Test", adminId)
	require.NoError(t, err)
	err = storage.blacklistUser(tx, user3Id, "Test", adminId)
	require.NoError(t, err)

	// Set different blacklist times by directly updating the table
	// (simulating users blacklisted at different times)
	now := time.Now().UTC()
	_, err = tx.Exec(`UPDATE user_blacklist SET blacklisted_at = $1 WHERE user_id = $2`,
		now.Add(-10*time.Hour), user1Id)
	require.NoError(t, err)
	_, err = tx.Exec(`UPDATE user_blacklist SET blacklisted_at = $1 WHERE user_id = $2`,
		now.Add(-5*time.Hour), user2Id)
	require.NoError(t, err)

	t.Run("get users blacklisted within time window", func(t *testing.T) {
		since := time.Now().UTC().Add(-6 * time.Hour)
		users, err := storage.getRecentlyBlacklistedUsers(tx, since)
		require.NoError(t, err)

		// Should include user2 and user3 (blacklisted within 6 hours)
		// Should NOT include user1 (blacklisted 10 hours ago)
		assert.Len(t, users, 2)
		assert.Contains(t, users, user2Id)
		assert.Contains(t, users, user3Id)
		assert.NotContains(t, users, user1Id)
	})

	t.Run("get all blacklisted users with far past time", func(t *testing.T) {
		since := time.Now().UTC().Add(-24 * time.Hour)
		users, err := storage.getRecentlyBlacklistedUsers(tx, since)
		require.NoError(t, err)

		// Should include all three users
		assert.Len(t, users, 3)
		assert.Contains(t, users, user1Id)
		assert.Contains(t, users, user2Id)
		assert.Contains(t, users, user3Id)
	})

	t.Run("get no users with recent time", func(t *testing.T) {
		since := time.Now().UTC().Add(1 * time.Hour) // Future time
		users, err := storage.getRecentlyBlacklistedUsers(tx, since)
		require.NoError(t, err)

		// Should return empty slice
		assert.Len(t, users, 0)
	})
}

func TestGetBlacklistedUsersWithDetails(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	adminId := createTestUser(t, tx, "admin@test.com")
	user1Id := createTestUser(t, tx, "user1@test.com")
	user2Id := createTestUser(t, tx, "user2@test.com")

	// Blacklist users with different reasons
	err := storage.blacklistUser(tx, user1Id, "Spam violation", adminId)
	require.NoError(t, err)
	err = storage.blacklistUser(tx, user2Id, "Harassment", adminId)
	require.NoError(t, err)

	t.Run("get all blacklisted users with details", func(t *testing.T) {
		entries, err := storage.getBlacklistedUsersWithDetails(tx)
		require.NoError(t, err)

		assert.Len(t, entries, 2)

		// Find user1's entry
		var user1Entry *domain.BlacklistEntry
		for i := range entries {
			if entries[i].UserId == user1Id {
				user1Entry = &entries[i]
				break
			}
		}
		require.NotNil(t, user1Entry)
		assert.Equal(t, "Spam violation", user1Entry.Reason)
		assert.Equal(t, adminId, user1Entry.BlacklistedBy)

		// Find user2's entry
		var user2Entry *domain.BlacklistEntry
		for i := range entries {
			if entries[i].UserId == user2Id {
				user2Entry = &entries[i]
				break
			}
		}
		require.NotNil(t, user2Entry)
		assert.Equal(t, "Harassment", user2Entry.Reason)
		assert.Equal(t, adminId, user2Entry.BlacklistedBy)
	})

	t.Run("empty list when no blacklisted users", func(t *testing.T) {
		// Create a new transaction with no blacklisted users
		tx2, rollback2 := beginTx(t)
		defer rollback2()

		entries, err := storage.getBlacklistedUsersWithDetails(tx2)
		require.NoError(t, err)
		assert.Len(t, entries, 0)
	})
}

func TestCascadeDeleteBlacklist(t *testing.T) {
	tx, rollback := beginTx(t)
	defer rollback()

	adminId := createTestUser(t, tx, "admin@test.com")
	userId := createTestUser(t, tx, "user@test.com")

	// Blacklist user
	err := storage.blacklistUser(tx, userId, "Test", adminId)
	require.NoError(t, err)

	// Verify blacklisted
	isBlacklisted, err := storage.isUserBlacklisted(tx, userId)
	require.NoError(t, err)
	assert.True(t, isBlacklisted)

	// Delete the user (should cascade to blacklist)
	// Use the same hash format as createTestUser
	emailHash := []byte("hash_user@test.com")
	err = storage.deleteUser(tx, emailHash)
	require.NoError(t, err)

	// Verify blacklist entry was also deleted (user doesn't exist anymore)
	var count int
	err = tx.QueryRow("SELECT COUNT(*) FROM user_blacklist WHERE user_id = $1", userId).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
