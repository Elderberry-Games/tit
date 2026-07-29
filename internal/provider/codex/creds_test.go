package codex

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCredentialsMissing(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())

	_, err := loadCredentials()
	if err != errCredentialsMissing {
		t.Fatalf("loadCredentials() error = %v, want errCredentialsMissing", err)
	}
}

func TestLoadCredentialsFallsBackToJWTAccount(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	token := fakeJWT(`{"https://api.openai.com/auth":{"chatgpt_account_id":"account-from-jwt","chatgpt_plan_type":"pro"}}`)
	writeAuthFile(t, dir, token, "")

	got, err := loadCredentials()
	if err != nil {
		t.Fatalf("loadCredentials() error = %v", err)
	}
	if got.AccessToken != "REDACTED-TOKEN" {
		t.Errorf("AccessToken = %q", got.AccessToken)
	}
	if got.AccountID != "account-from-jwt" {
		t.Errorf("AccountID = %q, want account-from-jwt", got.AccountID)
	}
	if got.PlanType != "pro" {
		t.Errorf("PlanType = %q, want pro", got.PlanType)
	}
}

func TestLoadCredentialsPrefersStoredAccount(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	token := fakeJWT(`{"https://api.openai.com/auth":{"chatgpt_account_id":"account-from-jwt"}}`)
	writeAuthFile(t, dir, token, "stored-account")

	got, err := loadCredentials()
	if err != nil {
		t.Fatalf("loadCredentials() error = %v", err)
	}
	if got.AccountID != "stored-account" {
		t.Errorf("AccountID = %q, want stored-account", got.AccountID)
	}
}

func TestLoadCredentialsUsesDefaultHomeAndRequiresAccessToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeAuthFile(t, dir, fakeJWT(`{}`), "stored-account")
	if _, err := loadCredentials(); err != nil {
		t.Fatalf("loadCredentials() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{"tokens":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCredentials(); err != errCredentialsMissing {
		t.Fatalf("loadCredentials() error = %v, want errCredentialsMissing", err)
	}
}

func TestDecodeClaimsRejectsMalformedToken(t *testing.T) {
	tests := []string{"not-a-jwt", "header.%%%", "header.e25vdC1qc29ufQ"}
	for _, token := range tests {
		if _, err := decodeClaims(token); err == nil {
			t.Errorf("decodeClaims(%q) unexpectedly succeeded", token)
		}
	}
}

func fakeJWT(payload string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return "REDACTED-HEADER." + encoded + ".REDACTED-SIGNATURE"
}

func writeAuthFile(t *testing.T, dir, idToken, accountID string) {
	t.Helper()
	content := fmt.Sprintf(
		`{"OPENAI_API_KEY":null,"tokens":{"id_token":%q,"access_token":"REDACTED-TOKEN","refresh_token":"REDACTED-REFRESH","account_id":%q}}`,
		idToken,
		accountID,
	)
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
