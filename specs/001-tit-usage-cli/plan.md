# Implementation Plan: tit — Unified AI Usage-Limit Monitor (CLI)

**Branch**: `001-tit-usage-cli` | **Date**: 2026-07-29 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-tit-usage-cli/spec.md`

## Summary

`tit` is a single-binary Go CLI whose `usage` subcommand shows Claude, Codex, and Kimi
rate-limit windows (plan subscriptions + API keys, merged and labeled) in one slick terminal
view. Approach: three provider adapters normalize wildly different undocumented endpoints into
one small data model; a pure `renderDashboard(snapshot, width)` function styled with Lip Gloss
v2 serves both the default one-shot print and the Bubble Tea v2 `--watch` live view (TC-001).
Concurrent fetches with per-provider 2.5 s timeouts keep the view under 3 s; per-provider
failures render as friendly hints without hiding healthy providers. Zero user configuration:
tit reads the OAuth credentials the harness CLIs have already cached locally. Full
endpoint/credential research in [research.md](research.md).

## Technical Context

**Language/Version**: Go, latest stable (currently 1.25.x); module `github.com/Elderberry-Games/tit`

**Primary Dependencies**: `charm.land/bubbletea/v2` (watch mode, TC-001),
`charm.land/lipgloss/v2` (styling both modes), `charm.land/bubbles/v2` (progress bars),
`github.com/charmbracelet/x/term` (TTY/width). Test-only: `x/exp/golden`, `x/exp/teatest/v2`.
No CLI framework — stdlib `flag` + manual dispatch (research R1).

**Storage**: none. Reads existing cached OAuth credentials (files/keychain/env) read-only
(research R3–R5, strategy confirmed in R11); never writes, never refreshes tokens, no cache
files. Harness-delegation alternatives (`claude --print "/usage"`, `codex app-server`,
`kimi web`) are researched and deferred as future headroom, not v1 scope.

**Testing**: `go test -race -cover` ≥85%; `httptest` fixture servers per provider;
golden-file rendering tests (fixed width, injected clock); watch model driven via direct
`Update` calls + one `teatest/v2` smoke test (research R9).

**Target Platform**: Linux and macOS terminals (amd64/arm64); single static binary.

**Project Type**: CLI tool (single project).

**Performance Goals**: complete one-shot view <3 s (SC-001) via concurrent fetches with 2.5 s
per-provider timeout; watch mode 1 s display tick, ≤60 s refresh (Claude plan ≥180 s min-poll).

**Constraints**: read-only credential access; no secrets in output/logs; graceful degradation
at 60-column widths; NO_COLOR/pipe strips styling; undocumented endpoints require lenient
parsing (fragility register in research.md).

**Scale/Scope**: 3 providers × ≤2 sources; ~10 windows max per run; single user, single
machine; ~2–3 k LOC estimate.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate check | Status |
|---|---|---|---|
| I | Security First | Credentials read-only from existing stores; never logged/printed; message templates exclude token contents; TLS everywhere, no insecure transport; deps limited to Charm stack, `govulncheck` in merge gates | ✅ PASS |
| II | Tests ≥85% | Fixture-server + golden + model-driven test strategy defined (R9); testable-by-construction: pure renderer, injected clock, `$HOME`/env-overridable discovery; regression fixtures for every failure mode | ✅ PASS |
| III | Go, and Only Go | Pure Go, latest stable, no CGo; single static binary; stdlib `flag` over cobra; every dep justified (TC-001 mandate + x/term) | ✅ PASS |
| IV | Fun Is a Feature | Slick Mobbin-derived layout (bars, chips, health glyphs); playful-but-precise copy in help and error hints; color used for meaning only | ✅ PASS |
| V | Simple as Fuck | One binary, one real subcommand; no plugin system (three hardcoded adapters behind one interface); no config file; no on-disk cache; boring stdlib HTTP | ✅ PASS |

**Post-Phase-1 re-check** (after data-model + contracts): still PASS — design added no new
dependencies, no storage, no scope growth. The only flagged tension: the OpenAI API-key probe
spends sub-cent money (documented in help text; R10) and the Claude plan endpoint requires a
`claude-code` User-Agent to access the user's own data (R3/R10) — both accepted as necessary
for the product's core purpose, neither weakens a principle.

## Project Structure

### Documentation (this feature)

```text
specs/001-tit-usage-cli/
├── plan.md              # This file
├── research.md          # Phase 0: endpoints, credentials, stack decisions, fragility register
├── data-model.md        # Phase 1: Snapshot/ProviderResult/SourceResult/LimitWindow
├── quickstart.md        # Phase 1: build, gates, validation scenarios V1–V6
├── contracts/
│   ├── cli.md           # Command surface, flags, exit codes, env inputs
│   └── usage-json.schema.json  # --json output schema (SC-006)
└── tasks.md             # Phase 2 (/speckit-tasks — not created here)
```

### Source Code (repository root)

```text
cmd/
└── tit/
    └── main.go          # entrypoint: flag parsing, subcommand dispatch, exit codes

internal/
├── usage/
│   ├── model.go         # Snapshot, ProviderResult, SourceResult, LimitWindow, Health
│   ├── fetch.go         # concurrent orchestration, timeouts, exit-code derivation
│   └── json.go          # --json encoder (schema-conformant)
├── provider/
│   ├── provider.go      # Provider interface + registry (fixed: claude, codex, kimi)
│   ├── claude/          # creds discovery (file/keychain/env), oauth/usage + count_tokens probe
│   ├── codex/           # auth.json + JWT claims, wham/usage (both shapes), openai probe
│   └── kimi/            # kimi-code creds, /coding/v1/usages (json.Number), moonshot balance
├── ui/
│   ├── render.go        # pure renderDashboard(snapshot, width) — shared by both modes
│   ├── styles.go        # lipgloss styles, health glyphs, bar config
│   └── watch.go         # bubbletea model: tick/fetch/quit (FR-016)
└── version/
    └── version.go       # version string (ldflags-injected)

tests live beside code (_test.go); fixtures under internal/provider/<p>/testdata/
```

**Structure Decision**: single Go module, standard `cmd/` + `internal/` layout. `internal/`
prevents accidental API exposure (this is a tool, not a library — constitution V). Provider
adapters implement one small interface behind a fixed registry: isolation per constitution's
extensibility boundary without a plugin framework.

## Complexity Tracking

No constitution violations to justify — table intentionally empty.
