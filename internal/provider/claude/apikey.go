package claude

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Elderberry-Games/tit/internal/usage"
)

const (
	anthropicAPIKeyEnv = "ANTHROPIC_API_KEY"
	countTokensPath    = "/v1/messages/count_tokens"
	countTokensBody    = `{"model":"claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`
)

type rateLimitHeaderSet struct {
	prefix string
	label  string
	unit   string
}

var rateLimitHeaderSets = []rateLimitHeaderSet{
	{prefix: "anthropic-ratelimit-requests", label: "Requests", unit: "requests"},
	{prefix: "anthropic-ratelimit-tokens", label: "Tokens", unit: "tokens"},
	{prefix: "anthropic-ratelimit-input-tokens", label: "Input tokens", unit: "tokens"},
	{prefix: "anthropic-ratelimit-output-tokens", label: "Output tokens", unit: "tokens"},
}

func (p *Provider) fetchAPIKey(ctx context.Context) usage.SourceResult {
	key := os.Getenv(anthropicAPIKeyEnv)
	if key == "" {
		return usage.SourceResult{
			Source: usage.SourceAPIKey,
			Status: usage.StatusUnconfigured,
			Detail: usage.MsgUnconfigured(
				"ANTHROPIC_API_KEY",
				"set it to see API-key rate limits",
			),
		}
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(p.apiKeyBaseURL, "/")+countTokensPath,
		bytes.NewBufferString(countTokensBody),
	)
	if err != nil {
		return apiKeyUnavailable("request failed")
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return apiKeyUnavailable("request failed")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusTooManyRequests:
	case http.StatusUnauthorized, http.StatusForbidden:
		return usage.SourceResult{
			Source: usage.SourceAPIKey,
			Status: usage.StatusUnauthorized,
			Detail: usage.MsgUnauthorized("claude"),
		}
	default:
		return apiKeyUnavailable(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}

	windows, malformed := parseRateLimitHeaders(resp.Header)
	if malformed {
		return usage.SourceResult{
			Source: usage.SourceAPIKey,
			Status: usage.StatusUnavailable,
			Detail: usage.MsgUnreadable(),
		}
	}
	if len(windows) == 0 {
		return usage.SourceResult{
			Source: usage.SourceAPIKey,
			Status: usage.StatusUnsupported,
			Detail: usage.MsgNoWindows("this API key"),
		}
	}

	return usage.SourceResult{
		Source:  usage.SourceAPIKey,
		Status:  usage.StatusOK,
		Windows: windows,
	}
}

func parseRateLimitHeaders(header http.Header) ([]usage.LimitWindow, bool) {
	windows := make([]usage.LimitWindow, 0, len(rateLimitHeaderSets))
	for _, set := range rateLimitHeaderSets {
		limitValue := header.Get(set.prefix + "-limit")
		remainingValue := header.Get(set.prefix + "-remaining")
		resetValue := header.Get(set.prefix + "-reset")
		if limitValue == "" || remainingValue == "" || resetValue == "" {
			continue
		}

		limit, err := strconv.ParseFloat(limitValue, 64)
		if err != nil {
			return nil, true
		}
		remaining, err := strconv.ParseFloat(remainingValue, 64)
		if err != nil {
			return nil, true
		}
		reset := parseReset(resetValue)
		if reset == nil {
			return nil, true
		}

		used := limit - remaining
		percent := 0.0
		if limit != 0 {
			percent = used / limit * 100
		}
		windows = append(windows, usage.LimitWindow{
			Label:       set.label,
			UsedPercent: percent,
			Used:        floatPointer(used),
			Limit:       floatPointer(limit),
			Unit:        set.unit,
			ResetAt:     reset,
		})
	}
	return windows, false
}

func floatPointer(value float64) *float64 {
	return &value
}

func apiKeyUnavailable(reason string) usage.SourceResult {
	return usage.SourceResult{
		Source: usage.SourceAPIKey,
		Status: usage.StatusUnavailable,
		Detail: usage.MsgUnreachable(reason),
	}
}
