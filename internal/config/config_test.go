package config

import (
	"fmt"
	"testing"
	"time"
)

func TestMustLoad(t *testing.T) {
	cfg := MustLoad("./test_data")

	if cfg.Public.Pg.Host != "localhost" {
		t.Errorf("pg.Host, got: %s, want: %s", cfg.Public.Pg.Host, "localhost")
	}
	if cfg.Public.Pg.Port != 5432 {
		t.Errorf("pg.Port, got: %s, want: %s", fmt.Sprint(cfg.Public.Pg.Port), "5432")
	}
	if cfg.Public.Pg.User != "itchan" {
		t.Errorf("pg.User, got: %s, want: %s", cfg.Public.Pg.User, "itchan")
	}
	if cfg.Public.Pg.Password != "pass" {
		t.Errorf("pg.Password, got: %s, want: %s", cfg.Public.Pg.Password, "pass")
	}
	if cfg.Public.Pg.Dbname != "itchan" {
		t.Errorf("pg.Name, got: %s, want: %s", cfg.Public.Pg.Dbname, "itchan")
	}
	if cfg.Public.Pg.InitPath != "path1" {
		t.Errorf("pg.InitPath, got: %s, want: %s", cfg.Public.Pg.InitPath, "path1")
	}
	if cfg.Public.Pg.ThreadsPerPage != 20 {
		t.Errorf("pg.ThreadsPerPage, got: %s, want: %s", fmt.Sprint(cfg.Public.Pg.ThreadsPerPage), "20")
	}
	if cfg.JwtTTL() != time.Duration(100) {
		t.Errorf("JwtTTL, got: %s, want: %s", fmt.Sprint(cfg.JwtTTL()), "100")
	}

	if cfg.JwtKey() != "123" {
		t.Errorf("private jwtkey, got: %s, want: %s", cfg.JwtKey(), "123")
	}
}
