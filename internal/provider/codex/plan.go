package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

type planPayload struct {
	PlanType  string `json:"plan_type"`
	RateLimit *struct {
		Primary   *currentWindow `json:"primary_window"`
		Secondary *currentWindow `json:"secondary_window"`
	} `json:"rate_limit"`
	RateLimits *struct {
		Primary   *legacyWindow `json:"primary"`
		Secondary *legacyWindow `json:"secondary"`
	} `json:"rate_limits"`
	Credits *struct {
		HasCredits bool   `json:"has_credits"`
		Unlimited  bool   `json:"unlimited"`
		Balance    string `json:"balance"`
	} `json:"credits"`
}

type currentWindow struct {
	UsedPercent       float64 `json:"used_percent"`
	LimitWindowSecond float64 `json:"limit_window_seconds"`
	ResetAt           *int64  `json:"reset_at"`
}

type legacyWindow struct {
	UsedPercent  float64 `json:"used_percent"`
	WindowMinute float64 `json:"window_minutes"`
	ResetsAt     *int64  `json:"resets_at"`
}

func (p *Provider) fetchPlan(ctx context.Context) usage.SourceResult {
	result := usage.SourceResult{Source: usage.SourcePlan}
	creds, err := loadCredentials()
	if err != nil {
		if err == errCredentialsMissing {
			result.Status = usage.StatusUnconfigured
			result.Detail = usage.MsgUnconfigured("Codex credential", "run `codex` to log in")
		} else {
			result.Status = usage.StatusUnavailable
			result.Detail = usage.MsgUnreadable()
		}
		return result
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.planBaseURL+"/backend-api/wham/usage", nil)
	if err != nil {
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreachable("invalid endpoint")
		return result
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("ChatGPT-Account-Id", creds.AccountID)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "codex-cli")

	resp, err := p.client.Do(req)
	if err != nil {
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreachable("request failed")
		return result
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		result.Status = usage.StatusUnauthorized
		result.Detail = usage.MsgUnauthorized("codex")
		return result
	case http.StatusTooManyRequests:
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgRateLimited()
		return result
	default:
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreachable(fmt.Sprintf("HTTP %d", resp.StatusCode))
		return result
	}

	var payload planPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreadable()
		return result
	}

	result.Status = usage.StatusOK
	result.Detail = creds.PlanType
	result.Windows = normalizePlanWindows(payload)
	if payload.Credits != nil && payload.Credits.HasCredits {
		balance := payload.Credits.Balance
		if payload.Credits.Unlimited {
			balance = "unlimited"
		}
		result.Info = append(result.Info, usage.InfoItem{Label: "Credits", Value: balance})
	}
	return result
}

func normalizePlanWindows(payload planPayload) []usage.LimitWindow {
	var windows []usage.LimitWindow
	if payload.RateLimit != nil {
		if payload.RateLimit.Primary != nil {
			windows = append(windows, normalizeCurrentWindow(payload.RateLimit.Primary))
		}
		if payload.RateLimit.Secondary != nil {
			windows = append(windows, normalizeCurrentWindow(payload.RateLimit.Secondary))
		}
		return windows
	}
	if payload.RateLimits != nil {
		if payload.RateLimits.Primary != nil {
			windows = append(windows, normalizeLegacyWindow(payload.RateLimits.Primary))
		}
		if payload.RateLimits.Secondary != nil {
			windows = append(windows, normalizeLegacyWindow(payload.RateLimits.Secondary))
		}
	}
	return windows
}

func normalizeCurrentWindow(window *currentWindow) usage.LimitWindow {
	duration := time.Duration(window.LimitWindowSecond * float64(time.Second))
	return usage.LimitWindow{
		Label:       windowLabel(duration),
		Duration:    duration,
		UsedPercent: window.UsedPercent,
		ResetAt:     unixTime(window.ResetAt),
	}
}

func normalizeLegacyWindow(window *legacyWindow) usage.LimitWindow {
	duration := time.Duration(window.WindowMinute * float64(time.Minute))
	return usage.LimitWindow{
		Label:       windowLabel(duration),
		Duration:    duration,
		UsedPercent: window.UsedPercent,
		ResetAt:     unixTime(window.ResetsAt),
	}
}

func unixTime(epoch *int64) *time.Time {
	if epoch == nil {
		return nil
	}
	value := time.Unix(*epoch, 0)
	return &value
}

func windowLabel(duration time.Duration) string {
	switch duration {
	case 5 * time.Hour:
		return "5-hour"
	case 7 * 24 * time.Hour:
		return "Weekly"
	case 0:
		return ""
	}
	if duration%time.Hour == 0 {
		return fmt.Sprintf("%d-hour", int64(duration/time.Hour))
	}
	if duration%time.Minute == 0 {
		return fmt.Sprintf("%d-minute", int64(duration/time.Minute))
	}
	return duration.String()
}
