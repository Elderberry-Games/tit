package claude

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

const claudeOAuthTokenEnv = "CLAUDE_CODE_OAUTH_TOKEN"

type credentialFile struct {
	ClaudeAIOAuth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    int64  `json:"expiresAt"`
	} `json:"claudeAiOauth"`
}

func discoverPlanCredential(now usage.Clock) (string, *usage.SourceResult) {
	if token := os.Getenv(claudeOAuthTokenEnv); token != "" {
		return token, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", unavailableCredential()
	}

	path := filepath.Join(home, ".claude", ".credentials.json")
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		return parseCredential(data, now)
	case !errors.Is(err, os.ErrNotExist):
		return "", unavailableCredential()
	}

	if runtime.GOOS == "darwin" {
		data, err = exec.Command(
			"security",
			"find-generic-password",
			"-s",
			"Claude Code-credentials",
			"-w",
		).Output()
		if err == nil && len(strings.TrimSpace(string(data))) != 0 {
			return parseCredential(data, now)
		}
	}

	result := usage.SourceResult{
		Source: usage.SourcePlan,
		Status: usage.StatusUnconfigured,
		Detail: usage.MsgUnconfigured(
			"Claude credential",
			"run `claude` to log in, or set CLAUDE_CODE_OAUTH_TOKEN",
		),
	}
	return "", &result
}

func parseCredential(data []byte, now usage.Clock) (string, *usage.SourceResult) {
	var file credentialFile
	if err := json.Unmarshal(data, &file); err != nil {
		return "", unavailableCredential()
	}

	credential := file.ClaudeAIOAuth
	if credential.AccessToken == "" {
		result := usage.SourceResult{
			Source: usage.SourcePlan,
			Status: usage.StatusUnconfigured,
			Detail: usage.MsgUnconfigured(
				"Claude credential",
				"run `claude` to log in, or set CLAUDE_CODE_OAUTH_TOKEN",
			),
		}
		return "", &result
	}

	if credential.ExpiresAt != 0 {
		expiresAt := time.UnixMilli(credential.ExpiresAt)
		if !expiresAt.After(clockTime(now)) {
			result := usage.SourceResult{
				Source: usage.SourcePlan,
				Status: usage.StatusUnauthorized,
				Detail: usage.MsgExpired("claude"),
			}
			return "", &result
		}
	}

	return credential.AccessToken, nil
}

func unavailableCredential() *usage.SourceResult {
	return &usage.SourceResult{
		Source: usage.SourcePlan,
		Status: usage.StatusUnavailable,
		Detail: usage.MsgUnreadable(),
	}
}

func clockTime(now usage.Clock) time.Time {
	if now == nil {
		return usage.SystemClock()
	}
	return now()
}
