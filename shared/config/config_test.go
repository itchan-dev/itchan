package config

import (
	"fmt"
	"testing"
	"time"
)

func TestMustLoad(t *testing.T) {
	cfg := MustLoad("./test_data")

	if cfg.Public.Pg.Host != "localhost" {
		t.Errorf("Host, got: %s, want: %s", cfg.Public.Pg.Host, "localhost")
	}
	if cfg.Public.Pg.Port != 5432 {
		t.Errorf("Port, got: %s, want: %s", fmt.Sprint(cfg.Public.Pg.Port), "5432")
	}
	if cfg.Public.Pg.User != "itchan" {
		t.Errorf("User, got: %s, want: %s", cfg.Public.Pg.User, "itchan")
	}
	if cfg.Public.Pg.Password != "pass" {
		t.Errorf("Password, got: %s, want: %s", cfg.Public.Pg.Password, "pass")
	}
	if cfg.Public.Pg.Dbname != "itchan" {
		t.Errorf("Name, got: %s, want: %s", cfg.Public.Pg.Dbname, "itchan")
	}
	if cfg.Public.ThreadsPerPage != 20 {
		t.Errorf("ThreadsPerPage, got: %s, want: %s", fmt.Sprint(cfg.Public.ThreadsPerPage), "20")
	}
	if cfg.Public.NLastMsg != 5 {
		t.Errorf("NLastMsg, got: %s, want: %s", fmt.Sprint(cfg.Public.NLastMsg), "5")
	}
	if cfg.Public.BumpLimit != 500 {
		t.Errorf("BumpLimit, got: %s, want: %s", fmt.Sprint(cfg.Public.BumpLimit), "500")
	}
	if cfg.JwtTTL() != time.Duration(100) {
		t.Errorf("JwtTTL, got: %s, want: %s", fmt.Sprint(cfg.JwtTTL()), "100")
	}

	if cfg.JwtKey() != "123" {
		t.Errorf("private jwtkey, got: %s, want: %s", cfg.JwtKey(), "123")
	}
}
