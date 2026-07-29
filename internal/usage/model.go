// Package usage defines the normalized model that every provider adapter
// produces and that the renderer and JSON encoder consume.
package usage

import (
	"context"
	"time"
)

// WarningThreshold is the percentage consumed at which a window stops being
// healthy and starts warning.
const WarningThreshold = 90.0

// ProviderID is the canonical lowercase identifier of a provider. It doubles as
// the value accepted by the `tit usage <provider>` filter.
type ProviderID string

const (
	ProviderClaude ProviderID = "claude"
	ProviderCodex  ProviderID = "codex"
	ProviderKimi   ProviderID = "kimi"
)

// SourceKind distinguishes the two limit sources a provider can expose.
type SourceKind string

const (
	// SourcePlan is a subscription/coding-plan window read via the harness CLI's
	// cached OAuth credentials.
	SourcePlan SourceKind = "plan"
	// SourceAPIKey is a developer API-key rate limit.
	SourceAPIKey SourceKind = "apikey"
)

// SourceStatus is the outcome of fetching one source.
type SourceStatus string

const (
	// StatusOK means the fetch succeeded and Windows is populated.
	StatusOK SourceStatus = "ok"
	// StatusUnconfigured means no credential was found. Informational: it never
	// counts as a failure for exit-code purposes.
	StatusUnconfigured SourceStatus = "unconfigured"
	// StatusUnauthorized means a credential was found but rejected or expired.
	StatusUnauthorized SourceStatus = "unauthorized"
	// StatusUnavailable means a network, timeout, server, or decoding failure.
	StatusUnavailable SourceStatus = "unavailable"
	// StatusUnsupported means the source exists but exposes no window data.
	// Informational: Info may still be populated.
	StatusUnsupported SourceStatus = "unsupported"
)

// IsFailure reports whether the status counts against the exit code.
func (s SourceStatus) IsFailure() bool {
	return s == StatusUnauthorized || s == StatusUnavailable
}

// Health is the derived visual state of a limit window.
type Health string

const (
	HealthHealthy   Health = "healthy"
	HealthWarning   Health = "warning"
	HealthExhausted Health = "exhausted"
)

// InfoItem is a non-window fact worth showing, such as an account balance.
type InfoItem struct {
	Label string
	Value string
}

// LimitWindow is one normalized rate-limit bucket.
//
// Providers differ in what they can report: Claude and Codex plan windows are
// percent-only (Used and Limit are nil), while Kimi and API-key sources report
// absolute counts. Consumers must handle both.
type LimitWindow struct {
	// Label is the human name, e.g. "5-hour", "Weekly", "Weekly (Opus)".
	Label string
	// Duration is the window length; zero when the provider does not state it.
	Duration time.Duration
	// UsedPercent is 0-100 (may exceed 100). Always populated; adapters compute
	// it from counts when the provider reports only absolutes.
	UsedPercent float64
	// Used and Limit are absolute counts, nil when unknown.
	Used  *float64
	Limit *float64
	// Unit describes the counts, e.g. "requests", "tokens". Empty when
	// percent-only.
	Unit string
	// Unlimited marks a window with no ceiling. Renders as "unlimited".
	Unlimited bool
	// Blocked is set when the provider explicitly reports the window as
	// exhausted regardless of the reported percentage.
	Blocked bool
	// ResetAt is when the window replenishes; nil when unknown.
	ResetAt *time.Time
}

// SourceResult is one limit source for one provider.
type SourceResult struct {
	Source SourceKind
	Status SourceStatus
	// Detail is a human-readable hint: a plan name when healthy, or a reason
	// plus next step when not. It must never contain credential material.
	Detail  string
	Windows []LimitWindow
	Info    []InfoItem
}

// ProviderResult is everything known about one provider in a single run.
type ProviderResult struct {
	Provider    ProviderID
	DisplayName string
	Sources     []SourceResult
}

// Snapshot is the complete result of one `tit usage` run.
type Snapshot struct {
	TakenAt   time.Time
	Providers []ProviderResult
}

// Clock returns the current time. Injected so tests are deterministic.
type Clock func() time.Time

// SystemClock is the default production clock.
func SystemClock() time.Time { return time.Now() }

// FixedClock returns a Clock that always reports t.
func FixedClock(t time.Time) Clock {
	return func() time.Time { return t }
}

// Provider is implemented by each provider adapter.
//
// Fetch must never return an error: every failure mode is expressed as a
// SourceResult with a non-OK status, so that one provider failing can never
// prevent another from rendering.
type Provider interface {
	ID() ProviderID
	DisplayName() string
	Fetch(ctx context.Context, now Clock) []SourceResult
	// MinPollInterval is the shortest safe interval between fetches, honored by
	// watch mode. Zero means no constraint beyond the default refresh.
	MinPollInterval() time.Duration
}
