package service

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/itchan-dev/itchan/shared/blacklist"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/errors"
	"github.com/itchan-dev/itchan/shared/logger"
	sharedutils "github.com/itchan-dev/itchan/shared/utils"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(creds domain.Credentials) error
	CheckConfirmationCode(email domain.Email, confirmationCode string) error
	Login(creds domain.Credentials) (string, error)

	// Invite system methods
	RegisterWithInvite(inviteCode string, password domain.Password) (string, error)
	GenerateInvite(user domain.User) (*domain.InviteCodeWithPlaintext, error)
	GetUserInvites(userId domain.UserId) ([]domain.InviteCode, error)
	RevokeInvite(userId domain.UserId, codeHash string) error

	// Admin blacklist operations
	BlacklistUser(userId domain.UserId, reason string, blacklistedBy domain.UserId) error
	UnblacklistUser(userId domain.UserId) error
	GetBlacklistedUsersWithDetails() ([]domain.BlacklistEntry, error)
	RefreshBlacklistCache() error
}

type Auth struct {
	storage        AuthStorage
	email          Email
	jwt            Jwt
	cfg            *config.Public
	blacklistCache *blacklist.Cache
	emailCrypto    EmailCrypto
}

type EmailCrypto interface {
	Encrypt(email string) ([]byte, error)
	Hash(email string) []byte
	ExtractDomain(email string) (string, error)
}

type AuthStorage interface {
	SaveUser(user domain.User) (domain.UserId, error)
	User(emailHash []byte) (domain.User, error)
	UpdatePassword(emailHash []byte, newPasswordHash domain.Password) error
	DeleteUser(emailHash []byte) error
	SaveConfirmationData(data domain.ConfirmationData) error
	ConfirmationData(emailHash []byte) (domain.ConfirmationData, error)
	DeleteConfirmationData(emailHash []byte) error

	// Invite code operations
	SaveInviteCode(invite domain.InviteCode) error
	InviteCodeByHash(codeHash string) (domain.InviteCode, error)
	GetInvitesByUser(userId domain.UserId) ([]domain.InviteCode, error)
	CountActiveInvites(userId domain.UserId) (int, error)
	MarkInviteUsed(codeHash string, usedBy domain.UserId) error
	DeleteInviteCode(codeHash string) error
	DeleteInvitesByUser(userId domain.UserId) error

	// Admin blacklist operations
	IsUserBlacklisted(userId domain.UserId) (bool, error)
	BlacklistUser(userId domain.UserId, reason string, blacklistedBy domain.UserId) error
	UnblacklistUser(userId domain.UserId) error
	GetBlacklistedUsersWithDetails() ([]domain.BlacklistEntry, error)
}

type Email interface {
	Send(recipientEmail, subject, body string) error
	IsCorrect(email domain.Email) error
}

type Jwt interface {
	NewToken(user domain.User) (string, error)
}

func NewAuth(storage AuthStorage, email Email, jwt Jwt, cfg *config.Public, blacklistCache *blacklist.Cache, emailCrypto EmailCrypto) *Auth {
	return &Auth{
		storage:        storage,
		email:          email,
		emailCrypto:    emailCrypto,
		jwt:            jwt,
		cfg:            cfg,
		blacklistCache: blacklistCache,
	}
}

func (a *Auth) Register(creds domain.Credentials) error {
	email := strings.ToLower(creds.Email)

	var err error

	err = a.email.IsCorrect(email)
	if err != nil {
		return err
	}

	// Check domain restrictions
	if len(a.cfg.AllowedRegistrationDomains) > 0 {
		emailDomain, err := a.emailCrypto.ExtractDomain(email)
		if err != nil {
			logger.Log.Warn("failed to extract domain during registration",
				"error", err)
			return &errors.ErrorWithStatusCode{
				Message:    "Invalid email format",
				StatusCode: http.StatusBadRequest,
			}
		}

		// Case-insensitive exact domain matching
		allowed := false
		for _, allowedDomain := range a.cfg.AllowedRegistrationDomains {
			if strings.EqualFold(emailDomain, allowedDomain) {
				allowed = true
				break
			}
		}

		if !allowed {
			logger.Log.Info("registration blocked - domain not allowed",
				"domain", emailDomain)
			return &errors.ErrorWithStatusCode{
				Message:    "Registration is restricted to specific email domains",
				StatusCode: http.StatusForbidden,
			}
		}
	}

	emailHash := a.emailCrypto.Hash(email)

	cData, err := a.storage.ConfirmationData(emailHash)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err == nil {
		if cData.Expires.Before(time.Now()) {
			if err := a.storage.DeleteConfirmationData(emailHash); err != nil {
				return err
			}
		} else {
			diff := time.Until(cData.Expires)
			return &errors.ErrorWithStatusCode{Message: fmt.Sprintf("Previous confirmation code is still valid. Retry after %.0fs", diff.Seconds()), StatusCode: http.StatusTooEarly}
		}
	}

	confirmationCode := sharedutils.GenerateConfirmationCode(a.cfg.ConfirmationCodeLen)
	passHash, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error("failed to hash password", "error", err)
		return err
	}
	confirmationCodeHash, err := bcrypt.GenerateFromPassword([]byte(confirmationCode), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error("failed to hash confirmation code", "error", err)
		return err
	}

	emailBody := fmt.Sprintf(`Здравствуйте.

Ваш код подтверждения для входа в Itchan:

%s

Код действителен в течение 15 минут.

Если вы не запрашивали этот код, просто проигнорируйте данное письмо.

---
Это автоматическое уведомление, пожалуйста, не отвечайте на него.`, confirmationCode)

	err = a.email.Send(email, fmt.Sprintf("Код подтверждения: %s (Itchan)", confirmationCode), emailBody)
	if err != nil {
		logger.Log.Error("failed to send confirmation email", "email_hash_prefix", fmt.Sprintf("%x", emailHash[:8]), "error", err)
		return err
	}

	err = a.storage.SaveConfirmationData(domain.ConfirmationData{
		EmailHash:            emailHash,
		PasswordHash:         domain.Password(passHash),
		ConfirmationCodeHash: string(confirmationCodeHash),
		Expires:              time.Now().UTC().Add(a.cfg.ConfirmationCodeTTL),
	})
	if err != nil {
		return err
	}

	logger.Log.Info("confirmation code sent", "email_hash_prefix", fmt.Sprintf("%x", emailHash[:8]), "expires_at", time.Now().UTC().Add(a.cfg.ConfirmationCodeTTL))
	return nil
}

func (a *Auth) CheckConfirmationCode(email domain.Email, confirmationCode string) error {
	email = strings.ToLower(email)

	if err := a.email.IsCorrect(email); err != nil {
		return err
	}

	emailHash := a.emailCrypto.Hash(email)

	data, err := a.storage.ConfirmationData(emailHash)
	if err != nil {
		return err
	}
	if data.Expires.Before(time.Now()) {
		return &errors.ErrorWithStatusCode{Message: "Confirmation time expired", StatusCode: http.StatusBadRequest}
	}
	if err := bcrypt.CompareHashAndPassword([]byte(data.ConfirmationCodeHash), []byte(confirmationCode)); err != nil {
		logger.Log.Warn("failed confirmation code attempt", "email_hash_prefix", fmt.Sprintf("%x", emailHash[:8]), "error", err)
		return &errors.ErrorWithStatusCode{Message: "Wrong confirmation code", StatusCode: http.StatusBadRequest}
	}
	_, err = a.storage.User(emailHash)
	if err != nil {
		e, ok := err.(*errors.ErrorWithStatusCode)
		if ok && e.StatusCode == http.StatusNotFound {
			emailEncrypted, err := a.emailCrypto.Encrypt(email)
			if err != nil {
				return fmt.Errorf("failed to encrypt email: %w", err)
			}
			emailDomain, err := a.emailCrypto.ExtractDomain(email)
			if err != nil {
				return fmt.Errorf("failed to extract email domain: %w", err)
			}

			userId, err := a.storage.SaveUser(domain.User{
				EmailEncrypted: emailEncrypted,
				EmailDomain:    emailDomain,
				EmailHash:      emailHash,
				PassHash:       data.PasswordHash,
				Admin:          false,
			})
			if err != nil {
				return err
			}
			logger.Log.Info("new user registered", "user_id", userId, "domain", emailDomain)
		} else {
			return err
		}
	} else {
		if err := a.storage.UpdatePassword(emailHash, data.PasswordHash); err != nil {
			return err
		}
		logger.Log.Info("password updated", "email_hash_prefix", fmt.Sprintf("%x", emailHash[:8]))
	}
	if err := a.storage.DeleteConfirmationData(emailHash); err != nil {
		return err
	}
	return nil
}

func (a *Auth) Login(creds domain.Credentials) (string, error) {
	email := strings.ToLower(creds.Email)
	password := creds.Password

	err := a.email.IsCorrect(email)
	if err != nil {
		return "", err
	}

	emailHash := a.emailCrypto.Hash(email)

	user, err := a.storage.User(emailHash)
	if err != nil {
		e, ok := err.(*errors.ErrorWithStatusCode)
		if ok && e.StatusCode == http.StatusNotFound {
			return "", &errors.ErrorWithStatusCode{
				Message:    "Invalid credentials",
				StatusCode: http.StatusUnauthorized,
			}
		}
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password))
	if err != nil {
		logger.Log.Warn("failed login attempt - invalid password", "user_id", user.Id)
		return "", &errors.ErrorWithStatusCode{Message: "Invalid credentials", StatusCode: http.StatusUnauthorized}
	}

	isBlacklisted, err := a.storage.IsUserBlacklisted(user.Id)
	if err != nil {
		logger.Log.Error("failed to check blacklist status", "user_id", user.Id, "error", err)
		return "", err
	}
	if isBlacklisted {
		logger.Log.Warn("login attempt by blacklisted user", "user_id", user.Id)
		return "", &errors.ErrorWithStatusCode{
			Message:    "Account suspended",
			StatusCode: http.StatusForbidden,
		}
	}

	token, err := a.jwt.NewToken(user)
	if err != nil {
		logger.Log.Error("failed to create jwt token", "user_id", user.Id, "error", err)
		return "", err
	}

	logger.Log.Info("successful login", "user_id", user.Id, "is_admin", user.Admin)
	return token, nil
}

func (a *Auth) BlacklistUser(userId domain.UserId, reason string, blacklistedBy domain.UserId) error {
	if err := a.storage.BlacklistUser(userId, reason, blacklistedBy); err != nil {
		return err
	}

	// Delete all unused invites created by this user
	if err := a.storage.DeleteInvitesByUser(userId); err != nil {
		logger.Log.Warn("failed to delete user's invites",
			"user_id", userId,
			"error", err)
	}

	if err := a.blacklistCache.Update(); err != nil {
		logger.Log.Warn("user blacklisted but cache update failed",
			"user_id", userId,
			"error", err)
	}

	return nil
}

func (a *Auth) UnblacklistUser(userId domain.UserId) error {
	if err := a.storage.UnblacklistUser(userId); err != nil {
		return err
	}

	if err := a.blacklistCache.Update(); err != nil {
		logger.Log.Warn("user unblacklisted but cache update failed",
			"user_id", userId,
			"error", err)
		// Don't fail the request - cache will update on next background tick
	}

	return nil
}

func (a *Auth) GetBlacklistedUsersWithDetails() ([]domain.BlacklistEntry, error) {
	return a.storage.GetBlacklistedUsersWithDetails()
}

func (a *Auth) RefreshBlacklistCache() error {
	return a.blacklistCache.Update()
}

// =========================================================================
// Invite System Methods
// =========================================================================

// RegisterWithInvite creates a user account using an invite code
// Returns the generated @itchan.ru email address
func (a *Auth) RegisterWithInvite(inviteCode string, password domain.Password) (string, error) {
	// 1. Hash invite code for storage lookup
	inviteCodeHash := sharedutils.HashSHA256(inviteCode)

	// 2. Validate invite code exists and is valid
	invite, err := a.storage.InviteCodeByHash(inviteCodeHash)
	if err != nil {
		if errors.IsNotFound(err) {
			return "", &errors.ErrorWithStatusCode{
				Message:    "Invalid or expired invite code",
				StatusCode: http.StatusBadRequest,
			}
		}
		return "", err
	}

	// 2. Check if already used
	if invite.UsedBy != nil {
		return "", &errors.ErrorWithStatusCode{
			Message:    "Invite code has already been used",
			StatusCode: http.StatusBadRequest,
		}
	}

	// 3. Check expiration
	if invite.ExpiresAt.Before(time.Now()) {
		return "", &errors.ErrorWithStatusCode{
			Message:    "Invite code has expired",
			StatusCode: http.StatusBadRequest,
		}
	}

	// 4. Generate random @itchan.ru email (retry on collision)
	email := sharedutils.GenerateRandomEmail()
	var emailHash []byte
	for range 10 {
		emailHash = a.emailCrypto.Hash(email)
		_, err := a.storage.User(emailHash)
		if errors.IsNotFound(err) {
			break // Email is available
		}
		if err != nil {
			return "", err
		}
		// Collision detected, try again
		email = sharedutils.GenerateRandomEmail()
	}

	// 5. Hash password
	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error("failed to hash password", "error", err)
		return "", err
	}

	// 6. Create user
	// Encrypt email data
	emailEncrypted, err := a.emailCrypto.Encrypt(email)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt email: %w", err)
	}
	emailDomain, err := a.emailCrypto.ExtractDomain(email)
	if err != nil {
		return "", fmt.Errorf("failed to extract email domain: %w", err)
	}
	// emailHash already set from the collision check loop above

	userId, err := a.storage.SaveUser(domain.User{
		EmailEncrypted: emailEncrypted,
		EmailDomain:    emailDomain,
		EmailHash:      emailHash,
		PassHash:       domain.Password(passHash),
		Admin:          false,
	})
	if err != nil {
		return "", err
	}

	// 7. Mark invite as used
	if err := a.storage.MarkInviteUsed(inviteCodeHash, userId); err != nil {
		logger.Log.Error("failed to mark invite used", "user_id", userId, "error", err)
		// Don't fail - user is already created
	}

	logger.Log.Info("user registered via invite",
		"user_id", userId,
		"domain", emailDomain,
		"invited_by", invite.CreatedBy)

	return email, nil
}

// GenerateInvite creates a new invite code for a user
func (a *Auth) GenerateInvite(user domain.User) (*domain.InviteCodeWithPlaintext, error) {
	// 1. Check if invites enabled
	if !a.cfg.InviteEnabled {
		return nil, &errors.ErrorWithStatusCode{
			Message:    "Invite system is disabled",
			StatusCode: http.StatusForbidden,
		}
	}

	// 2. Check account age (skip for admins to allow bootstrapping)
	if !user.Admin {
		accountAge := time.Since(user.CreatedAt)
		if accountAge < a.cfg.MinAccountAgeForInvites {
			// Calculate when the user will be eligible
			eligibleAt := user.CreatedAt.Add(a.cfg.MinAccountAgeForInvites)
			remaining := time.Until(eligibleAt)

			days := int(remaining.Hours() / 24)
			hours := int(remaining.Hours()) % 24
			requiredDays := int(a.cfg.MinAccountAgeForInvites.Hours() / 24)

			return nil, &errors.ErrorWithStatusCode{
				Message:    fmt.Sprintf("Account must be %d days old to generate invites. Wait %d days and %d hours", requiredDays, days, hours),
				StatusCode: http.StatusForbidden,
			}
		}
	}

	// 3. Check invite limit (skip for admins)
	if !user.Admin && a.cfg.MaxInvitesPerUser > 0 {
		activeCount, err := a.storage.CountActiveInvites(user.Id)
		if err != nil {
			return nil, err
		}

		if activeCount >= a.cfg.MaxInvitesPerUser {
			return nil, &errors.ErrorWithStatusCode{
				Message:    fmt.Sprintf("Maximum invite limit reached (%d)", a.cfg.MaxInvitesPerUser),
				StatusCode: http.StatusForbidden,
			}
		}
	}

	// 4. Generate invite code
	plainCode := sharedutils.GenerateConfirmationCode(a.cfg.InviteCodeLength)

	// 5. Hash code
	codeHash := sharedutils.HashSHA256(plainCode)

	invite := domain.InviteCode{
		CodeHash:  codeHash,
		CreatedBy: user.Id,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(a.cfg.InviteCodeTTL),
		UsedBy:    nil,
		UsedAt:    nil,
	}

	// 6. Save to database
	if err := a.storage.SaveInviteCode(invite); err != nil {
		return nil, err
	}

	logger.Log.Info("invite code generated",
		"user_id", user.Id,
		"expires_at", invite.ExpiresAt)

	return &domain.InviteCodeWithPlaintext{
		PlainCode:  plainCode,
		InviteCode: invite,
	}, nil
}

// GetUserInvites returns all invite codes created by a user
func (a *Auth) GetUserInvites(userId domain.UserId) ([]domain.InviteCode, error) {
	return a.storage.GetInvitesByUser(userId)
}

// RevokeInvite deletes an unused invite code
func (a *Auth) RevokeInvite(userId domain.UserId, codeHash string) error {
	// Verify the invite belongs to the user and is unused
	invite, err := a.storage.InviteCodeByHash(codeHash)
	if err != nil {
		return err
	}

	if invite.CreatedBy != userId {
		return &errors.ErrorWithStatusCode{
			Message:    "Unauthorized: invite not owned by user",
			StatusCode: http.StatusForbidden,
		}
	}

	if invite.UsedBy != nil {
		return &errors.ErrorWithStatusCode{
			Message:    "Cannot revoke used invite",
			StatusCode: http.StatusBadRequest,
		}
	}

	return a.storage.DeleteInviteCode(codeHash)
}
