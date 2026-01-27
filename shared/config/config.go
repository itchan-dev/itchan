package config

import (
	"os"
	"path"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Public  Public
	Private Private
}

type Public struct {
	JwtTTL                      time.Duration `yaml:"jwt_ttl" validate:"required"`
	ConfirmationCodeTTL         time.Duration `yaml:"confirmation_code_ttl"` // Time until confirmation code expires
	ThreadsPerPage              int           `yaml:"threads_per_page" validate:"required"`
	MessagesPerThreadPage       int           `yaml:"messages_per_thread_page"` // number of messages per thread page (0 = all)
	MaxThreadCount              *int          `yaml:"max_thread_count"`
	NLastMsg                    int           `yaml:"n_last_msg" validate:"required"` // number of last messages shown in board preview (materialized view)
	BumpLimit                   int           `yaml:"bump_limit" validate:"required"` // if thread have more messages it will not get "bumped"
	BoardPreviewRefreshInterval time.Duration `yaml:"board_preview_refresh_internval" validate:"required"`
	BoardActivityWindow         time.Duration `yaml:"board_activity_window"` // How far back to check for board activity (should be > refresh interval)
	BlacklistCacheInterval      int           `yaml:"blacklist_cache_interval" validate:"required"` // Interval in seconds to refresh blacklist cache

	// Security settings
	SecureCookies bool `yaml:"secure_cookies"` // Enable Secure flag on cookies (requires HTTPS)
	CSRFEnabled   bool `yaml:"csrf_enabled"`   // Enable CSRF protection (default: true)

	// Logging settings
	LogLevel  string `yaml:"log_level"`  // Log level: debug, info, warn, error (default: info)
	LogFormat string `yaml:"log_format"` // Log format: text or json (default: text)

	// Validation constants (optional; sensible defaults are used when zero)
	BoardNameMaxLen      int `yaml:"board_name_max_len"`
	BoardShortNameMaxLen int `yaml:"board_short_name_max_len"`
	ThreadTitleMaxLen    int `yaml:"thread_title_max_len"`
	MessageTextMaxLen    int `yaml:"message_text_max_len"`
	MessageTextMinLen    int `yaml:"message_text_min_len"`
	ConfirmationCodeLen  int `yaml:"confirmation_code_len"`
	PasswordMinLen       int `yaml:"password_min_len"`

	// Attachment validation constants (optional; sensible defaults are used when zero)
	MaxAttachmentsPerMessage int      `yaml:"max_attachments_per_message"`
	MaxAttachmentSizeBytes   int64    `yaml:"max_attachment_size_bytes"`
	MaxTotalAttachmentSize   int64    `yaml:"max_total_attachment_size"`
	AllowedImageMimeTypes    []string `yaml:"allowed_image_mime_types"`
	AllowedVideoMimeTypes    []string `yaml:"allowed_video_mime_types"`

	// Invite system configuration
	InviteEnabled           bool          `yaml:"invite_enabled"`
	InviteCodeLength        int           `yaml:"invite_code_length"`
	InviteCodeTTL           time.Duration `yaml:"invite_code_ttl"`
	MaxInvitesPerUser       int           `yaml:"max_invites_per_user"`
	MinAccountAgeForInvites time.Duration `yaml:"min_account_age_for_invites"`

	// User activity page settings
	UserMessagesPageLimit int `yaml:"user_messages_page_limit"` // Number of messages/replies shown on account page
}

type Pg struct {
	Host     string `yaml:"host" validate:"required"`
	Port     int    `yaml:"port" validate:"required"`
	User     string `yaml:"user" validate:"required"`
	Password string `yaml:"password" validate:"required"`
	Dbname   string `yaml:"dbname" validate:"required"`
}

type Email struct {
	SMTPServer         string `yaml:"smtp_server" validate:"required"`
	SMTPPort           int    `yaml:"smtp_port" validate:"required"`
	Username           string `yaml:"username" validate:"required"`
	Password           string `yaml:"password" validate:"required"`
	SenderName         string `yaml:"sender_name" validate:"required"`
	Timeout            int    `yaml:"timeout"`
	UseTLS             bool   `yaml:"use_tls"`
	InsecureSkipVerify bool   `yaml:"skip_verify"`
}

type Private struct {
	Pg            Pg     `yaml:"pg"`
	Email         Email  `yaml:"email"`
	JwtKey        string `yaml:"jwt_key" validate:"required"`
	EncryptionKey string `yaml:"encryption_key" validate:"required"`
}

// implementing logic.Config interface

func (s *Config) JwtKey() string {
	return s.Private.JwtKey
}

func (s *Config) JwtTTL() time.Duration {
	return s.Public.JwtTTL
}

func mustLoadPath(configPath string, output any) {
	// check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}
	configFile, err := os.ReadFile(configPath)

	if err != nil {
		panic("can't read config file")
	}

	err = yaml.Unmarshal(configFile, output)
	if err != nil {
		panic("can't unmarshal config file")
	}
}

func MustLoad(configFolder string) *Config {
	var public Public
	mustLoadPath(path.Join(configFolder, "public.yaml"), &public)

	var private Private
	mustLoadPath(path.Join(configFolder, "private.yaml"), &private)

	// Apply default values for validation constants if not set
	applyValidationDefaults(&public)

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(public); err != nil {
		panic("public config validation failed: " + err.Error())
	}
	if err := validate.Struct(private); err != nil {
		panic("private config validation failed: " + err.Error())
	}

	return &Config{public, private}
}

// applyValidationDefaults sets default values for validation constants if they are zero
func applyValidationDefaults(public *Public) {
	// Logging defaults
	if public.LogLevel == "" {
		public.LogLevel = "info"
	}
	if public.LogFormat == "" {
		public.LogFormat = "text"
	}

	if public.BoardNameMaxLen == 0 {
		public.BoardNameMaxLen = 10
	}
	if public.BoardShortNameMaxLen == 0 {
		public.BoardShortNameMaxLen = 3
	}
	if public.ThreadTitleMaxLen == 0 {
		public.ThreadTitleMaxLen = 50
	}
	if public.MessageTextMaxLen == 0 {
		public.MessageTextMaxLen = 10000
	}
	if public.MessageTextMinLen == 0 {
		public.MessageTextMinLen = 1
	}
	if public.ConfirmationCodeLen == 0 {
		public.ConfirmationCodeLen = 6
	}
	if public.PasswordMinLen == 0 {
		public.PasswordMinLen = 8
	}
	if public.ConfirmationCodeTTL == 0 {
		public.ConfirmationCodeTTL = 10 * time.Minute
	}

	// Attachment defaults
	if public.MaxAttachmentsPerMessage == 0 {
		public.MaxAttachmentsPerMessage = 4
	}
	if public.MaxAttachmentSizeBytes == 0 {
		public.MaxAttachmentSizeBytes = 10 * 1024 * 1024 // 10MB per file
	}
	if public.MaxTotalAttachmentSize == 0 {
		public.MaxTotalAttachmentSize = 20 * 1024 * 1024 // 20MB total
	}
	if len(public.AllowedImageMimeTypes) == 0 {
		public.AllowedImageMimeTypes = []string{
			"image/jpeg",
			"image/png",
			"image/gif",
		}
	}
	if len(public.AllowedVideoMimeTypes) == 0 {
		public.AllowedVideoMimeTypes = []string{
			"video/mp4",
			"video/webm",
			"video/ogg",
		}
	}

	// Invite system defaults
	if public.InviteEnabled {
		if public.InviteCodeLength == 0 {
			public.InviteCodeLength = 12 // Longer than confirmation codes for security
		}
		if public.InviteCodeTTL == 0 {
			public.InviteCodeTTL = 720 * time.Hour // 30 days
		}
		if public.MaxInvitesPerUser == 0 {
			public.MaxInvitesPerUser = 5
		}
		if public.MinAccountAgeForInvites == 0 {
			public.MinAccountAgeForInvites = 720 * time.Hour // 30 days (1 month)
		}
	}

	// User activity page defaults
	if public.UserMessagesPageLimit == 0 {
		public.UserMessagesPageLimit = 50
	}

	// Thread pagination defaults
	if public.MessagesPerThreadPage == 0 {
		public.MessagesPerThreadPage = 1000
	}

	// Board activity window default (6x the refresh interval for reliability)
	if public.BoardActivityWindow == 0 {
		public.BoardActivityWindow = public.BoardPreviewRefreshInterval * 6
	}

	// CSRF protection default (enabled by default for security)
	if !public.CSRFEnabled {
		public.CSRFEnabled = true
	}
}
