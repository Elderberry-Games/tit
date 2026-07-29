package kimi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func planServer(t *testing.T, status int, fixture string, hits *atomic.Int32) *httptest.Server {
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
		if hits != nil {
			hits.Add(1)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer REDACTED-TOKEN" {
			t.Errorf("Authorization = %q, want Bearer REDACTED-TOKEN", got)
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
}

func newPlanProvider(server *httptest.Server) *Provider {
	p := New()
	p.planBaseURL = server.URL
	return p
}

func TestFetchPlanSuccess(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	server := planServer(t, http.StatusOK, "plan_success.json", nil)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusOK {
		t.Fatalf("Status = %q (%s), want ok", result.Status, result.Detail)
	}
	if result.Detail != "Allegretto" {
		t.Fatalf("Detail = %q, want Allegretto", result.Detail)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("len(Windows) = %d, want 2", len(result.Windows))
	}

	weekly := result.Windows[0]
	if weekly.Label != "Weekly" || weekly.Duration != 7*24*time.Hour {
		t.Errorf("weekly = %+v, want label Weekly with 168h duration", weekly)
	}
	assertCounts(t, weekly, 214, 2048)
	if weekly.ResetAt == nil || weekly.ResetAt.Nanosecond() != 716839300 {
		t.Errorf("weekly.ResetAt = %v, want nanosecond .716839300", weekly.ResetAt)
	}

	fiveHour := result.Windows[1]
	if fiveHour.Label != "5-hour" || fiveHour.Duration != 5*time.Hour {
		t.Errorf("fiveHour = %+v, want label 5-hour with 5h duration", fiveHour)
	}
	assertCounts(t, fiveHour, 139, 200)
	if fiveHour.ResetAt == nil || fiveHour.ResetAt.Nanosecond() != 717479433 {
		t.Errorf("fiveHour.ResetAt = %v, want nanosecond .717479433", fiveHour.ResetAt)
	}
}

func assertCounts(t *testing.T, window usage.LimitWindow, used, limit float64) {
	t.Helper()
	if window.Used == nil || *window.Used != used {
		t.Errorf("Used = %v, want %v", window.Used, used)
	}
	if window.Limit == nil || *window.Limit != limit {
		t.Errorf("Limit = %v, want %v", window.Limit, limit)
	}
	if window.Unit != "requests" {
		t.Errorf("Unit = %q, want requests", window.Unit)
	}
	want := used / limit * 100
	if window.UsedPercent != want {
		t.Errorf("UsedPercent = %v, want %v", window.UsedPercent, want)
	}
}

func TestFetchPlanUsedComputedFromRemaining(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	server := planServer(t, http.StatusOK, "plan_used_absent.json", nil)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusOK {
		t.Fatalf("Status = %q (%s), want ok", result.Status, result.Detail)
	}
	if result.Detail != "Free" {
		t.Errorf("Detail = %q, want Free", result.Detail)
	}
	if len(result.Windows) != 3 {
		t.Fatalf("len(Windows) = %d, want 3 (null entries skipped)", len(result.Windows))
	}

	// used is absent: 500 - 125 = 375, reset via the alternate reset_at key.
	weekly := result.Windows[0]
	assertCounts(t, weekly, 375, 500)
	if weekly.ResetAt == nil {
		t.Error("weekly.ResetAt = nil, want parsed reset_at")
	}

	// 5h via TIME_UNIT_HOUR, used computed, reset via resetAt key.
	fiveHour := result.Windows[1]
	if fiveHour.Label != "5-hour" || fiveHour.Duration != 5*time.Hour {
		t.Errorf("fiveHour = %+v, want label 5-hour with 5h duration", fiveHour)
	}
	assertCounts(t, fiveHour, 60, 100)
	if fiveHour.ResetAt == nil {
		t.Error("fiveHour.ResetAt = nil, want parsed resetAt")
	}

	// Humanized label for 2 days; an unparseable reset_time yields nil.
	twoDay := result.Windows[2]
	if twoDay.Label != "48-hour" || twoDay.Duration != 48*time.Hour {
		t.Errorf("twoDay = %+v, want label 48-hour with 48h duration", twoDay)
	}
	if twoDay.ResetAt != nil {
		t.Errorf("twoDay.ResetAt = %v, want nil for unparseable reset_time", twoDay.ResetAt)
	}
}

func TestFetchPlanExpiredCredentialSkipsHTTP(t *testing.T) {
	home := isolateHome(t)
	writeCredentialFile(t, filepath.Join(home, ".kimi-code", "credentials", "kimi-code.json"),
		`{"access_token":"REDACTED-TOKEN","expires_at":1752561221.443}`)

	var hits atomic.Int32
	server := planServer(t, http.StatusOK, "plan_success.json", &hits)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusUnauthorized {
		t.Fatalf("Status = %q, want unauthorized", result.Status)
	}
	if result.Detail != usage.MsgExpired("kimi") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgExpired("kimi"))
	}
	if hits.Load() != 0 {
		t.Errorf("HTTP hits = %d, want 0 for an expired credential", hits.Load())
	}
}

func TestFetchPlanUnconfigured(t *testing.T) {
	isolateHome(t)
	server := planServer(t, http.StatusOK, "plan_success.json", nil)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusUnconfigured {
		t.Fatalf("Status = %q, want unconfigured", result.Status)
	}
	want := usage.MsgUnconfigured("Kimi credential", "run `kimi` to log in, or set KIMI_CODE_API_KEY")
	if result.Detail != want {
		t.Errorf("Detail = %q, want %q", result.Detail, want)
	}
}

func TestFetchPlanUnauthorized(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	server := planServer(t, http.StatusUnauthorized, "", nil)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusUnauthorized {
		t.Fatalf("Status = %q, want unauthorized", result.Status)
	}
	if result.Detail != usage.MsgUnauthorized("kimi") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgUnauthorized("kimi"))
	}
}

func TestFetchPlanNotFoundIsUnsupported(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	server := planServer(t, http.StatusNotFound, "", nil)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusUnsupported {
		t.Fatalf("Status = %q, want unsupported", result.Status)
	}
	if result.Detail != usage.MsgNoWindows("this Kimi key") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgNoWindows("this Kimi key"))
	}
}

func TestFetchPlanRateLimited(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	server := planServer(t, http.StatusTooManyRequests, "", nil)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", result.Status)
	}
	if result.Detail != usage.MsgRateLimited() {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgRateLimited())
	}
}

func TestFetchPlanServerError(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	server := planServer(t, http.StatusInternalServerError, "", nil)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", result.Status)
	}
	if result.Detail != usage.MsgUnreachable("HTTP 500") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgUnreachable("HTTP 500"))
	}
}

func TestFetchPlanMalformedJSON(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	server := planServer(t, http.StatusOK, "malformed.json", nil)
	defer server.Close()

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", result.Status)
	}
	if result.Detail != usage.MsgUnreadable() {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgUnreadable())
	}
}

func TestFetchPlanUnreachable(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close() // closed before the request: connection refused

	result := newPlanProvider(server).fetchPlan(context.Background(), testNow)

	if result.Status != usage.StatusUnavailable {
		t.Fatalf("Status = %q, want unavailable", result.Status)
	}
	if result.Detail != usage.MsgUnreachable("request failed") {
		t.Errorf("Detail = %q, want %q", result.Detail, usage.MsgUnreachable("request failed"))
	}
}

func TestProviderFetchReturnsBothSources(t *testing.T) {
	isolateHome(t)
	t.Setenv("KIMI_CODE_API_KEY", "REDACTED-TOKEN")
	t.Setenv("MOONSHOT_API_KEY", "")

	p := New()
	if p.ID() != usage.ProviderKimi {
		t.Errorf("ID = %q, want kimi", p.ID())
	}
	if p.DisplayName() != "Kimi" {
		t.Errorf("DisplayName = %q, want Kimi", p.DisplayName())
	}
	if p.MinPollInterval() != 60*time.Second {
		t.Errorf("MinPollInterval = %v, want 60s", p.MinPollInterval())
	}

	server := planServer(t, http.StatusOK, "plan_success.json", nil)
	defer server.Close()
	p.planBaseURL = server.URL

	results := p.Fetch(context.Background(), nil)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Source != usage.SourcePlan || results[0].Status != usage.StatusOK {
		t.Errorf("plan result = %+v, want ok", results[0])
	}
	if results[1].Source != usage.SourceAPIKey || results[1].Status != usage.StatusUnconfigured {
		t.Errorf("apikey result = %+v, want unconfigured", results[1])
	}
}

func TestWindowLabel(t *testing.T) {
	cases := []struct {
		duration time.Duration
		want     string
	}{
		{5 * time.Hour, "5-hour"},
		{7 * 24 * time.Hour, "Weekly"},
		{0, ""},
		{3 * time.Hour, "3-hour"},
		{45 * time.Minute, "45-minute"},
		{90 * time.Second, "1m30s"},
	}
	for _, tc := range cases {
		if got := windowLabel(tc.duration); got != tc.want {
			t.Errorf("windowLabel(%v) = %q, want %q", tc.duration, got, tc.want)
		}
	}
}

func TestWindowDuration(t *testing.T) {
	if got := windowDuration(nil); got != 0 {
		t.Errorf("windowDuration(nil) = %v, want 0", got)
	}
	cases := []struct {
		unit string
		want time.Duration
	}{
		{"TIME_UNIT_MINUTE", 30 * time.Minute},
		{"TIME_UNIT_HOUR", 30 * time.Hour},
		{"TIME_UNIT_DAY", 30 * 24 * time.Hour},
		{"TIME_UNIT_WEEK", 30 * 7 * 24 * time.Hour},
		{"TIME_UNIT_UNKNOWN", 0},
	}
	for _, tc := range cases {
		spec := &windowSpec{Duration: 30, TimeUnit: tc.unit}
		if got := windowDuration(spec); got != tc.want {
			t.Errorf("windowDuration(%s) = %v, want %v", tc.unit, got, tc.want)
		}
	}
}

func TestPlanName(t *testing.T) {
	cases := map[string]string{
		"LEVEL_FREE":         "Free",
		"LEVEL_BASIC":        "Adagio",
		"LEVEL_STANDARD":     "Moderato",
		"LEVEL_INTERMEDIATE": "Allegretto",
		"LEVEL_ADVANCED":     "Allegro",
		"LEVEL_PREMIUM":      "Vivace",
		"LEVEL_WHATEVER":     "LEVEL_WHATEVER",
	}
	for level, want := range cases {
		if got := planName(level); got != want {
			t.Errorf("planName(%q) = %q, want %q", level, got, want)
		}
	}
}
