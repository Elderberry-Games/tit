package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Elderberry-Games/tit/internal/usage"
)

type balancePayload struct {
	Data *struct {
		AvailableBalance float64 `json:"available_balance"`
	} `json:"data"`
}

// fetchAPIKey reads the Moonshot open platform balance. The platform
// publishes no rate-limit windows, so a successful fetch always reports
// StatusUnsupported with the balance as an info item.
func (p *Provider) fetchAPIKey(ctx context.Context) usage.SourceResult {
	result := usage.SourceResult{Source: usage.SourceAPIKey}
	key := os.Getenv("MOONSHOT_API_KEY")
	if key == "" {
		result.Status = usage.StatusUnconfigured
		result.Detail = usage.MsgUnconfigured("MOONSHOT_API_KEY", "set it to see your Moonshot balance")
		return result
	}

	baseURL := p.balanceBaseURLAI
	currency := "$"
	if os.Getenv("MOONSHOT_REGION") == "cn" {
		baseURL = p.balanceBaseURLCN
		currency = "¥"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/users/me/balance", nil)
	if err != nil {
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreachable("invalid endpoint")
		return result
	}
	req.Header.Set("Authorization", "Bearer "+key)

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
	case http.StatusTooManyRequests:
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgRateLimited()
		return result
	default:
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreachable(fmt.Sprintf("HTTP %d", resp.StatusCode))
		return result
	}

	var payload balancePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		result.Status = usage.StatusUnavailable
		result.Detail = usage.MsgUnreadable()
		return result
	}

	result.Status = usage.StatusUnsupported
	result.Detail = usage.MsgNoWindows("Moonshot's open platform")
	if payload.Data != nil {
		result.Info = append(result.Info, usage.InfoItem{
			Label: "Balance",
			Value: fmt.Sprintf("%s%.2f", currency, payload.Data.AvailableBalance),
		})
	}
	return result
}
