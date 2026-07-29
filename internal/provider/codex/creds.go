package codex

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var errCredentialsMissing = errors.New("codex credentials missing")

type credentials struct {
	AccessToken string
	AccountID   string
	PlanType    string
}

type authFile struct {
	Tokens struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	} `json:"tokens"`
}

type tokenClaims struct {
	Auth struct {
		AccountID string `json:"chatgpt_account_id"`
		PlanType  string `json:"chatgpt_plan_type"`
	} `json:"https://api.openai.com/auth"`
}

func loadCredentials() (credentials, error) {
	dir := os.Getenv("CODEX_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return credentials{}, err
		}
		dir = filepath.Join(home, ".codex")
	}

	data, err := os.ReadFile(filepath.Join(dir, "auth.json"))
	if errors.Is(err, os.ErrNotExist) {
		return credentials{}, errCredentialsMissing
	}
	if err != nil {
		return credentials{}, err
	}

	var auth authFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return credentials{}, err
	}
	if auth.Tokens.AccessToken == "" {
		return credentials{}, errCredentialsMissing
	}

	claims, _ := decodeClaims(auth.Tokens.IDToken)
	accountID := auth.Tokens.AccountID
	if accountID == "" {
		accountID = claims.Auth.AccountID
	}
	return credentials{
		AccessToken: auth.Tokens.AccessToken,
		AccountID:   accountID,
		PlanType:    claims.Auth.PlanType,
	}, nil
}

func decodeClaims(token string) (tokenClaims, error) {
	var claims tokenClaims
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return claims, errors.New("invalid JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, err
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, err
	}
	return claims, nil
}
