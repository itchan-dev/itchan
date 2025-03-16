package config

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/itchan-dev/itchan/shared/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Public  Public
	Private Private
}

type Public struct {
	JwtTTL                      time.Duration `yaml:"jwt_ttl"`
	ThreadsPerPage              int           `yaml:"threads_per_page"`
	NLastMsg                    int           `yaml:"n_last_msg"` // number of last messages shown in board preview (materialized view)
	BumpLimit                   int           `yaml:"bump_limit"` // if thread have more messages it will not get "bumped"
	BoardPreviewRefreshInterval time.Duration `yaml:"board_preview_refresh_internval"`
}

type Pg struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Dbname   string `yaml:"dbname"`
}

type Email struct {
	SMTPServer         string `yaml:"smtp_server"`
	SMTPPort           int    `yaml:"smtp_port"`
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	SenderName         string `yaml:"sender_name"`
	Timeout            int    `yaml:"timeout"`
	UseTLS             bool   `yaml:"use_tls"`
	InsecureSkipVerify bool   `yaml:"skip_verify"`
}

type Private struct {
	Pg     Pg
	Email  Email
	JwtKey string `yaml:"jwt_key"`
}

// implementing logic.Config interface

func (s *Config) JwtKey() string {
	return s.Private.JwtKey
}

func (s *Config) JwtTTL() time.Duration {
	return s.Public.JwtTTL
}

func mustLoadPath(configPath string, output interface{}) {
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

	return &Config{public, private}
}

func loadValidate(r io.ReadCloser, body any) error {
	if err := json.NewDecoder(r).Decode(body); err != nil {
		log.Printf(err.Error())
		return &errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400}
	}
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(body); err != nil {
		log.Printf(err.Error())
		return &errors.ErrorWithStatusCode{Message: "Required fields missing", StatusCode: 400}
	}
	return nil
}
