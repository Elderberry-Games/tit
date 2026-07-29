package claude

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func TestFetchPlanAllWindowsAndHeaders(t *testing.T) {
	body := readFixture(t, "plan_all.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != planUsagePath {
			t.Errorf("request = %s %s, want GET %s", r.Method, r.URL.Path, planUsagePath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer REDACTED-TOKEN" {
			t.Errorf("Authorization header = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "claude-code/2.1.220" {
			t.Errorf("User-Agent = %q", got)
		}
		if got := r.Header.Get("anthropic-beta"); got != "oauth-2025-04-20" {
			t.Errorf("anthropic-beta = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	provider := planTestProvider(t, server.URL)
	result := provider.fetchPlan(context.Background(), fixedNow)

	if result.Status != usage.StatusOK {
		t.Fatalf("Status = %q, want ok; detail = %q", result.Status, result.Detail)
	}
	if len(result.Windows) != 4 {
		t.Fatalf("len(Windows) = %d, want 4", len(result.Windows))
	}
	want := []struct {
		label    string
		duration time.Duration
		percent  float64
	}{
		{"5-hour", 5 * time.Hour, 42},
		{"Weekly", 168 * time.Hour, 18},
		{"Weekly (Opus)", 168 * time.Hour, 7},
		{"Weekly (Sonnet)", 168 * time.Hour, 12},
	}
	for i, expected := range want {
		window := result.Windows[i]
		if window.Label != expected.label ||
			window.Duration != expected.duration ||
			window.UsedPercent != expected.percent {
			t.Errorf("Windows[%d] = %#v, want label=%q duration=%s percent=%v",
				i, window, expected.label, expected.duration, expected.percent)
		}
		if window.Used != nil || window.Limit != nil || window.Unit != "" {
			t.Errorf("Windows[%d] has absolute-count fields: %#v", i, window)
		}
		if window.ResetAt == nil {
			t.Errorf("Windows[%d].ResetAt = nil", i)
		}
	}
}

func TestFetchPlanSkipsNullWindow(t *testing.T) {
	server := fixtureServer(t, http.StatusOK, "plan_null_opus.json")
	defer server.Close()

	result := planTestProvider(t, server.URL).fetchPlan(context.Background(), fixedNow)
	if result.Status != usage.StatusOK {
		t.Fatalf("Status = %q, want ok", result.Status)
	}
	if len(result.Windows) != 3 {
		t.Fatalf("len(Windows) = %d, want 3", len(result.Windows))
	}
	for _, window := range result.Windows {
		if window.Label == "Weekly (Opus)" {
			t.Error("null Opus block emitted a window")
		}
	}
}

func TestFetchPlanHTTPStatuses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantStatus usage.SourceStatus
		wantDetail string
	}{
		{"unauthorized", http.StatusUnauthorized, usage.StatusUnauthorized, usage.MsgUnauthorized("claude")},
		{"forbidden", http.StatusForbidden, usage.StatusUnauthorized, usage.MsgUnauthorized("claude")},
		{"rate limited", http.StatusTooManyRequests, usage.StatusUnavailable, usage.MsgRateLimited()},
		{"server error", http.StatusInternalServerError, usage.StatusUnavailable, usage.MsgUnreachable("HTTP 500")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
			}))
			defer server.Close()

			result := planTestProvider(t, server.URL).fetchPlan(context.Background(), fixedNow)
			assertStatusDetail(t, result, test.wantStatus, test.wantDetail)
		})
	}
}

func TestFetchPlanMalformedJSON(t *testing.T) {
	server := fixtureServer(t, http.StatusOK, "malformed.json")
	defer server.Close()

	result := planTestProvider(t, server.URL).fetchPlan(context.Background(), fixedNow)
	assertStatusDetail(t, result, usage.StatusUnavailable, usage.MsgUnreadable())
}

func TestFetchPlanRejectsTrailingJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{} {}`))
	}))
	defer server.Close()

	result := planTestProvider(t, server.URL).fetchPlan(context.Background(), fixedNow)
	assertStatusDetail(t, result, usage.StatusUnavailable, usage.MsgUnreadable())
}

func TestFetchPlanTransportFailure(t *testing.T) {
	provider := planTestProvider(t, "http://127.0.0.1:1")
	result := provider.fetchPlan(context.Background(), fixedNow)
	assertStatusDetail(t, result, usage.StatusUnavailable, usage.MsgUnreachable("request failed"))
}

func TestFetchPlanInvalidResetIsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":3,"resets_at":"not-a-date"}}`))
	}))
	defer server.Close()

	result := planTestProvider(t, server.URL).fetchPlan(context.Background(), fixedNow)
	if result.Status != usage.StatusOK || len(result.Windows) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if result.Windows[0].ResetAt != nil {
		t.Errorf("ResetAt = %v, want nil", result.Windows[0].ResetAt)
	}
}

func planTestProvider(t *testing.T, baseURL string) *Provider {
	t.Helper()
	isolateCredentials(t)
	t.Setenv(claudeOAuthTokenEnv, "REDACTED-TOKEN")
	provider := New()
	provider.planBaseURL = baseURL
	return provider
}

func fixtureServer(t *testing.T, status int, name string) *httptest.Server {
	t.Helper()
	body := readFixture(t, name)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
