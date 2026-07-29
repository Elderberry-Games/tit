package codex

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func TestAPIKeyWindowsAndDurationResets(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "REDACTED-API-KEY")
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer REDACTED-API-KEY" {
			t.Errorf("Authorization = %q", got)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"model":"gpt-5-nano"`) {
			t.Errorf("body = %s", data)
		}
		w.Header().Set("x-ratelimit-limit-requests", "100")
		w.Header().Set("x-ratelimit-remaining-requests", "75")
		w.Header().Set("x-ratelimit-reset-requests", "1d2h")
		w.Header().Set("x-ratelimit-limit-tokens", "1000")
		w.Header().Set("x-ratelimit-remaining-tokens", "900")
		w.Header().Set("x-ratelimit-reset-tokens", "8.64s")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	provider := New()
	provider.apiBaseURL = server.URL
	got := provider.fetchAPIKey(context.Background(), usage.FixedClock(now))
	if got.Status != usage.StatusOK || len(got.Windows) != 2 {
		t.Fatalf("got %#v", got)
	}
	requests := got.Windows[0]
	if requests.Label != "Requests" || requests.Unit != "requests" {
		t.Errorf("requests metadata = %#v", requests)
	}
	if requests.Limit == nil || *requests.Limit != 100 || requests.Used == nil || *requests.Used != 25 || requests.UsedPercent != 25 {
		t.Errorf("requests counts = %#v", requests)
	}
	if requests.ResetAt == nil || !requests.ResetAt.Equal(now.Add(26*time.Hour)) {
		t.Errorf("requests ResetAt = %v", requests.ResetAt)
	}
	tokens := got.Windows[1]
	if tokens.UsedPercent != 10 {
		t.Errorf("tokens UsedPercent = %v", tokens.UsedPercent)
	}
	if tokens.ResetAt == nil || !tokens.ResetAt.Equal(now.Add(8640*time.Millisecond)) {
		t.Errorf("tokens ResetAt = %v", tokens.ResetAt)
	}
}

func TestParseResetDuration(t *testing.T) {
	tests := map[string]time.Duration{
		"1s":      time.Second,
		"8.64s":   8640 * time.Millisecond,
		"6m0s":    6 * time.Minute,
		"1d2h":    26 * time.Hour,
		"1.5d30m": 36*time.Hour + 30*time.Minute,
	}
	for input, want := range tests {
		got, err := parseResetDuration(input)
		if err != nil {
			t.Errorf("parseResetDuration(%q) error = %v", input, err)
		} else if got != want {
			t.Errorf("parseResetDuration(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestAPIKeyMissingIsUnconfigured(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	got := New().fetchAPIKey(context.Background(), usage.SystemClock)
	if got.Status != usage.StatusUnconfigured {
		t.Fatalf("Status = %q, want unconfigured", got.Status)
	}
}

func TestAPIKeyMissingHeadersIsUnsupported(t *testing.T) {
	got := fetchAPIKeyResponse(t, http.StatusOK, nil)
	if got.Status != usage.StatusUnsupported || got.Detail != usage.MsgNoWindows("this API key") {
		t.Fatalf("got status/detail %q/%q", got.Status, got.Detail)
	}
}

func TestAPIKeyInvalidResetDoesNotFailWindow(t *testing.T) {
	got := fetchAPIKeyResponse(t, http.StatusOK, func(header http.Header) {
		header.Set("x-ratelimit-limit-requests", "10")
		header.Set("x-ratelimit-remaining-requests", "4")
		header.Set("x-ratelimit-reset-requests", "tomorrow")
	})
	if got.Status != usage.StatusOK || len(got.Windows) != 1 {
		t.Fatalf("got %#v", got)
	}
	if got.Windows[0].ResetAt != nil {
		t.Errorf("ResetAt = %v, want nil", got.Windows[0].ResetAt)
	}
}

func TestAPIKeyZeroLimitAndTransportFailure(t *testing.T) {
	header := make(http.Header)
	header.Set("x-ratelimit-limit-requests", "0")
	header.Set("x-ratelimit-remaining-requests", "0")
	windows := appendRateWindow(nil, header, "requests", "Requests", time.Unix(0, 0))
	if len(windows) != 1 || windows[0].UsedPercent != 0 {
		t.Fatalf("windows = %#v", windows)
	}

	t.Setenv("OPENAI_API_KEY", "REDACTED-API-KEY")
	provider := New()
	provider.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("synthetic transport failure")
	})}
	got := provider.fetchAPIKey(context.Background(), usage.SystemClock)
	if got.Status != usage.StatusUnavailable || got.Detail != usage.MsgUnreachable("request failed") {
		t.Fatalf("got status/detail %q/%q", got.Status, got.Detail)
	}
}

func TestAPIKeyHTTPFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantStatus usage.SourceStatus
		wantDetail string
	}{
		{"unauthorized", http.StatusUnauthorized, usage.StatusUnauthorized, usage.MsgUnauthorized("codex")},
		{"rate limited", http.StatusTooManyRequests, usage.StatusUnavailable, usage.MsgRateLimited()},
		{"server error", http.StatusBadGateway, usage.StatusUnavailable, usage.MsgUnreachable("HTTP 502")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fetchAPIKeyResponse(t, tt.statusCode, nil)
			if got.Status != tt.wantStatus || got.Detail != tt.wantDetail {
				t.Fatalf("got status/detail %q/%q, want %q/%q", got.Status, got.Detail, tt.wantStatus, tt.wantDetail)
			}
		})
	}
}

func TestProviderMetadataAndFetchIsolation(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	provider := New()

	if provider.ID() != usage.ProviderCodex || provider.DisplayName() != "Codex" {
		t.Errorf("provider metadata = %q/%q", provider.ID(), provider.DisplayName())
	}
	if provider.MinPollInterval() != time.Minute {
		t.Errorf("MinPollInterval = %s", provider.MinPollInterval())
	}
	results := provider.Fetch(context.Background(), nil)
	if len(results) != 2 || results[0].Source != usage.SourcePlan || results[1].Source != usage.SourceAPIKey {
		t.Fatalf("results = %#v", results)
	}
}

func TestSafeFetchRecovers(t *testing.T) {
	got := safeFetch(usage.SourcePlan, func() usage.SourceResult {
		panic("synthetic panic")
	})
	if got.Source != usage.SourcePlan || got.Status != usage.StatusUnavailable || got.Detail != usage.MsgUnreadable() {
		t.Fatalf("got %#v", got)
	}
}

func fetchAPIKeyResponse(t *testing.T, statusCode int, setHeaders func(http.Header)) usage.SourceResult {
	t.Helper()
	t.Setenv("OPENAI_API_KEY", "REDACTED-API-KEY")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if setHeaders != nil {
			setHeaders(w.Header())
		}
		w.WriteHeader(statusCode)
	}))
	t.Cleanup(server.Close)

	provider := New()
	provider.apiBaseURL = server.URL
	return provider.fetchAPIKey(context.Background(), usage.FixedClock(time.Unix(100, 0)))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
