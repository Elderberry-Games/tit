package codex

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

var dayDuration = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)d`)

func (p *Provider) fetchAPIKey(ctx context.Context, now usage.Clock) usage.SourceResult {
	result := usage.SourceResult{Source: usage.SourceAPIKey}
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		result.Status = usage.StatusUnconfigured
		result.Detail = usage.MsgUnconfigured("OPENAI_API_KEY", "set it to see API-key rate limits")
		return result
	}

	body := []byte(`{"model":"gpt-5-nano","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":1}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiBaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreachable("invalid endpoint")
		return result
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

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

	currentTime := now()
	result.Windows = appendRateWindow(result.Windows, resp.Header, "requests", "Requests", currentTime)
	result.Windows = appendRateWindow(result.Windows, resp.Header, "tokens", "Tokens", currentTime)
	if len(result.Windows) == 0 {
		result.Status = usage.StatusUnsupported
		result.Detail = usage.MsgNoWindows("this API key")
		return result
	}
	result.Status = usage.StatusOK
	return result
}

func appendRateWindow(windows []usage.LimitWindow, header http.Header, unit, label string, now time.Time) []usage.LimitWindow {
	limit, limitOK := headerFloat(header, "x-ratelimit-limit-"+unit)
	remaining, remainingOK := headerFloat(header, "x-ratelimit-remaining-"+unit)
	if !limitOK || !remainingOK {
		return windows
	}
	used := limit - remaining
	percent := 0.0
	if limit != 0 {
		percent = used / limit * 100
	}
	var resetAt *time.Time
	if reset, err := parseResetDuration(header.Get("x-ratelimit-reset-" + unit)); err == nil {
		value := now.Add(reset)
		resetAt = &value
	}
	return append(windows, usage.LimitWindow{
		Label:       label,
		UsedPercent: percent,
		Used:        floatPointer(used),
		Limit:       floatPointer(limit),
		Unit:        unit,
		ResetAt:     resetAt,
	})
}

func headerFloat(header http.Header, name string) (float64, bool) {
	value := header.Get(name)
	if value == "" {
		return 0, false
	}
	number, err := strconv.ParseFloat(value, 64)
	return number, err == nil
}

func parseResetDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("empty duration")
	}
	var conversionErr error
	converted := dayDuration.ReplaceAllStringFunc(value, func(match string) string {
		days, err := strconv.ParseFloat(strings.TrimSuffix(match, "d"), 64)
		if err != nil {
			conversionErr = err
			return match
		}
		return strconv.FormatFloat(days*24, 'f', -1, 64) + "h"
	})
	if conversionErr != nil {
		return 0, conversionErr
	}
	return time.ParseDuration(converted)
}

func floatPointer(value float64) *float64 {
	return &value
}
