package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/Elderberry-Games/tit/internal/usage"
)

var testNow = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

func fptr(f float64) *float64 { return &f }

func tptr(t time.Time) *time.Time { return &t }

func allHealthySnapshot() usage.Snapshot {
	return usage.Snapshot{
		TakenAt: testNow,
		Providers: []usage.ProviderResult{
			{
				Provider: usage.ProviderClaude, DisplayName: "Claude",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourcePlan, Status: usage.StatusOK, Detail: "Max plan",
						Windows: []usage.LimitWindow{
							{Label: "5-hour", Duration: 5 * time.Hour, UsedPercent: 54, ResetAt: tptr(testNow.Add(2*time.Hour + 14*time.Minute))},
							{Label: "Weekly", Duration: 7 * 24 * time.Hour, UsedPercent: 31, ResetAt: tptr(testNow.Add(3*24*time.Hour + 4*time.Hour))},
						},
					},
					{
						Source: usage.SourceAPIKey, Status: usage.StatusOK,
						Windows: []usage.LimitWindow{
							{Label: "Requests", UsedPercent: 12, Used: fptr(600), Limit: fptr(5000), Unit: "requests", ResetAt: tptr(testNow.Add(45 * time.Second))},
						},
					},
				},
			},
			{
				Provider: usage.ProviderCodex, DisplayName: "Codex",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourcePlan, Status: usage.StatusOK, Detail: "Plus",
						Windows: []usage.LimitWindow{
							{Label: "5-hour", UsedPercent: 8, ResetAt: tptr(testNow.Add(76 * time.Minute))},
						},
					},
				},
			},
			{
				Provider: usage.ProviderKimi, DisplayName: "Kimi",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourceAPIKey, Status: usage.StatusOK,
						Windows: []usage.LimitWindow{
							{Label: "Tokens/min", UsedPercent: 62.4, Used: fptr(12480), Limit: fptr(20000), Unit: "tokens"},
						},
					},
				},
			},
		},
	}
}

func mixedHealthSnapshot() usage.Snapshot {
	return usage.Snapshot{
		TakenAt: testNow,
		Providers: []usage.ProviderResult{
			{
				Provider: usage.ProviderClaude, DisplayName: "Claude",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourcePlan, Status: usage.StatusOK, Detail: "Max plan",
						Windows: []usage.LimitWindow{
							{Label: "5-hour", UsedPercent: 54, ResetAt: tptr(testNow.Add(2*time.Hour + 14*time.Minute))},
							{Label: "Weekly", UsedPercent: 91, ResetAt: tptr(testNow.Add(3*24*time.Hour + 4*time.Hour))},
							{Label: "Weekly (Opus)", UsedPercent: 100, ResetAt: tptr(testNow.Add(-time.Hour))},
						},
					},
				},
			},
			{
				Provider: usage.ProviderKimi, DisplayName: "Kimi",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourceAPIKey, Status: usage.StatusOK,
						Windows: []usage.LimitWindow{
							{Label: "Tokens/min", UsedPercent: 34, Used: fptr(6800), Limit: fptr(20000), Unit: "tokens", Blocked: true, ResetAt: tptr(testNow.Add(12 * time.Minute))},
						},
					},
				},
			},
		},
	}
}

func failureIsolationSnapshot() usage.Snapshot {
	return usage.Snapshot{
		TakenAt: testNow,
		Providers: []usage.ProviderResult{
			{
				Provider: usage.ProviderClaude, DisplayName: "Claude",
				Sources: []usage.SourceResult{
					{Source: usage.SourcePlan, Status: usage.StatusUnconfigured, Detail: usage.MsgUnconfigured("Claude credential", "run `claude` to log in")},
				},
			},
			{
				Provider: usage.ProviderCodex, DisplayName: "Codex",
				Sources: []usage.SourceResult{
					{Source: usage.SourcePlan, Status: usage.StatusUnauthorized, Detail: usage.MsgUnauthorized("codex")},
				},
			},
			{
				Provider: usage.ProviderKimi, DisplayName: "Kimi",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourceAPIKey, Status: usage.StatusOK,
						Windows: []usage.LimitWindow{
							{Label: "Tokens/min", UsedPercent: 41, Used: fptr(8200), Limit: fptr(20000), Unit: "tokens", ResetAt: tptr(testNow.Add(30 * time.Second))},
						},
					},
				},
			},
		},
	}
}

func allExhaustedSnapshot() usage.Snapshot {
	return usage.Snapshot{
		TakenAt: testNow,
		Providers: []usage.ProviderResult{
			{
				Provider: usage.ProviderClaude, DisplayName: "Claude",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourcePlan, Status: usage.StatusOK, Detail: "Max plan",
						Windows: []usage.LimitWindow{
							{Label: "5-hour", UsedPercent: 100, ResetAt: tptr(testNow.Add(time.Hour + 12*time.Minute))},
							{Label: "Weekly", UsedPercent: 104, ResetAt: tptr(testNow.Add(3 * time.Hour))},
						},
					},
				},
			},
			{
				Provider: usage.ProviderKimi, DisplayName: "Kimi",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourceAPIKey, Status: usage.StatusOK,
						Windows: []usage.LimitWindow{
							{Label: "Tokens/min", UsedPercent: 100, Used: fptr(20000), Limit: fptr(20000), Unit: "tokens", ResetAt: tptr(testNow.Add(2 * time.Hour))},
						},
					},
				},
			},
		},
	}
}

func unlimitedSnapshot() usage.Snapshot {
	return usage.Snapshot{
		TakenAt: testNow,
		Providers: []usage.ProviderResult{
			{
				Provider: usage.ProviderCodex, DisplayName: "Codex",
				Sources: []usage.SourceResult{
					{
						Source: usage.SourcePlan, Status: usage.StatusOK, Detail: "Business",
						Windows: []usage.LimitWindow{
							{Label: "Weekly", Unlimited: true},
							{Label: "5-hour", UsedPercent: 22, ResetAt: tptr(testNow.Add(58 * time.Minute))},
						},
					},
				},
			},
		},
	}
}

func TestRenderGolden(t *testing.T) {
	cases := []struct {
		name string
		snap usage.Snapshot
	}{
		{"all_healthy", allHealthySnapshot()},
		{"mixed_health", mixedHealthSnapshot()},
		{"failure_isolation", failureIsolationSnapshot()},
		{"all_exhausted", allExhaustedSnapshot()},
		{"unlimited_window", unlimitedSnapshot()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			golden.RequireEqual(t, []byte(Render(tc.snap, 80, testNow)))
		})
	}
}

func TestRenderRespectsWidth(t *testing.T) {
	for _, width := range []int{60, 40} {
		for name, snap := range map[string]usage.Snapshot{
			"all_healthy":       allHealthySnapshot(),
			"mixed_health":      mixedHealthSnapshot(),
			"failure_isolation": failureIsolationSnapshot(),
			"all_exhausted":     allExhaustedSnapshot(),
			"unlimited_window":  unlimitedSnapshot(),
		} {
			got := Render(snap, width, testNow)
			for i, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("width %d, %s: line %d is %d cells wide: %q", width, name, i, w, line)
				}
			}
		}
	}
}

func TestMarkersReadableWithoutColor(t *testing.T) {
	plain := ansi.Strip(Render(mixedHealthSnapshot(), 80, testNow))
	if !strings.Contains(plain, "⚠ WARN") {
		t.Errorf("warning marker missing from plain output:\n%s", plain)
	}
	if !strings.Contains(plain, "✗ FULL") {
		t.Errorf("exhausted marker missing from plain output:\n%s", plain)
	}
}

func TestRenderDeterministic(t *testing.T) {
	snap := mixedHealthSnapshot()
	first := Render(snap, 80, testNow)
	second := Render(snap, 80, testNow)
	if first != second {
		t.Errorf("Render is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestHumanize(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{-time.Minute, "0s"},
		{45 * time.Second, "45s"},
		{12 * time.Minute, "12m"},
		{2*time.Hour + 14*time.Minute, "2h 14m"},
		{2 * time.Hour, "2h"},
		{3*24*time.Hour + 4*time.Hour, "3d 4h"},
		{3 * 24 * time.Hour, "3d"},
	}
	for _, tc := range cases {
		if got := humanize(tc.d); got != tc.want {
			t.Errorf("humanize(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestFormatCount(t *testing.T) {
	cases := []struct {
		f    float64
		want string
	}{
		{0, "0"},
		{600, "600"},
		{5000, "5,000"},
		{12480, "12,480"},
		{1234567, "1,234,567"},
	}
	for _, tc := range cases {
		if got := formatCount(tc.f); got != tc.want {
			t.Errorf("formatCount(%v) = %q, want %q", tc.f, got, tc.want)
		}
	}
}

func TestRenderEmptySnapshot(t *testing.T) {
	if got := Render(usage.Snapshot{}, 80, testNow); got != "" {
		t.Errorf("Render on empty snapshot = %q, want empty", got)
	}
}
