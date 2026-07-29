package usage

import "time"

// Health derives the visual state of the window.
//
// A window is exhausted when the provider says it is blocked or when usage has
// reached 100%, warning at or above WarningThreshold, healthy otherwise. An
// unlimited window is never anything but healthy.
func (w LimitWindow) Health() Health {
	if w.Unlimited {
		return HealthHealthy
	}
	if w.Blocked || w.UsedPercent >= 100 {
		return HealthExhausted
	}
	if w.UsedPercent >= WarningThreshold {
		return HealthWarning
	}
	return HealthHealthy
}

// ResetIn reports how long until the window replenishes. It returns nil when
// the reset time is unknown, and zero when the reset time has already passed.
func (w LimitWindow) ResetIn(now time.Time) *time.Duration {
	if w.ResetAt == nil {
		return nil
	}
	d := w.ResetAt.Sub(now)
	if d < 0 {
		d = 0
	}
	return &d
}

// ExitCode derives the process exit status from the snapshot.
//
//	0 - every attempted source succeeded
//	1 - total failure: at least one source failed and none succeeded
//	3 - partial failure: at least one succeeded and at least one failed
//
// Unconfigured and unsupported sources are informational and never influence
// the result.
func (s Snapshot) ExitCode() int {
	var ok, failed int
	for _, p := range s.Providers {
		for _, src := range p.Sources {
			switch {
			case src.Status == StatusOK:
				ok++
			case src.Status.IsFailure():
				failed++
			}
		}
	}
	switch {
	case failed == 0:
		return 0
	case ok == 0:
		return 1
	default:
		return 3
	}
}

// EarliestReset returns the soonest upcoming reset across every window, or nil
// when no window reports one. It is used to answer "when can I work again?"
// when everything is exhausted.
func (s Snapshot) EarliestReset() *time.Time {
	var earliest *time.Time
	for _, p := range s.Providers {
		for _, src := range p.Sources {
			for _, w := range src.Windows {
				if w.ResetAt == nil {
					continue
				}
				if earliest == nil || w.ResetAt.Before(*earliest) {
					t := *w.ResetAt
					earliest = &t
				}
			}
		}
	}
	return earliest
}
