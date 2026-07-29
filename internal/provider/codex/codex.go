package codex

import (
	"context"
	"net/http"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

const (
	defaultPlanBaseURL = "https://chatgpt.com"
	defaultAPIBaseURL  = "https://api.openai.com"
)

type Provider struct {
	client      *http.Client
	planBaseURL string
	apiBaseURL  string
}

func New() *Provider {
	return &Provider{
		client:      http.DefaultClient,
		planBaseURL: defaultPlanBaseURL,
		apiBaseURL:  defaultAPIBaseURL,
	}
}

func (p *Provider) ID() usage.ProviderID {
	return usage.ProviderCodex
}

func (p *Provider) DisplayName() string {
	return "Codex"
}

func (p *Provider) MinPollInterval() time.Duration {
	return 60 * time.Second
}

func (p *Provider) Fetch(ctx context.Context, now usage.Clock) []usage.SourceResult {
	if now == nil {
		now = usage.SystemClock
	}
	return []usage.SourceResult{
		safeFetch(usage.SourcePlan, func() usage.SourceResult { return p.fetchPlan(ctx) }),
		safeFetch(usage.SourceAPIKey, func() usage.SourceResult { return p.fetchAPIKey(ctx, now) }),
	}
}

func safeFetch(source usage.SourceKind, fetch func() usage.SourceResult) (result usage.SourceResult) {
	defer func() {
		if recover() != nil {
			result = usage.SourceResult{
				Source: source,
				Status: usage.StatusUnavailable,
				Detail: usage.MsgUnreadable(),
			}
		}
	}()
	return fetch()
}
