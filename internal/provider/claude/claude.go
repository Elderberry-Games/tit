package claude

import (
	"context"
	"net/http"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

const (
	defaultPlanBaseURL   = "https://api.anthropic.com"
	defaultAPIKeyBaseURL = "https://api.anthropic.com"
)

// Provider reads Claude subscription and API-key usage limits.
type Provider struct {
	client        *http.Client
	planBaseURL   string
	apiKeyBaseURL string
}

// New constructs a Claude usage provider.
func New() *Provider {
	return &Provider{
		client:        http.DefaultClient,
		planBaseURL:   defaultPlanBaseURL,
		apiKeyBaseURL: defaultAPIKeyBaseURL,
	}
}

func (p *Provider) ID() usage.ProviderID { return usage.ProviderClaude }

func (p *Provider) DisplayName() string { return "Claude" }

func (p *Provider) MinPollInterval() time.Duration { return 180 * time.Second }

// Fetch returns each source independently so a failure in one cannot hide the
// other source.
func (p *Provider) Fetch(ctx context.Context, now usage.Clock) []usage.SourceResult {
	return []usage.SourceResult{
		safeFetch(usage.SourcePlan, func() usage.SourceResult {
			return p.fetchPlan(ctx, now)
		}),
		safeFetch(usage.SourceAPIKey, func() usage.SourceResult {
			return p.fetchAPIKey(ctx)
		}),
	}
}

func safeFetch(source usage.SourceKind, fetch func() usage.SourceResult) (result usage.SourceResult) {
	result.Source = source
	result.Status = usage.StatusUnavailable
	result.Detail = usage.MsgUnreachable("internal error")

	defer func() {
		if recover() != nil {
			result = usage.SourceResult{
				Source: source,
				Status: usage.StatusUnavailable,
				Detail: usage.MsgUnreachable("internal error"),
			}
		}
	}()

	return fetch()
}

func (p *Provider) httpClient() *http.Client {
	if p.client != nil {
		return p.client
	}
	return http.DefaultClient
}
