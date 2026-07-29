package claude

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func TestProviderMetadata(t *testing.T) {
	provider := New()
	if provider.ID() != usage.ProviderClaude {
		t.Errorf("ID() = %q, want %q", provider.ID(), usage.ProviderClaude)
	}
	if provider.DisplayName() != "Claude" {
		t.Errorf("DisplayName() = %q, want Claude", provider.DisplayName())
	}
	if provider.MinPollInterval() != 180*time.Second {
		t.Errorf("MinPollInterval() = %s, want 180s", provider.MinPollInterval())
	}
}

func TestFetchReturnsBothSources(t *testing.T) {
	isolateCredentials(t)
	t.Setenv(anthropicAPIKeyEnv, "")

	results := New().Fetch(context.Background(), fixedNow)
	if len(results) != 2 {
		t.Fatalf("len(Fetch()) = %d, want 2", len(results))
	}
	if results[0].Source != usage.SourcePlan || results[1].Source != usage.SourceAPIKey {
		t.Errorf("source order = %q, %q", results[0].Source, results[1].Source)
	}
	if results[0].Status != usage.StatusUnconfigured ||
		results[1].Status != usage.StatusUnconfigured {
		t.Errorf("statuses = %q, %q, want both unconfigured", results[0].Status, results[1].Status)
	}
}

func TestFetchRecoversEachSourcePanic(t *testing.T) {
	t.Setenv(claudeOAuthTokenEnv, "REDACTED-TOKEN")
	t.Setenv(anthropicAPIKeyEnv, "REDACTED-API-KEY")
	provider := New()
	provider.client = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			panic("test transport panic")
		}),
	}

	results := provider.Fetch(context.Background(), fixedNow)
	if len(results) != 2 {
		t.Fatalf("len(Fetch()) = %d, want 2", len(results))
	}
	for i, result := range results {
		if result.Status != usage.StatusUnavailable {
			t.Errorf("results[%d].Status = %q, want unavailable", i, result.Status)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
