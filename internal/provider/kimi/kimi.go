package kimi

import (
	"context"
	"net/http"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

const (
	defaultPlanBaseURL      = "https://api.kimi.com"
	defaultBalanceBaseURLAI = "https://api.moonshot.ai"
	defaultBalanceBaseURLCN = "https://api.moonshot.cn"
)

type Provider struct {
	client           *http.Client
	planBaseURL      string
	balanceBaseURLAI string
	balanceBaseURLCN string
}

func New() *Provider {
	return &Provider{
		client:           http.DefaultClient,
		planBaseURL:      defaultPlanBaseURL,
		balanceBaseURLAI: defaultBalanceBaseURLAI,
		balanceBaseURLCN: defaultBalanceBaseURLCN,
	}
}

func (p *Provider) ID() usage.ProviderID {
	return usage.ProviderKimi
}

func (p *Provider) DisplayName() string {
	return "Kimi"
}

func (p *Provider) MinPollInterval() time.Duration {
	return 60 * time.Second
}

func (p *Provider) Fetch(ctx context.Context, now usage.Clock) []usage.SourceResult {
	if now == nil {
		now = usage.SystemClock
	}
	return []usage.SourceResult{
		safeFetch(usage.SourcePlan, func() usage.SourceResult { return p.fetchPlan(ctx, now) }),
		safeFetch(usage.SourceAPIKey, func() usage.SourceResult { return p.fetchAPIKey(ctx) }),
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
