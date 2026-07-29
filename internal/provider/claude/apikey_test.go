package claude

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func TestFetchAPIKeyParsesRateLimitHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != countTokensPath {
			t.Errorf("request = %s %s, want POST %s", r.Method, r.URL.Path, countTokensPath)
		}
		if got := r.Header.Get("x-api-key"); got != "REDACTED-API-KEY" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version = %q", got)
		}
		if got := r.Header.Get("content-type"); got != "application/json" {
			t.Errorf("content-type = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"messages"`) {
			t.Errorf("request body = %q", body)
		}

		setRateHeaders(w.Header(), "requests", "100", "25")
		setRateHeaders(w.Header(), "tokens", "1000", "400")
		setRateHeaders(w.Header(), "input-tokens", "600", "300")
		setRateHeaders(w.Header(), "output-tokens", "400", "100")
		_, _ = w.Write([]byte(`{"input_tokens":1}`))
	}))
	defer server.Close()

	result := apiKeyTestProvider(t, server.URL).fetchAPIKey(context.Background())
	if result.Status != usage.StatusOK {
		t.Fatalf("Status = %q, want ok; detail = %q", result.Status, result.Detail)
	}
	if len(result.Windows) != 4 {
		t.Fatalf("len(Windows) = %d, want 4", len(result.Windows))
	}

	want := []struct {
		label   string
		unit    string
		used    float64
		limit   float64
		percent float64
	}{
		{"Requests", "requests", 75, 100, 75},
		{"Tokens", "tokens", 600, 1000, 60},
		{"Input tokens", "tokens", 300, 600, 50},
		{"Output tokens", "tokens", 300, 400, 75},
	}
	for i, expected := range want {
		window := result.Windows[i]
		if window.Label != expected.label || window.Unit != expected.unit {
			t.Errorf("Windows[%d] label/unit = %q/%q", i, window.Label, window.Unit)
		}
		if window.Used == nil || *window.Used != expected.used {
			t.Errorf("Windows[%d].Used = %v, want %v", i, window.Used, expected.used)
		}
		if window.Limit == nil || *window.Limit != expected.limit {
			t.Errorf("Windows[%d].Limit = %v, want %v", i, window.Limit, expected.limit)
		}
		if window.UsedPercent != expected.percent {
			t.Errorf("Windows[%d].UsedPercent = %v, want %v", i, window.UsedPercent, expected.percent)
		}
		if window.ResetAt == nil {
			t.Errorf("Windows[%d].ResetAt = nil", i)
		}
	}
}

func TestFetchAPIKeyMissingHeadersIsUnsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := apiKeyTestProvider(t, server.URL).fetchAPIKey(context.Background())
	assertStatusDetail(t, result, usage.StatusUnsupported, usage.MsgNoWindows("this API key"))
}

func TestFetchAPIKeyUnconfigured(t *testing.T) {
	isolateCredentials(t)
	t.Setenv(anthropicAPIKeyEnv, "")

	result := New().fetchAPIKey(context.Background())
	assertStatusDetail(
		t,
		result,
		usage.StatusUnconfigured,
		usage.MsgUnconfigured("ANTHROPIC_API_KEY", "set it to see API-key rate limits"),
	)
}

func TestFetchAPIKeyHTTPStatuses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantStatus usage.SourceStatus
		wantDetail string
	}{
		{"unauthorized", http.StatusUnauthorized, usage.StatusUnauthorized, usage.MsgUnauthorized("claude")},
		{"forbidden", http.StatusForbidden, usage.StatusUnauthorized, usage.MsgUnauthorized("claude")},
		{"server error", http.StatusInternalServerError, usage.StatusUnavailable, usage.MsgUnreachable("HTTP 500")},
		{"rate limited without headers", http.StatusTooManyRequests, usage.StatusUnsupported, usage.MsgNoWindows("this API key")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
			}))
			defer server.Close()

			result := apiKeyTestProvider(t, server.URL).fetchAPIKey(context.Background())
			assertStatusDetail(t, result, test.wantStatus, test.wantDetail)
		})
	}
}

func TestFetchAPIKeyUsesHeadersOnRateLimitedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		setRateHeaders(w.Header(), "requests", "10", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	result := apiKeyTestProvider(t, server.URL).fetchAPIKey(context.Background())
	if result.Status != usage.StatusOK || len(result.Windows) != 1 {
		t.Fatalf("result = %#v, want one usable window", result)
	}
}

func TestFetchAPIKeyMalformedHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		setRateHeaders(w.Header(), "requests", "not-a-number", "2")
	}))
	defer server.Close()

	result := apiKeyTestProvider(t, server.URL).fetchAPIKey(context.Background())
	assertStatusDetail(t, result, usage.StatusUnavailable, usage.MsgUnreadable())
}

func TestParseRateLimitHeadersRejectsMalformedValues(t *testing.T) {
	tests := []struct {
		name      string
		limit     string
		remaining string
		reset     string
	}{
		{"limit", "invalid", "1", "2026-07-29T17:00:00Z"},
		{"remaining", "10", "invalid", "2026-07-29T17:00:00Z"},
		{"reset", "10", "1", "invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := make(http.Header)
			header.Set("anthropic-ratelimit-requests-limit", test.limit)
			header.Set("anthropic-ratelimit-requests-remaining", test.remaining)
			header.Set("anthropic-ratelimit-requests-reset", test.reset)

			windows, malformed := parseRateLimitHeaders(header)
			if !malformed || windows != nil {
				t.Fatalf("parseRateLimitHeaders() = %#v, %v; want nil, true", windows, malformed)
			}
		})
	}
}

func TestParseRateLimitHeadersSkipsIncompleteSet(t *testing.T) {
	header := make(http.Header)
	header.Set("anthropic-ratelimit-requests-limit", "10")

	windows, malformed := parseRateLimitHeaders(header)
	if malformed || len(windows) != 0 {
		t.Fatalf("parseRateLimitHeaders() = %#v, %v; want no windows", windows, malformed)
	}
}

func TestFetchAPIKeyZeroLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		setRateHeaders(w.Header(), "requests", "0", "0")
	}))
	defer server.Close()

	result := apiKeyTestProvider(t, server.URL).fetchAPIKey(context.Background())
	if result.Status != usage.StatusOK || result.Windows[0].UsedPercent != 0 {
		t.Fatalf("result = %#v, want zero percent", result)
	}
}

func TestFetchAPIKeyTransportFailure(t *testing.T) {
	result := apiKeyTestProvider(t, "http://127.0.0.1:1").fetchAPIKey(context.Background())
	assertStatusDetail(t, result, usage.StatusUnavailable, usage.MsgUnreachable("request failed"))
}

func apiKeyTestProvider(t *testing.T, baseURL string) *Provider {
	t.Helper()
	isolateCredentials(t)
	t.Setenv(anthropicAPIKeyEnv, "REDACTED-API-KEY")
	provider := New()
	provider.apiKeyBaseURL = baseURL
	return provider
}

func setRateHeaders(header http.Header, name, limit, remaining string) {
	prefix := "anthropic-ratelimit-" + name
	header.Set(prefix+"-limit", limit)
	header.Set(prefix+"-remaining", remaining)
	header.Set(prefix+"-reset", "2026-07-29T17:00:00Z")
}
