package kimi

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

var testNow = usage.FixedClock(time.Date(2026, 1, 6, 12, 0, 0, 0, time.UTC))

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KIMI_CODE_API_KEY", "")
	t.Setenv("KIMI_CODE_HOME", "")
	return home
}

func writeCredentialFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCredentialsFromEnv(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")

	creds, err := loadCredentials(testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessToken != "REDACTED-TOKEN" {
		t.Fatalf("AccessToken = %q, want REDACTED-TOKEN", creds.AccessToken)
	}
}

func TestLoadCredentialsFromKimiCodeHome(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", dir)
	writeCredentialFile(t, filepath.Join(dir, "credentials", "kimi-code.json"),
		`{"access_token":"REDACTED-TOKEN","expires_at":4767691200.443,"token_type":"Bearer"}`)

	creds, err := loadCredentials(testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessToken != "REDACTED-TOKEN" {
		t.Fatalf("AccessToken = %q, want REDACTED-TOKEN", creds.AccessToken)
	}
}

func TestLoadCredentialsFromDefaultPath(t *testing.T) {
	home := isolateHome(t)
	writeCredentialFile(t, filepath.Join(home, ".kimi-code", "credentials", "kimi-code.json"),
		`{"access_token":"REDACTED-TOKEN","expires_at":4767691200}`)

	creds, err := loadCredentials(testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessToken != "REDACTED-TOKEN" {
		t.Fatalf("AccessToken = %q, want REDACTED-TOKEN", creds.AccessToken)
	}
}

func TestLoadCredentialsLegacyPath(t *testing.T) {
	home := isolateHome(t)
	writeCredentialFile(t, filepath.Join(home, ".kimi", "credentials", "kimi-code.json"),
		`{"access_token":"REDACTED-LEGACY-TOKEN","expires_at":4767691200}`)

	creds, err := loadCredentials(testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessToken != "REDACTED-LEGACY-TOKEN" {
		t.Fatalf("AccessToken = %q, want REDACTED-LEGACY-TOKEN", creds.AccessToken)
	}
}

func TestLoadCredentialsMissing(t *testing.T) {
	isolateHome(t)

	_, err := loadCredentials(testNow)
	if !errors.Is(err, errCredentialsMissing) {
		t.Fatalf("err = %v, want errCredentialsMissing", err)
	}
}

func TestLoadCredentialsExpired(t *testing.T) {
	home := isolateHome(t)
	writeCredentialFile(t, filepath.Join(home, ".kimi-code", "credentials", "kimi-code.json"),
		`{"access_token":"REDACTED-TOKEN","expires_at":1752561221.443}`)

	_, err := loadCredentials(testNow)
	if !errors.Is(err, errCredentialsExpired) {
		t.Fatalf("err = %v, want errCredentialsExpired", err)
	}
}

func TestLoadCredentialsMillisecondsExpiry(t *testing.T) {
	home := isolateHome(t)
	writeCredentialFile(t, filepath.Join(home, ".kimi-code", "credentials", "kimi-code.json"),
		`{"access_token":"REDACTED-TOKEN","expires_at":4767691200443}`)

	creds, err := loadCredentials(testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessToken != "REDACTED-TOKEN" {
		t.Fatalf("AccessToken = %q, want REDACTED-TOKEN", creds.AccessToken)
	}
}

func TestLoadCredentialsMalformedFile(t *testing.T) {
	home := isolateHome(t)
	writeCredentialFile(t, filepath.Join(home, ".kimi-code", "credentials", "kimi-code.json"), "{not json")

	_, err := loadCredentials(testNow)
	if err == nil || errors.Is(err, errCredentialsMissing) || errors.Is(err, errCredentialsExpired) {
		t.Fatalf("err = %v, want a JSON decode error", err)
	}
}
