package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/Elderberry-Games/tit/internal/usage"
)

// A provider whose plan source works but carries no plan name, followed by an
// unconfigured API-key source, must not advertise itself as "not configured".
// Doing so contradicts the live numbers printed directly underneath.
func TestProviderHeaderIgnoresFailingSourcesWhenAnotherSucceeded(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	reset := now.Add(2 * time.Hour)

	snapshot := usage.Snapshot{TakenAt: now, Providers: []usage.ProviderResult{{
		Provider:    usage.ProviderClaude,
		DisplayName: "Claude",
		Sources: []usage.SourceResult{
			{
				Source:  usage.SourcePlan,
				Status:  usage.StatusOK,
				Windows: []usage.LimitWindow{{Label: "5-hour", UsedPercent: 22, ResetAt: &reset}},
			},
			{
				Source: usage.SourceAPIKey,
				Status: usage.StatusUnconfigured,
				Detail: usage.MsgUnconfigured("ANTHROPIC_API_KEY", "set it to see API-key rate limits"),
			},
		},
	}}}

	got := ansi.Strip(Render(snapshot, 80, now))
	header := strings.SplitN(got, "\n", 2)[0]

	if strings.Contains(header, "not configured") {
		t.Errorf("header claims the provider is unconfigured while showing its data:\n%s", got)
	}
	if !strings.Contains(got, "22%") {
		t.Errorf("the working source's data disappeared:\n%s", got)
	}
	// The unconfigured source should still be explained, just not in the header.
	if !strings.Contains(got, "ANTHROPIC_API_KEY") {
		t.Errorf("the unconfigured source was silently dropped:\n%s", got)
	}
}

// When nothing works, the header should say so.
func TestProviderHeaderReportsStatusWhenEverythingFailed(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	snapshot := usage.Snapshot{TakenAt: now, Providers: []usage.ProviderResult{{
		Provider:    usage.ProviderCodex,
		DisplayName: "Codex",
		Sources: []usage.SourceResult{{
			Source: usage.SourcePlan,
			Status: usage.StatusUnconfigured,
			Detail: usage.MsgUnconfigured("Codex credential", "run `codex` to log in"),
		}},
	}}}

	got := ansi.Strip(Render(snapshot, 80, now))

	if !strings.Contains(got, "not configured") {
		t.Errorf("header does not report the provider as unconfigured:\n%s", got)
	}
}
