package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Elderberry-Games/tit/internal/usage"
)

// scriptedProvider returns a fixed set of sources without touching the network.
type scriptedProvider struct {
	id      usage.ProviderID
	name    string
	sources []usage.SourceResult
}

func (s scriptedProvider) ID() usage.ProviderID           { return s.id }
func (s scriptedProvider) DisplayName() string            { return s.name }
func (s scriptedProvider) MinPollInterval() time.Duration { return time.Minute }
func (s scriptedProvider) Fetch(context.Context, usage.Clock) []usage.SourceResult {
	return s.sources
}

// useProviders swaps in stub providers for the duration of the test.
func useProviders(t *testing.T, providers ...usage.Provider) {
	t.Helper()

	origAll, origLookup := allProviders, lookupProvider
	t.Cleanup(func() { allProviders, lookupProvider = origAll, origLookup })

	allProviders = func() []usage.Provider { return providers }
	lookupProvider = func(name string) (usage.Provider, bool) {
		for _, p := range providers {
			if string(p.ID()) == strings.ToLower(strings.TrimSpace(name)) {
				return p, true
			}
		}
		return nil, false
	}
}

func okProvider(id usage.ProviderID, name string) scriptedProvider {
	reset := time.Now().Add(90 * time.Minute)
	return scriptedProvider{id: id, name: name, sources: []usage.SourceResult{{
		Source:  usage.SourcePlan,
		Status:  usage.StatusOK,
		Detail:  "test plan",
		Windows: []usage.LimitWindow{{Label: "5-hour", Duration: 5 * time.Hour, UsedPercent: 40, ResetAt: &reset}},
	}}}
}

func failingProvider(id usage.ProviderID, name string) scriptedProvider {
	return scriptedProvider{id: id, name: name, sources: []usage.SourceResult{{
		Source: usage.SourcePlan,
		Status: usage.StatusUnavailable,
		Detail: usage.MsgUnreachable("connection refused"),
	}}}
}

func TestUsageRendersDashboard(t *testing.T) {
	useProviders(t, okProvider(usage.ProviderClaude, "Claude"))

	code, stdout, stderr := exec(t, "usage")

	if code != exitOK {
		t.Errorf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}
	for _, want := range []string{"Claude", "5-hour", "40%"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("dashboard missing %q:\n%s", want, stdout)
		}
	}
}

func TestUsageExitCodeReflectsPartialFailure(t *testing.T) {
	useProviders(t,
		okProvider(usage.ProviderClaude, "Claude"),
		failingProvider(usage.ProviderCodex, "Codex"),
	)

	code, stdout, _ := exec(t, "usage")

	if code != 3 {
		t.Errorf("exit code = %d, want 3 for a partial failure", code)
	}
	// The healthy provider must still be visible: one provider failing can
	// never hide another.
	if !strings.Contains(stdout, "Claude") || !strings.Contains(stdout, "5-hour") {
		t.Errorf("healthy provider was hidden by the failing one:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Codex") {
		t.Errorf("failing provider was omitted entirely:\n%s", stdout)
	}
}

func TestUsageExitCodeReflectsTotalFailure(t *testing.T) {
	useProviders(t, failingProvider(usage.ProviderClaude, "Claude"))

	if code, _, _ := exec(t, "usage"); code != exitFailure {
		t.Errorf("exit code = %d, want %d for a total failure", code, exitFailure)
	}
}

func TestUsageFilterSelectsOneProvider(t *testing.T) {
	useProviders(t,
		okProvider(usage.ProviderClaude, "Claude"),
		okProvider(usage.ProviderCodex, "Codex"),
	)

	code, stdout, _ := exec(t, "usage", "CODEX")

	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout, "Codex") {
		t.Errorf("filtered output missing the requested provider:\n%s", stdout)
	}
	if strings.Contains(stdout, "Claude") {
		t.Errorf("filtered output leaked another provider:\n%s", stdout)
	}
}

func TestUsageJSONIsValidAndComplete(t *testing.T) {
	useProviders(t, okProvider(usage.ProviderClaude, "Claude"))

	code, stdout, _ := exec(t, "usage", "--json")

	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, stdout)
	}

	providers, ok := got["providers"].([]any)
	if !ok || len(providers) != 1 {
		t.Fatalf("providers = %v, want one entry", got["providers"])
	}

	window := providers[0].(map[string]any)["sources"].([]any)[0].(map[string]any)["windows"].([]any)[0].(map[string]any)
	if window["used_percent"].(float64) != 40 {
		t.Errorf("used_percent = %v, want 40", window["used_percent"])
	}
	if window["health"].(string) != string(usage.HealthHealthy) {
		t.Errorf("health = %v, want healthy", window["health"])
	}
}

func TestUsageJSONCarriesNoStyling(t *testing.T) {
	useProviders(t, okProvider(usage.ProviderClaude, "Claude"))

	_, stdout, _ := exec(t, "usage", "--json")

	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("JSON output contains ANSI escapes:\n%q", stdout)
	}
}

func TestDashboardToNonTerminalHasNoANSI(t *testing.T) {
	useProviders(t, okProvider(usage.ProviderClaude, "Claude"))

	// The test writer is a buffer, never a TTY, so styling must be stripped
	// while every figure survives.
	_, stdout, _ := exec(t, "usage")

	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("piped dashboard contains ANSI escapes:\n%q", stdout)
	}
	if !strings.Contains(stdout, "40%") {
		t.Errorf("piped dashboard lost its data:\n%s", stdout)
	}
}

func TestWidthOfFallsBackWhenNotATerminal(t *testing.T) {
	if got := widthOf(&bytes.Buffer{}); got != defaultWidth {
		t.Errorf("widthOf(non-file) = %d, want %d", got, defaultWidth)
	}
}

func TestIsTerminalRejectsNonFiles(t *testing.T) {
	if isTerminal(&bytes.Buffer{}) {
		t.Error("isTerminal(buffer) = true, want false")
	}
}
