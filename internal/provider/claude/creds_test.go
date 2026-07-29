package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func TestDiscoverPlanCredentialEnvironmentWins(t *testing.T) {
	isolateCredentials(t)
	t.Setenv(claudeOAuthTokenEnv, "REDACTED-ENV-TOKEN")
	writeCredentialFile(t, `{
		"claudeAiOauth": {
			"accessToken": "REDACTED-FILE-TOKEN",
			"expiresAt": 1
		}
	}`)

	token, result := discoverPlanCredential(fixedNow)
	if result != nil {
		t.Fatalf("discoverPlanCredential() result = %#v, want nil", result)
	}
	if token != "REDACTED-ENV-TOKEN" {
		t.Fatalf("discoverPlanCredential() token = %q, want environment token", token)
	}
}

func TestDiscoverPlanCredentialFile(t *testing.T) {
	isolateCredentials(t)
	writeCredentialFile(t, `{
		"claudeAiOauth": {
			"accessToken": "REDACTED-FILE-TOKEN",
			"refreshToken": "REDACTED-REFRESH-TOKEN",
			"expiresAt": 1790000000000
		}
	}`)

	token, result := discoverPlanCredential(fixedNow)
	if result != nil {
		t.Fatalf("discoverPlanCredential() result = %#v, want nil", result)
	}
	if token != "REDACTED-FILE-TOKEN" {
		t.Fatalf("discoverPlanCredential() token = %q, want file token", token)
	}
}

func TestDiscoverPlanCredentialMissing(t *testing.T) {
	isolateCredentials(t)

	token, result := discoverPlanCredential(fixedNow)
	if token != "" {
		t.Fatal("discoverPlanCredential() returned a token")
	}
	assertStatusDetail(
		t,
		*result,
		usage.StatusUnconfigured,
		usage.MsgUnconfigured(
			"Claude credential",
			"run `claude` to log in, or set CLAUDE_CODE_OAUTH_TOKEN",
		),
	)
}

func TestDiscoverPlanCredentialExpired(t *testing.T) {
	isolateCredentials(t)
	writeCredentialFile(t, `{
		"claudeAiOauth": {
			"accessToken": "REDACTED-TOKEN",
			"expiresAt": 1
		}
	}`)

	token, result := discoverPlanCredential(fixedNow)
	if token != "" {
		t.Fatal("discoverPlanCredential() returned an expired token")
	}
	assertStatusDetail(t, *result, usage.StatusUnauthorized, usage.MsgExpired("claude"))
}

func TestDiscoverPlanCredentialMalformed(t *testing.T) {
	isolateCredentials(t)
	writeCredentialFile(t, `{`)

	token, result := discoverPlanCredential(fixedNow)
	if token != "" {
		t.Fatal("discoverPlanCredential() returned a token")
	}
	assertStatusDetail(t, *result, usage.StatusUnavailable, usage.MsgUnreadable())
}

func TestDiscoverPlanCredentialEmptyFileEntry(t *testing.T) {
	isolateCredentials(t)
	writeCredentialFile(t, `{"claudeAiOauth": {}}`)

	_, result := discoverPlanCredential(fixedNow)
	if result == nil || result.Status != usage.StatusUnconfigured {
		t.Fatalf("discoverPlanCredential() result = %#v, want unconfigured", result)
	}
}

func isolateCredentials(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(claudeOAuthTokenEnv, "")
}

func writeCredentialFile(t *testing.T, contents string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
}

func assertStatusDetail(
	t *testing.T,
	result usage.SourceResult,
	wantStatus usage.SourceStatus,
	wantDetail string,
) {
	t.Helper()
	if result.Status != wantStatus {
		t.Errorf("Status = %q, want %q", result.Status, wantStatus)
	}
	if result.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", result.Detail, wantDetail)
	}
}
