package kimi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func balanceServer(t *testing.T, status int, fixture string) *httptest.Server {
	t.Helper()
	var body []byte
	if fixture != "" {
		var err error
		body, err = os.ReadFile(filepath.Join("testdata", fixture))
		if err != nil {
			t.Fatal(err)
		}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer REDACTED-TOKEN" {
			t.Errorf("Authorization = %q, want Bearer REDACTED-TOKEN", got)
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
}

func TestFetchAPIKeyUnconfigured(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "")

	result := New().fetchAPIKey(context.Background())

	if result.Status != usage.StatusUnconfigured {
		t.Fatalf("Status = %q, want unconfigured", result.Status)
	}
	want := usage.MsgUnconfigured("MOONSHOT_API_KEY", "set it to see your Moonshot balance")
	if result.Detail != want {
		t.Errorf("Detail = %q, want %q", result.Detail, want)
	}
}

func TestFetchAPIKeyBalance(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "REDACTED-TOKEN")
	t.Setenv("MOONSHOT_REGION", "")
	server := balanceServer(t, http.StatusOK, "balance.json")
	defer server.Close()

	p := New()
	p.balanceBaseURLAI = server.URL
	result := p.fetchAPIKey(context.Background())

	if result.Status != usage.StatusUnsupported {
		t.Fatalf("Status = %q, want unsupported", result.Status)
	}
	if result.Detail != usage.MsgNoWindows("Moonshot's open platform") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgNoWindows("Moonshot's open platform"))
	}
	if len(result.Windows) != 0 {
		t.Errorf("len(Windows) = %d, want 0", len(result.Windows))
	}
	if len(result.Info) != 1 || result.Info[0].Label != "Balance" || result.Info[0].Value != "$49.59" {
		t.Errorf("Info = %+v, want Balance $49.59", result.Info)
	}
}

func TestFetchAPIKeyRegionCN(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "REDACTED-TOKEN")
	t.Setenv("MOONSHOT_REGION", "cn")
	server := balanceServer(t, http.StatusOK, "balance.json")
	defer server.Close()

	p := New()
	p.balanceBaseURLCN = server.URL
	// Point the .ai host somewhere dead so a wrong selection fails loudly.
	p.balanceBaseURLAI = "http://127.0.0.1:1"
	result := p.fetchAPIKey(context.Background())

	if result.Status != usage.StatusUnsupported {
		t.Fatalf("Status = %q, want unsupported", result.Status)
	}
	if len(result.Info) != 1 || result.Info[0].Value != "¥49.59" {
		t.Errorf("Info = %+v, want Balance ¥49.59", result.Info)
	}
}

func TestFetchAPIKeyUnauthorized(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "REDACTED-TOKEN")
	server := balanceServer(t, http.StatusUnauthorized, "")
	defer server.Close()

	p := New()
	p.balanceBaseURLAI = server.URL
	result := p.fetchAPIKey(context.Background())

	if result.Status != usage.StatusUnauthorized {
		t.Fatalf("Status = %q, want unauthorized", result.Status)
	}
	if result.Detail != usage.MsgUnauthorized("kimi") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgUnauthorized("kimi"))
	}
}

func TestFetchAPIKeyServerError(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "REDACTED-TOKEN")
	server := balanceServer(t, http.StatusBadGateway, "")
	defer server.Close()

	p := New()
	p.balanceBaseURLAI = server.URL
	result := p.fetchAPIKey(context.Background())

	if result.Status != usage.StatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", result.Status)
	}
	if result.Detail != usage.MsgUnreachable("HTTP 502") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgUnreachable("HTTP 502"))
	}
}

func TestFetchAPIKeyMalformedJSON(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "REDACTED-TOKEN")
	server := balanceServer(t, http.StatusOK, "malformed.json")
	defer server.Close()

	p := New()
	p.balanceBaseURLAI = server.URL
	result := p.fetchAPIKey(context.Background())

	if result.Status != usage.StatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", result.Status)
	}
	if result.Detail != usage.MsgUnreadable() {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgUnreadable())
	}
}

func TestFetchAPIKeyUnreachable(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "REDACTED-TOKEN")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	p := New()
	p.balanceBaseURLAI = server.URL
	result := p.fetchAPIKey(context.Background())

	if result.Status != usage.StatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", result.Status)
	}
	if result.Detail != usage.MsgUnreachable("request failed") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgUnreachable("request failed"))
	}
}
