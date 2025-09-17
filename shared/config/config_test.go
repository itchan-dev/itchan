package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMustLoad_RequiredFields(t *testing.T) {
	// Create temp config with a missing required field to ensure validation panics
	dir := t.TempDir()
	public := []byte("threads_per_page: 20\n# n_last_msg is intentionally missing\nbump_limit: 10\njwt_ttl: 1\nboard_preview_refresh_internval: 1\n")
	private := []byte("jwt_key: 'k'\n")
	if err := os.WriteFile(filepath.Join(dir, "public.yaml"), public, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private.yaml"), private, 0o600); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic due to missing required field, got none")
		}
	}()

	_ = MustLoad(dir)
}
