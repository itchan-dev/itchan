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
	ThreadsPerPage              int           `yaml:"threads_per_page" validate:"required"`
	MaxThreadCount              *int          `yaml:"max_thread_count"`
	NLastMsg                    int           `yaml:"n_last_msg" validate:"required"` // number of last messages shown in board preview (materialized view)
	BumpLimit                   int           `yaml:"bump_limit" validate:"required"` // if thread have more messages it will not get "bumped"
	BoardPreviewRefreshInterval time.Duration `yaml:"board_preview_refresh_internval" validate:"required"`

	// Validation constants (optional; sensible defaults are used when zero)
	BoardNameMaxLen      int `yaml:"board_name_max_len"`
	BoardShortNameMaxLen int `yaml:"board_short_name_max_len"`
	ThreadTitleMaxLen    int `yaml:"thread_title_max_len"`
	MessageTextMaxLen    int `yaml:"message_text_max_len"`
	MessageTextMinLen    int `yaml:"message_text_min_len"`
	ConfirmationCodeLen  int `yaml:"confirmation_code_len"`
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
	Pg     Pg     `yaml:"pg"`
	Email  Email  `yaml:"email"`
	JwtKey string `yaml:"jwt_key" validate:"required"`
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
}
