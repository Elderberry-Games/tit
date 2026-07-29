package usage

import (
	"testing"
	"time"
)

func ptr(f float64) *float64 { return &f }

func TestLimitWindowHealth(t *testing.T) {
	tests := []struct {
		name string
		w    LimitWindow
		want Health
	}{
		{"zero usage is healthy", LimitWindow{UsedPercent: 0}, HealthHealthy},
		{"just below threshold is healthy", LimitWindow{UsedPercent: 89.9}, HealthHealthy},
		{"exactly at threshold warns", LimitWindow{UsedPercent: 90}, HealthWarning},
		{"above threshold warns", LimitWindow{UsedPercent: 99.9}, HealthWarning},
		{"full is exhausted", LimitWindow{UsedPercent: 100}, HealthExhausted},
		{"over full is exhausted", LimitWindow{UsedPercent: 137}, HealthExhausted},
		{"blocked is exhausted regardless of percent", LimitWindow{UsedPercent: 3, Blocked: true}, HealthExhausted},
		{"unlimited is always healthy", LimitWindow{UsedPercent: 100, Unlimited: true}, HealthHealthy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.Health(); got != tt.want {
				t.Errorf("Health() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLimitWindowResetIn(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	t.Run("nil reset time yields nil", func(t *testing.T) {
		if got := (LimitWindow{}).ResetIn(now); got != nil {
			t.Errorf("ResetIn() = %v, want nil", got)
		}
	})

	t.Run("future reset yields remaining duration", func(t *testing.T) {
		future := now.Add(90 * time.Minute)
		got := LimitWindow{ResetAt: &future}.ResetIn(now)
		if got == nil || *got != 90*time.Minute {
			t.Errorf("ResetIn() = %v, want 1h30m0s", got)
		}
	})

	t.Run("past reset clamps to zero", func(t *testing.T) {
		past := now.Add(-5 * time.Minute)
		got := LimitWindow{ResetAt: &past}.ResetIn(now)
		if got == nil || *got != 0 {
			t.Errorf("ResetIn() = %v, want 0s", got)
		}
	})
}

func TestSnapshotExitCode(t *testing.T) {
	snap := func(statuses ...SourceStatus) Snapshot {
		var sources []SourceResult
		for _, st := range statuses {
			sources = append(sources, SourceResult{Status: st})
		}
		return Snapshot{Providers: []ProviderResult{{Sources: sources}}}
	}

	tests := []struct {
		name string
		s    Snapshot
		want int
	}{
		{"all ok", snap(StatusOK, StatusOK), 0},
		{"no sources at all", snap(), 0},
		{"unconfigured only is not a failure", snap(StatusUnconfigured, StatusUnconfigured), 0},
		{"unsupported only is not a failure", snap(StatusUnsupported), 0},
		{"ok plus unconfigured", snap(StatusOK, StatusUnconfigured), 0},
		{"total failure via unauthorized", snap(StatusUnauthorized), 1},
		{"total failure via unavailable", snap(StatusUnavailable, StatusUnavailable), 1},
		{"failure with unconfigured is still total", snap(StatusUnavailable, StatusUnconfigured), 1},
		{"partial failure", snap(StatusOK, StatusUnavailable), 3},
		{"partial failure via unauthorized", snap(StatusOK, StatusUnauthorized), 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.ExitCode(); got != tt.want {
				t.Errorf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSnapshotEarliestReset(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	soon := base.Add(time.Hour)
	later := base.Add(10 * time.Hour)

	s := Snapshot{Providers: []ProviderResult{
		{Sources: []SourceResult{{Windows: []LimitWindow{{ResetAt: &later}, {ResetAt: nil}}}}},
		{Sources: []SourceResult{{Windows: []LimitWindow{{ResetAt: &soon}}}}},
	}}

	got := s.EarliestReset()
	if got == nil || !got.Equal(soon) {
		t.Errorf("EarliestReset() = %v, want %v", got, soon)
	}

	if got := (Snapshot{}).EarliestReset(); got != nil {
		t.Errorf("EarliestReset() on empty snapshot = %v, want nil", got)
	}
}

func TestSourceStatusIsFailure(t *testing.T) {
	failures := []SourceStatus{StatusUnauthorized, StatusUnavailable}
	neutral := []SourceStatus{StatusOK, StatusUnconfigured, StatusUnsupported}

	for _, st := range failures {
		if !st.IsFailure() {
			t.Errorf("%q.IsFailure() = false, want true", st)
		}
	}
	for _, st := range neutral {
		if st.IsFailure() {
			t.Errorf("%q.IsFailure() = true, want false", st)
		}
	}
}
