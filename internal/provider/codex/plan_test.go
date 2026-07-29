package codex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func TestPlanCurrentAndLegacyNormalizeIdentically(t *testing.T) {
	current := fetchPlanFixture(t, "plan_current.json", func(r *http.Request) {
		if got := r.Header.Get("ChatGPT-Account-Id"); got != "account-from-jwt" {
			t.Errorf("ChatGPT-Account-Id = %q, want account-from-jwt", got)
		}
		if got := r.Header.Get("User-Agent"); got != "codex-cli" {
			t.Errorf("User-Agent = %q, want codex-cli", got)
		}
	})
	legacy := fetchPlanFixture(t, "plan_legacy.json", nil)

	if current.Status != usage.StatusOK || legacy.Status != usage.StatusOK {
		t.Fatalf("statuses = %q and %q, want ok", current.Status, legacy.Status)
	}
	if !reflect.DeepEqual(current.Windows, legacy.Windows) {
		t.Fatalf("normalized windows differ:\ncurrent: %#v\nlegacy:  %#v", current.Windows, legacy.Windows)
	}
	if len(current.Windows) != 2 {
		t.Fatalf("window count = %d, want 2", len(current.Windows))
	}
	if current.Windows[0].Label != "5-hour" || current.Windows[0].Duration != 5*time.Hour {
		t.Errorf("primary window = %#v", current.Windows[0])
	}
	if current.Windows[1].Label != "Weekly" || current.Windows[1].Duration != 7*24*time.Hour {
		t.Errorf("secondary window = %#v", current.Windows[1])
	}
	if current.Windows[0].Used != nil || current.Windows[0].Limit != nil || current.Windows[0].Unit != "" {
		t.Errorf("plan window contains absolute counts: %#v", current.Windows[0])
	}
	if current.Windows[0].ResetAt == nil || current.Windows[0].ResetAt.Unix() != 1776098704 {
		t.Errorf("primary ResetAt = %v", current.Windows[0].ResetAt)
	}
	if len(current.Info) != 1 || current.Info[0] != (usage.InfoItem{Label: "Credits", Value: "12.50"}) {
		t.Errorf("Info = %#v", current.Info)
	}
}

func TestPlanMissingAuthIsUnconfigured(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())

	got := New().fetchPlan(context.Background())
	if got.Status != usage.StatusUnconfigured {
		t.Fatalf("Status = %q, want unconfigured", got.Status)
	}
	if got.Detail != usage.MsgUnconfigured("Codex credential", "run `codex` to log in") {
		t.Errorf("Detail = %q", got.Detail)
	}
}

func TestPlanHTTPFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantStatus usage.SourceStatus
		wantDetail string
	}{
		{"unauthorized", http.StatusUnauthorized, usage.StatusUnauthorized, usage.MsgUnauthorized("codex")},
		{"forbidden", http.StatusForbidden, usage.StatusUnauthorized, usage.MsgUnauthorized("codex")},
		{"rate limited", http.StatusTooManyRequests, usage.StatusUnavailable, usage.MsgRateLimited()},
		{"server error", http.StatusInternalServerError, usage.StatusUnavailable, usage.MsgUnreachable("HTTP 500")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fetchPlanResponse(t, tt.statusCode, `{}`, nil)
			if got.Status != tt.wantStatus || got.Detail != tt.wantDetail {
				t.Fatalf("got status/detail %q/%q, want %q/%q", got.Status, got.Detail, tt.wantStatus, tt.wantDetail)
			}
		})
	}
}

func TestPlanMalformedJSONIsUnavailable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "malformed.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := fetchPlanResponse(t, http.StatusOK, string(data), nil)
	if got.Status != usage.StatusUnavailable || got.Detail != usage.MsgUnreadable() {
		t.Fatalf("got status/detail %q/%q", got.Status, got.Detail)
	}
}

func TestPlanUnreadableCredentialFileIsUnavailable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := New().fetchPlan(context.Background())
	if got.Status != usage.StatusUnavailable || got.Detail != usage.MsgUnreadable() {
		t.Fatalf("got status/detail %q/%q", got.Status, got.Detail)
	}
}

func TestPlanTransportFailureIsUnavailable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	writeAuthFile(t, dir, fakeJWT(`{}`), "account")
	provider := New()
	provider.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("synthetic transport failure")
	})}

	got := provider.fetchPlan(context.Background())
	if got.Status != usage.StatusUnavailable || got.Detail != usage.MsgUnreachable("request failed") {
		t.Fatalf("got status/detail %q/%q", got.Status, got.Detail)
	}
}

func TestPlanSkipsNullWindowsAndShowsUnlimitedCredits(t *testing.T) {
	body := `{"rate_limit":{"primary_window":null,"secondary_window":{"used_percent":12}},"credits":{"has_credits":true,"unlimited":true,"balance":"9"}}`
	got := fetchPlanResponse(t, http.StatusOK, body, nil)
	if got.Status != usage.StatusOK || len(got.Windows) != 1 {
		t.Fatalf("got %#v", got)
	}
	if len(got.Info) != 1 || got.Info[0].Value != "unlimited" {
		t.Errorf("Info = %#v", got.Info)
	}
}

func fetchPlanFixture(t *testing.T, fixture string, inspect func(*http.Request)) usage.SourceResult {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	return fetchPlanResponse(t, http.StatusOK, string(data), inspect)
}

func fetchPlanResponse(t *testing.T, statusCode int, body string, inspect func(*http.Request)) usage.SourceResult {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	token := fakeJWT(`{"https://api.openai.com/auth":{"chatgpt_account_id":"account-from-jwt","chatgpt_plan_type":"plus"}}`)
	writeAuthFile(t, dir, token, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if inspect != nil {
			inspect(r)
		}
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	provider := New()
	provider.planBaseURL = server.URL
	return provider.fetchPlan(context.Background())
}
