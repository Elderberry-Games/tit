package ui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Elderberry-Games/tit/internal/usage"
)

// stubProvider satisfies usage.Provider without touching the network.
type stubProvider struct {
	id       usage.ProviderID
	name     string
	minPoll  time.Duration
	fetchNum int
}

func (s *stubProvider) ID() usage.ProviderID           { return s.id }
func (s *stubProvider) DisplayName() string            { return s.name }
func (s *stubProvider) MinPollInterval() time.Duration { return s.minPoll }

func (s *stubProvider) Fetch(context.Context, usage.Clock) []usage.SourceResult {
	s.fetchNum++
	return []usage.SourceResult{{Source: usage.SourcePlan, Status: usage.StatusOK}}
}

// clockAt returns a Clock backed by a pointer so tests can advance time.
func clockAt(t *time.Time) usage.Clock {
	return func() time.Time { return *t }
}

func recordingFetch(calls *[][]usage.ProviderID) fetchFunc {
	return func(_ context.Context, providers []usage.Provider, now usage.Clock, _ time.Duration) usage.Snapshot {
		var ids []usage.ProviderID
		results := make([]usage.ProviderResult, 0, len(providers))
		for _, p := range providers {
			ids = append(ids, p.ID())
			results = append(results, usage.ProviderResult{
				Provider:    p.ID(),
				DisplayName: p.DisplayName(),
				Sources:     []usage.SourceResult{{Source: usage.SourcePlan, Status: usage.StatusOK}},
			})
		}
		*calls = append(*calls, ids)
		return usage.Snapshot{TakenAt: now(), Providers: results}
	}
}

func TestWatchQuitsOnKeys(t *testing.T) {
	keys := map[string]tea.KeyPressMsg{
		"q":      {Code: 'q', Text: "q"},
		"esc":    {Code: tea.KeyEscape},
		"ctrl+c": {Code: 'c', Mod: tea.ModCtrl},
	}

	for name, key := range keys {
		t.Run(name, func(t *testing.T) {
			now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
			m := newWatchModel(nil, clockAt(&now), nil)

			_, cmd := m.Update(key)
			if cmd == nil {
				t.Fatalf("key %q produced no command, expected quit", name)
			}
			if msg := cmd(); msg == nil {
				t.Fatalf("key %q did not produce a quit message", name)
			}
		})
	}
}

func TestWatchManualRefreshKey(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	var calls [][]usage.ProviderID
	providers := []usage.Provider{&stubProvider{id: usage.ProviderKimi, name: "Kimi"}}
	m := newWatchModel(providers, clockAt(&now), recordingFetch(&calls))

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("r produced no refresh command")
	}
	cmd()
	if len(calls) != 1 {
		t.Errorf("manual refresh triggered %d fetches, want 1", len(calls))
	}
}

func TestWatchResizeUpdatesWidth(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	m := newWatchModel(nil, clockAt(&now), nil)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
}

func TestWatchRespectsMinPollInterval(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	var calls [][]usage.ProviderID

	providers := []usage.Provider{
		&stubProvider{id: usage.ProviderClaude, name: "Claude", minPoll: 180 * time.Second},
		&stubProvider{id: usage.ProviderCodex, name: "Codex", minPoll: 60 * time.Second},
	}
	m := newWatchModel(providers, clockAt(&now), recordingFetch(&calls))

	// First refresh: nothing has been fetched yet, so both are due.
	if cmd := m.refreshCmd(); cmd != nil {
		if msg, ok := cmd().(dataMsg); ok {
			m.merge(msg)
		}
	}
	if len(calls) != 1 || len(calls[0]) != 2 {
		t.Fatalf("first refresh fetched %v, want both providers", calls)
	}

	// 90 seconds later only Codex is due; Claude must be left alone because its
	// endpoint throttles below three minutes.
	now = now.Add(90 * time.Second)
	if cmd := m.refreshCmd(); cmd != nil {
		if msg, ok := cmd().(dataMsg); ok {
			m.merge(msg)
		}
	}
	if len(calls) != 2 {
		t.Fatalf("expected a second refresh, got %d calls", len(calls))
	}
	if len(calls[1]) != 1 || calls[1][0] != usage.ProviderCodex {
		t.Errorf("second refresh fetched %v, want only codex", calls[1])
	}

	// Past Claude's floor, both are due again.
	now = now.Add(120 * time.Second)
	if cmd := m.refreshCmd(); cmd != nil {
		cmd()
	}
	if len(calls) != 3 || len(calls[2]) != 2 {
		t.Errorf("third refresh fetched %v, want both providers", calls)
	}
}

func TestWatchRefreshCmdIsNilWhenNothingDue(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	var calls [][]usage.ProviderID

	providers := []usage.Provider{&stubProvider{id: usage.ProviderKimi, name: "Kimi", minPoll: 60 * time.Second}}
	m := newWatchModel(providers, clockAt(&now), recordingFetch(&calls))

	if cmd := m.refreshCmd(); cmd != nil {
		if msg, ok := cmd().(dataMsg); ok {
			m.merge(msg)
		}
	}

	now = now.Add(5 * time.Second)
	if cmd := m.refreshCmd(); cmd != nil {
		t.Error("refreshCmd() returned a command while the provider was not due")
	}
}

func TestWatchTickKeepsTicking(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	m := newWatchModel(nil, clockAt(&now), nil)

	_, cmd := m.Update(tickMsg(now))
	if cmd == nil {
		t.Fatal("tick produced no follow-up command; the countdown would stop")
	}
}

func TestWatchMergePreservesUnfetchedProviders(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	providers := []usage.Provider{
		&stubProvider{id: usage.ProviderClaude, name: "Claude"},
		&stubProvider{id: usage.ProviderCodex, name: "Codex"},
	}
	m := newWatchModel(providers, clockAt(&now), nil)

	// Seed Claude with data, then merge a refresh that only covers Codex.
	m.snapshot.Providers[0].Sources = []usage.SourceResult{{Source: usage.SourcePlan, Status: usage.StatusOK}}
	m.merge(dataMsg{at: now, results: []usage.ProviderResult{
		{Provider: usage.ProviderCodex, DisplayName: "Codex", Sources: []usage.SourceResult{{Status: usage.StatusUnavailable}}},
	}})

	if len(m.snapshot.Providers[0].Sources) != 1 {
		t.Error("Claude lost its data when only Codex was refreshed")
	}
	if m.snapshot.Providers[1].Sources[0].Status != usage.StatusUnavailable {
		t.Error("Codex was not updated by the merge")
	}
}

func TestWatchInitStartsFetchAndTick(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	var calls [][]usage.ProviderID
	providers := []usage.Provider{&stubProvider{id: usage.ProviderKimi, name: "Kimi"}}

	m := newWatchModel(providers, clockAt(&now), recordingFetch(&calls))

	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init() returned no command")
	}
}

func TestNewWatchModelDefaults(t *testing.T) {
	m := newWatchModel(nil, nil, nil)

	if m.now == nil {
		t.Error("clock was not defaulted")
	}
	if m.fetch == nil {
		t.Error("fetch function was not defaulted")
	}
	if m.width != 80 {
		t.Errorf("width = %d, want 80", m.width)
	}
}
