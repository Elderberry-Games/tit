package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

const planUsagePath = "/api/oauth/usage"

type planWindow struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

type planResponse struct {
	FiveHour       *planWindow `json:"five_hour"`
	SevenDay       *planWindow `json:"seven_day"`
	SevenDayOpus   *planWindow `json:"seven_day_opus"`
	SevenDaySonnet *planWindow `json:"seven_day_sonnet"`
}

func (p *Provider) fetchPlan(ctx context.Context, now usage.Clock) usage.SourceResult {
	token, credentialResult := discoverPlanCredential(now)
	if credentialResult != nil {
		return *credentialResult
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(p.planBaseURL, "/")+planUsagePath,
		nil,
	)
	if err != nil {
		return planUnavailable("request failed")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-code/2.1.220")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return planUnavailable("request failed")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return usage.SourceResult{
			Source: usage.SourcePlan,
			Status: usage.StatusUnauthorized,
			Detail: usage.MsgUnauthorized("claude"),
		}
	case http.StatusTooManyRequests:
		return usage.SourceResult{
			Source: usage.SourcePlan,
			Status: usage.StatusUnavailable,
			Detail: usage.MsgRateLimited(),
		}
	default:
		return planUnavailable(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}

	var response planResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&response); err != nil {
		return unreadablePlan()
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return unreadablePlan()
	}

	windows := make([]usage.LimitWindow, 0, 4)
	appendPlanWindow := func(label string, duration time.Duration, window *planWindow) {
		if window == nil {
			return
		}
		windows = append(windows, usage.LimitWindow{
			Label:       label,
			Duration:    duration,
			UsedPercent: window.Utilization,
			ResetAt:     parseReset(window.ResetsAt),
		})
	}
	appendPlanWindow("5-hour", 5*time.Hour, response.FiveHour)
	appendPlanWindow("Weekly", 168*time.Hour, response.SevenDay)
	appendPlanWindow("Weekly (Opus)", 168*time.Hour, response.SevenDayOpus)
	appendPlanWindow("Weekly (Sonnet)", 168*time.Hour, response.SevenDaySonnet)

	return usage.SourceResult{
		Source:  usage.SourcePlan,
		Status:  usage.StatusOK,
		Windows: windows,
	}
}

func parseReset(value string) *time.Time {
	if value == "" {
		return nil
	}
	reset, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &reset
}

func planUnavailable(reason string) usage.SourceResult {
	return usage.SourceResult{
		Source: usage.SourcePlan,
		Status: usage.StatusUnavailable,
		Detail: usage.MsgUnreachable(reason),
	}
}

func unreadablePlan() usage.SourceResult {
	return usage.SourceResult{
		Source: usage.SourcePlan,
		Status: usage.StatusUnavailable,
		Detail: usage.MsgUnreadable(),
	}
}
