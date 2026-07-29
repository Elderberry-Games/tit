package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

// The usages endpoint encodes numeric values as decimal strings; json.Number
// accepts both string and number literals.
type usagesPayload struct {
	User *struct {
		Membership *struct {
			Level string `json:"level"`
		} `json:"membership"`
	} `json:"user"`
	Usage  *quotaBlock  `json:"usage"`
	Limits []limitEntry `json:"limits"`
}

type limitEntry struct {
	Window *windowSpec `json:"window"`
	Detail *quotaBlock `json:"detail"`
}

type windowSpec struct {
	Duration float64 `json:"duration"`
	TimeUnit string  `json:"timeUnit"`
}

type quotaBlock struct {
	Limit          *json.Number `json:"limit"`
	Used           *json.Number `json:"used"`
	Remaining      *json.Number `json:"remaining"`
	ResetTime      string       `json:"resetTime"`
	ResetAt        string       `json:"reset_at"`
	ResetAtCamel   string       `json:"resetAt"`
	ResetTimeSnake string       `json:"reset_time"`
}

func (p *Provider) fetchPlan(ctx context.Context, now usage.Clock) usage.SourceResult {
	result := usage.SourceResult{Source: usage.SourcePlan}
	creds, err := loadCredentials(now)
	if err != nil {
		switch err {
		case errCredentialsMissing:
			result.Status = usage.StatusUnconfigured
			result.Detail = usage.MsgUnconfigured("Kimi credential", "run `kimi` to log in, or set KIMI_CODE_API_KEY")
		case errCredentialsExpired:
			result.Status = usage.StatusUnauthorized
			result.Detail = usage.MsgExpired("kimi")
		default:
			result.Status = usage.StatusUnavailable
			result.Detail = usage.MsgUnreadable()
		}
		return result
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.planBaseURL+"/coding/v1/usages", nil)
	if err != nil {
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreachable("invalid endpoint")
		return result
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/json")

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
		result.Detail = usage.MsgUnauthorized("kimi")
		return result
	case http.StatusNotFound:
		result.Status = usage.StatusUnsupported
		result.Detail = usage.MsgNoWindows("this Kimi key")
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

	var payload usagesPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreadable()
		return result
	}

	result.Status = usage.StatusOK
	if payload.User != nil && payload.User.Membership != nil && payload.User.Membership.Level != "" {
		result.Detail = planName(payload.User.Membership.Level)
	}
	if payload.Usage != nil {
		result.Windows = append(result.Windows, quotaWindow("Weekly", 7*24*time.Hour, payload.Usage))
	}
	for _, entry := range payload.Limits {
		if entry.Detail == nil {
			continue
		}
		duration := windowDuration(entry.Window)
		result.Windows = append(result.Windows, quotaWindow(windowLabel(duration), duration, entry.Detail))
	}
	return result
}

func quotaWindow(label string, duration time.Duration, block *quotaBlock) usage.LimitWindow {
	window := usage.LimitWindow{Label: label, Duration: duration}
	limit, hasLimit := numberValue(block.Limit)
	used, hasUsed := numberValue(block.Used)
	if !hasUsed && hasLimit {
		if remaining, ok := numberValue(block.Remaining); ok {
			used = limit - remaining
			hasUsed = true
		}
	}
	if hasLimit {
		window.Limit = floatPointer(limit)
		window.Unit = "requests"
	}
	if hasUsed {
		window.Used = floatPointer(used)
		window.Unit = "requests"
	}
	if hasUsed && hasLimit && limit != 0 {
		window.UsedPercent = used / limit * 100
	}
	window.ResetAt = block.resetTime()
	return window
}

func (b *quotaBlock) resetTime() *time.Time {
	for _, raw := range []string{b.ResetTime, b.ResetAtCamel, b.ResetAt, b.ResetTimeSnake} {
		if raw == "" {
			continue
		}
		if value, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return &value
		}
	}
	return nil
}

func windowDuration(window *windowSpec) time.Duration {
	if window == nil {
		return 0
	}
	var unit time.Duration
	switch window.TimeUnit {
	case "TIME_UNIT_MINUTE":
		unit = time.Minute
	case "TIME_UNIT_HOUR":
		unit = time.Hour
	case "TIME_UNIT_DAY":
		unit = 24 * time.Hour
	case "TIME_UNIT_WEEK":
		unit = 7 * 24 * time.Hour
	default:
		return 0
	}
	return time.Duration(window.Duration * float64(unit))
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

func planName(level string) string {
	switch level {
	case "LEVEL_FREE":
		return "Free"
	case "LEVEL_BASIC":
		return "Adagio"
	case "LEVEL_STANDARD":
		return "Moderato"
	case "LEVEL_INTERMEDIATE":
		return "Allegretto"
	case "LEVEL_ADVANCED":
		return "Allegro"
	case "LEVEL_PREMIUM":
		return "Vivace"
	}
	return level
}

func numberValue(number *json.Number) (float64, bool) {
	if number == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(number.String(), 64)
	return value, err == nil
}

func floatPointer(value float64) *float64 {
	return &value
}
