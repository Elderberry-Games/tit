package kimi

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

var (
	errCredentialsMissing = errors.New("kimi credentials missing")
	errCredentialsExpired = errors.New("kimi credentials expired")
)

type credentials struct {
	AccessToken string
}

type credentialFile struct {
	AccessToken string   `json:"access_token"`
	ExpiresAt   *float64 `json:"expires_at"`
}

// loadCredentials discovers a Kimi for Coding bearer token. OAuth tokens are
// never refreshed here: an expired token is reported so the user can
// re-authenticate with the harness CLI.
func loadCredentials(now usage.Clock) (credentials, error) {
	if key := os.Getenv("KIMI_CODE_API_KEY"); key != "" {
		return credentials{AccessToken: key}, nil
	}

	var paths []string
	if dir := os.Getenv("KIMI_CODE_HOME"); dir != "" {
		paths = append(paths, filepath.Join(dir, "credentials", "kimi-code.json"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".kimi-code", "credentials", "kimi-code.json"),
			filepath.Join(home, ".kimi", "credentials", "kimi-code.json"),
		)
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return credentials{}, err
		}
		var file credentialFile
		if err := json.Unmarshal(data, &file); err != nil {
			return credentials{}, err
		}
		if file.AccessToken == "" {
			continue
		}
		if file.ExpiresAt != nil && expiredAt(*file.ExpiresAt, now()) {
			return credentials{}, errCredentialsExpired
		}
		return credentials{AccessToken: file.AccessToken}, nil
	}
	return credentials{}, errCredentialsMissing
}

// expiredAt interprets expires_at as UNIX seconds; values past 1e12 are
// milliseconds instead. Kimi access tokens live only 15 minutes, so expiry is
// the common case.
func expiredAt(expiresAt float64, now time.Time) bool {
	seconds := expiresAt
	if seconds > 1e12 {
		seconds = seconds / 1000
	}
	return !time.Unix(int64(seconds), 0).After(now)
}
