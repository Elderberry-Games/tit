# Quickstart & Validation: tit

How to build, run, and prove the feature works end-to-end. Contracts:
[contracts/cli.md](contracts/cli.md), [contracts/usage-json.schema.json](contracts/usage-json.schema.json).

## Prerequisites

- Go (latest stable) on PATH
- Optional, for live validation: at least one of — Claude Code logged in, Codex CLI logged in,
  Kimi CLI logged in, or `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `MOONSHOT_API_KEY` set.
  (All automated tests run with zero live credentials.)

## Build & quality gates (constitution merge gates)

```sh
go build ./...
gofmt -l .            # expect: no output
go vet ./...
govulncheck ./...
go test -race -cover ./...   # coverage must be ≥85% total
```

## Validation scenarios

### V1 — Unified view (US1)

```sh
go run ./cmd/tit usage
```

Expect: one section per provider (Claude, Codex, Kimi); per configured source, aligned rows
with bar, percent (and counts where the provider reports them), health marker, and "resets in
…" chip. Providers without local credentials show a one-line "not configured" hint — not an
error. Exit code 0 if everything attempted succeeded (`echo $?`).

### V2 — Partial failure isolation (US1 scenario 4)

Run with one provider's credential deliberately broken (e.g. `CODEX_HOME=/tmp/empty tit
usage` with the others configured). Expect: other providers render normally; broken one shows
reason + fix; exit code 3 only if a configured source *failed* (missing ⇒ still 0).

### V3 — Discoverability (US2)

```sh
go run ./cmd/tit            # help, exit 0
go run ./cmd/tit bogus      # friendly suggestion, exit 2
go run ./cmd/tit --version  # "tit vX.Y.Z"
```

### V4 — Filter & JSON (US3)

```sh
go run ./cmd/tit usage claude          # only Claude section
go run ./cmd/tit usage --json | jq .   # parses; validates against schema
go run ./cmd/tit usage --json --watch  # friendly error, exit 2
```

JSON must round-trip every figure in the human view (SC-006); validate with any JSON Schema
validator against `contracts/usage-json.schema.json`.

### V5 — Styling degradation (FR-010)

```sh
go run ./cmd/tit usage | cat            # no ANSI codes, same content
NO_COLOR=1 go run ./cmd/tit usage       # no color; health markers still distinguish states
```

### V6 — Watch mode (US3 scenario 4, FR-016)

```sh
go run ./cmd/tit usage --watch
```

Expect: full-screen live view; reset countdowns tick every second; data refreshes ≤60 s
(Claude plan ≥180 s); `q` and Ctrl+C exit cleanly restoring the terminal.

## Fixture-based validation (no credentials)

`go test ./internal/provider/...` runs every provider adapter against recorded fixtures:
success, 401 expired-token, 429, garbage, legacy payload shapes (see research.md fragility
register). `go test ./internal/ui/...` golden-tests the renderer at width 80 with a fixed
clock. These are the authoritative acceptance checks for CI; the live scenarios above are the
human smoke test.
