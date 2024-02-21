package config

import (
	"os"
	"path"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Public  Public
	private Private
}

type Public struct {
	Pg     Pg            `yaml:"pg"`
	JwtTTL time.Duration `yaml:"jwt_ttl"`
}

type Pg struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Dbname   string `yaml:"dbname"`
	InitPath string `yaml:"initpath"`
}

type Private struct {
	JwtKey string `yaml:"jwt_key"`
}

// implementing logic.Config interface

func (s *Config) JwtKey() string {
	return s.private.JwtKey
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
