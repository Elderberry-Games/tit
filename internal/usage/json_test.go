package usage

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
	"time"
)

func sampleSnapshot() Snapshot {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	reset := now.Add(2 * time.Hour)

	return Snapshot{
		TakenAt: now,
		Providers: []ProviderResult{
			{
				Provider:    ProviderClaude,
				DisplayName: "Claude",
				Sources: []SourceResult{
					{
						Source: SourcePlan,
						Status: StatusOK,
						Detail: "Max plan",
						Windows: []LimitWindow{
							{Label: "5-hour", Duration: 5 * time.Hour, UsedPercent: 54.321, ResetAt: &reset},
							{Label: "Requests", UsedPercent: 12, Used: ptr(600), Limit: ptr(5000), Unit: "requests"},
						},
					},
				},
			},
			{
				Provider:    ProviderKimi,
				DisplayName: "Kimi",
				Sources: []SourceResult{
					{
						Source: SourceAPIKey,
						Status: StatusUnsupported,
						Detail: MsgNoWindows("Moonshot's open platform"),
						Info:   []InfoItem{{Label: "Balance", Value: "$49.59"}},
					},
				},
			},
		},
	}
}

func encodeSnapshot(t *testing.T, s Snapshot) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	if err := WriteJSON(&buf, s); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	return got
}

func TestWriteJSONTopLevelShape(t *testing.T) {
	got := encodeSnapshot(t, sampleSnapshot())

	if _, ok := got["taken_at"]; !ok {
		t.Error("missing required key taken_at")
	}
	if ts, _ := got["taken_at"].(string); ts != "2026-07-29T12:00:00Z" {
		t.Errorf("taken_at = %q, want RFC3339 UTC", ts)
	}

	providers, ok := got["providers"].([]any)
	if !ok {
		t.Fatalf("providers is %T, want array", got["providers"])
	}
	if len(providers) != 2 {
		t.Fatalf("len(providers) = %d, want 2", len(providers))
	}

	// The schema forbids extra top-level keys.
	if len(got) != 2 {
		t.Errorf("top level has %d keys, want exactly taken_at and providers", len(got))
	}
}

func TestWriteJSONRequiredKeysPresent(t *testing.T) {
	got := encodeSnapshot(t, sampleSnapshot())
	provider := got["providers"].([]any)[0].(map[string]any)

	for _, key := range []string{"provider", "display_name", "sources"} {
		if _, ok := provider[key]; !ok {
			t.Errorf("provider missing required key %q", key)
		}
	}

	source := provider["sources"].([]any)[0].(map[string]any)
	for _, key := range []string{"source", "status", "windows", "info"} {
		if _, ok := source[key]; !ok {
			t.Errorf("source missing required key %q", key)
		}
	}

	window := source["windows"].([]any)[0].(map[string]any)
	for _, key := range []string{"label", "used_percent", "health"} {
		if _, ok := window[key]; !ok {
			t.Errorf("window missing required key %q", key)
		}
	}
}

func TestWriteJSONRoundTripsFigures(t *testing.T) {
	got := encodeSnapshot(t, sampleSnapshot())
	source := got["providers"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)
	windows := source["windows"].([]any)

	first := windows[0].(map[string]any)
	if pct := first["used_percent"].(float64); pct != 54.32 {
		t.Errorf("used_percent = %v, want 54.32 (rounded to 2dp)", pct)
	}
	if d := first["duration_seconds"].(float64); d != 18000 {
		t.Errorf("duration_seconds = %v, want 18000", d)
	}
	if r, _ := first["reset_at"].(string); r != "2026-07-29T14:00:00Z" {
		t.Errorf("reset_at = %q, want 2026-07-29T14:00:00Z", r)
	}
	if secs := first["reset_in_seconds"].(float64); secs != 7200 {
		t.Errorf("reset_in_seconds = %v, want 7200", secs)
	}
	if h := first["health"].(string); h != string(HealthHealthy) {
		t.Errorf("health = %q, want %q", h, HealthHealthy)
	}

	// Absolute counts must survive when the provider reports them.
	second := windows[1].(map[string]any)
	if used := second["used"].(float64); used != 600 {
		t.Errorf("used = %v, want 600", used)
	}
	if limit := second["limit"].(float64); limit != 5000 {
		t.Errorf("limit = %v, want 5000", limit)
	}
	if unit := second["unit"].(string); unit != "requests" {
		t.Errorf("unit = %q, want requests", unit)
	}
}

func TestWriteJSONPercentOnlyWindowsUseNull(t *testing.T) {
	got := encodeSnapshot(t, sampleSnapshot())
	window := got["providers"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)["windows"].([]any)[0].(map[string]any)

	if window["used"] != nil {
		t.Errorf("used = %v, want null for a percent-only window", window["used"])
	}
	if window["limit"] != nil {
		t.Errorf("limit = %v, want null for a percent-only window", window["limit"])
	}
}

func TestWriteJSONEmptyCollectionsAreArraysNotNull(t *testing.T) {
	got := encodeSnapshot(t, sampleSnapshot())
	source := got["providers"].([]any)[1].(map[string]any)["sources"].([]any)[0].(map[string]any)

	// The schema requires windows and info to be arrays; a null here would break
	// naive consumers doing `for w in source.windows`.
	if _, ok := source["windows"].([]any); !ok {
		t.Errorf("windows = %v, want an empty array", source["windows"])
	}
	if info, ok := source["info"].([]any); !ok || len(info) != 1 {
		t.Errorf("info = %v, want a one-element array", source["info"])
	}
}

func TestWriteJSONHealthStates(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	s := Snapshot{TakenAt: now, Providers: []ProviderResult{{
		Provider: ProviderCodex, DisplayName: "Codex",
		Sources: []SourceResult{{Source: SourcePlan, Status: StatusOK, Windows: []LimitWindow{
			{Label: "healthy", UsedPercent: 10},
			{Label: "warning", UsedPercent: 95},
			{Label: "exhausted", UsedPercent: 100},
			{Label: "unlimited", UsedPercent: 100, Unlimited: true},
		}}},
	}}}

	got := encodeSnapshot(t, s)
	windows := got["providers"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)["windows"].([]any)

	want := []string{"healthy", "warning", "exhausted", "healthy"}
	for i, w := range want {
		if h := windows[i].(map[string]any)["health"].(string); h != w {
			t.Errorf("window %d health = %q, want %q", i, h, w)
		}
	}
}

func TestWriteJSONOmitsResetWhenUnknown(t *testing.T) {
	s := Snapshot{TakenAt: time.Now(), Providers: []ProviderResult{{
		Provider: ProviderCodex, DisplayName: "Codex",
		Sources: []SourceResult{{Source: SourcePlan, Status: StatusOK, Windows: []LimitWindow{
			{Label: "no reset", UsedPercent: 5},
		}}},
	}}}

	got := encodeSnapshot(t, s)
	window := got["providers"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)["windows"].([]any)[0].(map[string]any)

	if window["reset_at"] != nil {
		t.Errorf("reset_at = %v, want null", window["reset_at"])
	}
	if window["reset_in_seconds"] != nil {
		t.Errorf("reset_in_seconds = %v, want null", window["reset_in_seconds"])
	}
	if window["duration_seconds"] != nil {
		t.Errorf("duration_seconds = %v, want null when unset", window["duration_seconds"])
	}
}

func TestRound2HandlesNonFiniteValues(t *testing.T) {
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := round2(f); got != 0 {
			t.Errorf("round2(%v) = %v, want 0", f, got)
		}
	}
}
