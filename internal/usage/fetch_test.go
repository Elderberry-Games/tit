package usage

import (
	"context"
	"testing"
	"time"
)

// fakeProvider is a Provider whose behavior is fully scripted by tests.
type fakeProvider struct {
	id      ProviderID
	name    string
	delay   time.Duration
	sources []SourceResult
}

func (f fakeProvider) ID() ProviderID                 { return f.id }
func (f fakeProvider) DisplayName() string            { return f.name }
func (f fakeProvider) MinPollInterval() time.Duration { return 0 }

func (f fakeProvider) Fetch(ctx context.Context, _ Clock) []SourceResult {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return []SourceResult{{
				Source: SourcePlan,
				Status: StatusUnavailable,
				Detail: MsgUnreachable("timed out"),
			}}
		}
	}
	return f.sources
}

func okSource(kind SourceKind) SourceResult {
	return SourceResult{Source: kind, Status: StatusOK, Windows: []LimitWindow{{Label: "5-hour", UsedPercent: 10}}}
}

func TestFetchAllSucceed(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	providers := []Provider{
		fakeProvider{id: ProviderClaude, name: "Claude", sources: []SourceResult{okSource(SourcePlan)}},
		fakeProvider{id: ProviderCodex, name: "Codex", sources: []SourceResult{okSource(SourcePlan)}},
	}

	got := Fetch(context.Background(), providers, FixedClock(now), DefaultTimeout)

	if !got.TakenAt.Equal(now) {
		t.Errorf("TakenAt = %v, want %v", got.TakenAt, now)
	}
	if len(got.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(got.Providers))
	}
	if got.ExitCode() != 0 {
		t.Errorf("ExitCode() = %d, want 0", got.ExitCode())
	}
}

func TestFetchPreservesProviderOrder(t *testing.T) {
	// The slow provider is listed first: results must still come back in the
	// order given, not in completion order.
	providers := []Provider{
		fakeProvider{id: ProviderClaude, name: "Claude", delay: 40 * time.Millisecond, sources: []SourceResult{okSource(SourcePlan)}},
		fakeProvider{id: ProviderCodex, name: "Codex", sources: []SourceResult{okSource(SourcePlan)}},
		fakeProvider{id: ProviderKimi, name: "Kimi", sources: []SourceResult{okSource(SourcePlan)}},
	}

	got := Fetch(context.Background(), providers, nil, DefaultTimeout)

	want := []ProviderID{ProviderClaude, ProviderCodex, ProviderKimi}
	for i, id := range want {
		if got.Providers[i].Provider != id {
			t.Errorf("Providers[%d] = %q, want %q", i, got.Providers[i].Provider, id)
		}
	}
}

func TestFetchPartialFailureIsolatesProviders(t *testing.T) {
	providers := []Provider{
		fakeProvider{id: ProviderClaude, name: "Claude", sources: []SourceResult{okSource(SourcePlan)}},
		fakeProvider{id: ProviderCodex, name: "Codex", sources: []SourceResult{{
			Source: SourcePlan, Status: StatusUnavailable, Detail: MsgUnreachable("connection refused"),
		}}},
	}

	got := Fetch(context.Background(), providers, nil, DefaultTimeout)

	if code := got.ExitCode(); code != 3 {
		t.Errorf("ExitCode() = %d, want 3 (partial failure)", code)
	}
	// The healthy provider must still carry its data.
	if len(got.Providers[0].Sources[0].Windows) != 1 {
		t.Error("healthy provider lost its windows when another provider failed")
	}
}

func TestFetchTotalFailure(t *testing.T) {
	providers := []Provider{
		fakeProvider{id: ProviderClaude, name: "Claude", sources: []SourceResult{{Source: SourcePlan, Status: StatusUnauthorized}}},
		fakeProvider{id: ProviderCodex, name: "Codex", sources: []SourceResult{{Source: SourcePlan, Status: StatusUnavailable}}},
	}

	if code := Fetch(context.Background(), providers, nil, DefaultTimeout).ExitCode(); code != 1 {
		t.Errorf("ExitCode() = %d, want 1 (total failure)", code)
	}
}

func TestFetchUnconfiguredIsNotAFailure(t *testing.T) {
	providers := []Provider{
		fakeProvider{id: ProviderKimi, name: "Kimi", sources: []SourceResult{{
			Source: SourcePlan, Status: StatusUnconfigured, Detail: MsgUnconfigured("Kimi credential", "run `kimi` to log in"),
		}}},
	}

	if code := Fetch(context.Background(), providers, nil, DefaultTimeout).ExitCode(); code != 0 {
		t.Errorf("ExitCode() = %d, want 0 (unconfigured is informational)", code)
	}
}

func TestFetchTimesOutSlowProvider(t *testing.T) {
	providers := []Provider{
		fakeProvider{id: ProviderClaude, name: "Claude", delay: time.Hour, sources: []SourceResult{okSource(SourcePlan)}},
		fakeProvider{id: ProviderCodex, name: "Codex", sources: []SourceResult{okSource(SourcePlan)}},
	}

	start := time.Now()
	got := Fetch(context.Background(), providers, nil, 50*time.Millisecond)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("Fetch took %v, expected it to abandon the slow provider quickly", elapsed)
	}
	if status := got.Providers[0].Sources[0].Status; status != StatusUnavailable {
		t.Errorf("slow provider status = %q, want %q", status, StatusUnavailable)
	}
	if code := got.ExitCode(); code != 3 {
		t.Errorf("ExitCode() = %d, want 3", code)
	}
}

func TestFetchSortsPlanSourceFirst(t *testing.T) {
	providers := []Provider{
		fakeProvider{id: ProviderClaude, name: "Claude", sources: []SourceResult{
			okSource(SourceAPIKey),
			okSource(SourcePlan),
		}},
	}

	got := Fetch(context.Background(), providers, nil, DefaultTimeout)

	if got.Providers[0].Sources[0].Source != SourcePlan {
		t.Errorf("first source = %q, want %q", got.Providers[0].Sources[0].Source, SourcePlan)
	}
}

func TestSnapshotFilter(t *testing.T) {
	s := Snapshot{Providers: []ProviderResult{
		{Provider: ProviderClaude, DisplayName: "Claude"},
		{Provider: ProviderCodex, DisplayName: "Codex"},
	}}

	got, found := s.Filter(ProviderCodex)
	if !found {
		t.Fatal("Filter() found = false, want true")
	}
	if len(got.Providers) != 1 || got.Providers[0].Provider != ProviderCodex {
		t.Errorf("Filter() = %+v, want only Codex", got.Providers)
	}

	if _, found := s.Filter(ProviderID("nope")); found {
		t.Error("Filter() found = true for unknown provider, want false")
	}
}

func TestFetchDefaultsClockAndTimeout(t *testing.T) {
	providers := []Provider{fakeProvider{id: ProviderKimi, name: "Kimi", sources: []SourceResult{okSource(SourcePlan)}}}

	got := Fetch(context.Background(), providers, nil, 0)

	if got.TakenAt.IsZero() {
		t.Error("TakenAt is zero, expected the system clock to be used")
	}
}
