package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp .env: %v", err)
	}
	return path
}

func TestLoadDotenvMissingFileIsNotAnError(t *testing.T) {
	if err := loadDotenv(filepath.Join(t.TempDir(), "does-not-exist.env")); err != nil {
		t.Errorf("expected a missing .env file to be a no-op, got err=%v", err)
	}
}

func TestLoadDotenvParsesKeyValuePairs(t *testing.T) {
	path := writeTempEnvFile(t, `
# a comment
KONSENSOMAT_TEST_FOO=bar

KONSENSOMAT_TEST_QUOTED="quoted value"
KONSENSOMAT_TEST_SINGLE='single quoted'
`)
	t.Cleanup(func() {
		os.Unsetenv("KONSENSOMAT_TEST_FOO")
		os.Unsetenv("KONSENSOMAT_TEST_QUOTED")
		os.Unsetenv("KONSENSOMAT_TEST_SINGLE")
	})

	if err := loadDotenv(path); err != nil {
		t.Fatalf("loadDotenv: %v", err)
	}

	if got := os.Getenv("KONSENSOMAT_TEST_FOO"); got != "bar" {
		t.Errorf("KONSENSOMAT_TEST_FOO = %q, want %q", got, "bar")
	}
	if got := os.Getenv("KONSENSOMAT_TEST_QUOTED"); got != "quoted value" {
		t.Errorf("KONSENSOMAT_TEST_QUOTED = %q, want %q", got, "quoted value")
	}
	if got := os.Getenv("KONSENSOMAT_TEST_SINGLE"); got != "single quoted" {
		t.Errorf("KONSENSOMAT_TEST_SINGLE = %q, want %q", got, "single quoted")
	}
}

func TestLoadDotenvDoesNotOverrideExistingEnv(t *testing.T) {
	t.Setenv("KONSENSOMAT_TEST_PRESET", "from-real-env")

	path := writeTempEnvFile(t, "KONSENSOMAT_TEST_PRESET=from-dotenv\n")
	if err := loadDotenv(path); err != nil {
		t.Fatalf("loadDotenv: %v", err)
	}

	if got := os.Getenv("KONSENSOMAT_TEST_PRESET"); got != "from-real-env" {
		t.Errorf("a real environment variable must take precedence over .env, got %q", got)
	}
}

func TestLoadDotenvRejectsMissingEquals(t *testing.T) {
	path := writeTempEnvFile(t, "THIS_LINE_HAS_NO_EQUALS_SIGN\n")
	if err := loadDotenv(path); err == nil {
		t.Error("expected an error for a line without '='")
	}
}

func TestLoadDotenvRejectsEmptyKey(t *testing.T) {
	path := writeTempEnvFile(t, "=value\n")
	if err := loadDotenv(path); err == nil {
		t.Error("expected an error for an empty key")
	}
}
