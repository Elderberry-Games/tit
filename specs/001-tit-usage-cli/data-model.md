# Data Model: tit — Unified AI Usage-Limit Monitor

**Date**: 2026-07-29 | **Spec**: [spec.md](spec.md) | **Research**: [research.md](research.md)

Core principle: every provider adapter normalizes its wildly different payloads (percent-only,
absolute counts, string-numbers) into the same small set of types below. The renderer and JSON
encoder consume only these types and never see provider-specific shapes.

## Entities

### Snapshot

One `tit usage` run's complete result (spec: Usage Snapshot).

| Field | Type | Notes |
|---|---|---|
| `TakenAt` | `time.Time` | Injected clock (`func() time.Time`) for testability |
| `Providers` | `[]ProviderResult` | Fixed order: Claude, Codex, Kimi (stable output) |

Derived: `ExitCode()` per research R7 (0/1/3; unconfigured ignored). `EarliestReset()` for the
all-exhausted edge case (spec Edge Cases).

### ProviderResult

| Field | Type | Notes |
|---|---|---|
| `Provider` | `ProviderID` | `claude` \| `codex` \| `kimi` (canonical lowercase, also the filter argument) |
| `DisplayName` | `string` | "Claude", "Codex", "Kimi" |
| `Sources` | `[]SourceResult` | 0–2 entries: plan, apikey |

A provider with zero configured sources renders as "unconfigured" with per-source hints
(FR-007, US2 scenario 3).

### SourceResult

One limit source (plan subscription or API key) for one provider (clarification 2026-07-29:
both sources, merged, labeled).

| Field | Type | Notes |
|---|---|---|
| `Source` | `SourceKind` | `plan` \| `apikey` |
| `Status` | `SourceStatus` | see state machine below |
| `Detail` | `string` | Human hint: plan name ("Max", "plus", "Allegretto"), or failure reason + fix ("token expired — run `codex` to re-login") |
| `Windows` | `[]LimitWindow` | Empty unless `Status == ok` |
| `Info` | `[]InfoItem` | Non-window facts: Kimi/Moonshot balance, Codex credits |

`SourceStatus` values and semantics:

| Status | Meaning | Affects exit code? |
|---|---|---|
| `ok` | Fetched successfully | success |
| `unconfigured` | No credential found locally | no (informational) |
| `unauthorized` | Credential present but rejected/expired | yes (failure) |
| `unavailable` | Network/timeout/5xx/garbage response | yes (failure) |
| `unsupported` | Source exists but exposes no limit data (Moonshot windows) | no (informational; `Info` may still be populated) |

State transitions (per run — no persistence): `unconfigured → ok | unauthorized | unavailable`
resolved during fetch; a run never mutates stored state.

### LimitWindow

One rate-limit bucket, normalized (spec: Limit Window).

| Field | Type | Notes |
|---|---|---|
| `Label` | `string` | Human name: "5-hour", "Weekly", "Weekly (Opus)", "Requests/min" |
| `Duration` | `time.Duration` | 0 when provider doesn't state it |
| `UsedPercent` | `float64` | 0–100. Canonical health input; computed from counts when absent |
| `Used`, `Limit` | `*float64` | Absolute counts when known (Kimi, API-key headers); nil for percent-only providers (Claude/Codex plan) |
| `Unit` | `string` | "requests", "tokens", "" when percent-only |
| `ResetAt` | `*time.Time` | nil ⇒ unknown; renders as "—" not an error |
| `Unlimited` | `bool` | Renders "unlimited", never 0% (spec Edge Cases) |

Derived: `Health(now)` and `ResetIn(now)`.

### Health (derived, FR-005)

```
exhausted  ⇐ UsedPercent >= 100 or provider reports blocked
warning    ⇐ UsedPercent >= 90
healthy    ⇐ otherwise
```

Each state carries a non-color marker (symbol + label) so states survive `NO_COLOR`/pipes:
`●` healthy, `◐ WARN` warning, `○ FULL` exhausted (exact glyphs finalized in implementation;
contract is "distinguishable without color").

### InfoItem

| Field | Type | Notes |
|---|---|---|
| `Label` | `string` | e.g. "Balance", "Extra credits" |
| `Value` | `string` | Pre-formatted ("$49.59", "1,834 requests") |

## Validation rules (from FRs)

- `UsedPercent` clamped to [0, 100+] — values >100 are kept (render as exhausted) but never
  negative.
- Unknown window types from providers are passed through generically with their reported
  label/duration (spec Edge Cases: never dropped).
- Credential material MUST never appear in any field of this model (FR-011) — enforced by
  construction: adapters populate `Detail` from fixed message templates + file paths only.
- All numeric parsing is lenient: `json.Number`, string-numbers, absent fields → nil, never a
  parse abort for the whole source (fragility register).

## Provider normalization map (adapter contracts)

| Provider/source | Raw | → LimitWindow |
|---|---|---|
| Claude plan | `five_hour/seven_day[_opus/_sonnet].utilization` (0–100), `resets_at` ISO | percent-only; labels "5-hour", "Weekly", "Weekly (Opus/Sonnet)"; null blocks skipped |
| Claude apikey | `anthropic-ratelimit-*-{limit,remaining,reset}` headers | counts; Used = limit−remaining; per-minute windows |
| Codex plan | `rate_limit.primary_window/secondary_window.used_percent`, `reset_at` epoch s | percent-only; labels "5-hour", "Weekly"; legacy shape fallback |
| Codex apikey | `x-ratelimit-{limit,remaining,reset}-{requests,tokens}` (durations) | counts; reset = now + parsed duration |
| Kimi plan | `usage` (weekly, string numbers) + `limits[]` (windowed) | counts + percent computed; RFC3339Nano resets |
| Kimi apikey (Moonshot) | `/users/me/balance` | no windows → `unsupported` + `InfoItem` balance |

## JSON output shape (FR-009)

Mirrors the model 1:1 (lowercase snake_case keys); formal schema in
[contracts/usage-json.schema.json](contracts/usage-json.schema.json). Timestamps RFC 3339 UTC;
percents as numbers; absolute counts nullable — satisfying SC-006 round-trip.
