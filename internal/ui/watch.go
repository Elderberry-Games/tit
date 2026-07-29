package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Elderberry-Games/tit/internal/usage"
)

// watchHintStyle dims the key hints so they never compete with the data.
var watchHintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

// baseRefresh is how often watch mode reconsiders fetching. Individual
// providers can demand a longer gap via MinPollInterval — Claude's usage
// endpoint throttles hard below three minutes — so this is a floor, not a
// promise.
const baseRefresh = 60 * time.Second

// tickMsg drives the once-per-second countdown redraw.
type tickMsg time.Time

// dataMsg carries the results of a refresh for a subset of providers.
type dataMsg struct {
	at      time.Time
	results []usage.ProviderResult
}

type watchModel struct {
	providers []usage.Provider
	// lastFetch records when each provider was last queried, so that a provider
	// with a long MinPollInterval is not refetched on every cycle.
	lastFetch map[usage.ProviderID]time.Time
	snapshot  usage.Snapshot
	width     int
	now       usage.Clock
	fetch     fetchFunc
}

// fetchFunc is the seam that lets tests drive the model without a network.
type fetchFunc func(context.Context, []usage.Provider, usage.Clock, time.Duration) usage.Snapshot

func newWatchModel(providers []usage.Provider, now usage.Clock, fetch fetchFunc) *watchModel {
	if now == nil {
		now = usage.SystemClock
	}
	if fetch == nil {
		fetch = usage.Fetch
	}

	results := make([]usage.ProviderResult, len(providers))
	for i, p := range providers {
		results[i] = usage.ProviderResult{Provider: p.ID(), DisplayName: p.DisplayName()}
	}

	return &watchModel{
		providers: providers,
		lastFetch: make(map[usage.ProviderID]time.Time, len(providers)),
		snapshot:  usage.Snapshot{TakenAt: now(), Providers: results},
		width:     80,
		now:       now,
		fetch:     fetch,
	}
}

// Watch runs the live view until the user quits.
func Watch(providers []usage.Provider) error {
	m := newWatchModel(providers, usage.SystemClock, usage.Fetch)
	_, err := tea.NewProgram(m).Run()
	return err
}

func (m *watchModel) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// due reports the providers whose minimum poll interval has elapsed.
func (m *watchModel) due() []usage.Provider {
	now := m.now()
	var out []usage.Provider

	for _, p := range m.providers {
		gap := baseRefresh
		if min := p.MinPollInterval(); min > gap {
			gap = min
		}
		last, seen := m.lastFetch[p.ID()]
		if !seen || now.Sub(last) >= gap {
			out = append(out, p)
		}
	}
	return out
}

func (m *watchModel) refreshCmd() tea.Cmd {
	due := m.due()
	if len(due) == 0 {
		return nil
	}

	now := m.now
	fetch := m.fetch

	return func() tea.Msg {
		snap := fetch(context.Background(), due, now, usage.DefaultTimeout)
		return dataMsg{at: now(), results: snap.Providers}
	}
}

// merge folds freshly fetched providers into the existing snapshot, leaving
// providers that were not refetched untouched.
func (m *watchModel) merge(msg dataMsg) {
	for _, fresh := range msg.results {
		for i, existing := range m.snapshot.Providers {
			if existing.Provider == fresh.Provider {
				m.snapshot.Providers[i] = fresh
				break
			}
		}
		m.lastFetch[fresh.Provider] = msg.at
	}
	m.snapshot.TakenAt = msg.at
}

func (m *watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "r":
			// Manual refresh ignores the poll schedule for providers that allow
			// it; those that do not are simply not due yet.
			return m, m.refreshCmd()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tickMsg:
		return m, tea.Batch(tickCmd(), m.refreshCmd())

	case dataMsg:
		m.merge(msg)
	}

	return m, nil
}

func (m *watchModel) View() tea.View {
	body := Render(m.snapshot, m.width, m.now())
	body += "\n" + watchHintStyle.Render("q quit · r refresh")

	v := tea.NewView(body)
	v.AltScreen = true
	return v
}
