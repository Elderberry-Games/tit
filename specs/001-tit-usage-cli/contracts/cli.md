# CLI Contract: tit

The user-facing command surface. Breaking any of this is a breaking change.

## Synopsis

```
tit                      # help + subcommand list, exit 0
tit help                 # same as above
tit --help               # same as above
tit --version | version  # "tit vX.Y.Z", exit 0
tit usage                # unified one-shot view, all providers
tit usage <provider>     # filter: claude | codex | kimi (case-insensitive)
tit usage --json         # machine-readable output (see usage-json.schema.json)
tit usage --watch        # live full-screen view; quit with q or Ctrl+C
tit usage --help         # usage-specific help (flags, providers, probe-cost note)
```

## Flags (usage subcommand)

| Flag | Effect | Conflicts |
|---|---|---|
| `--json` | Emit JSON snapshot to stdout, no styling | `--watch` (exit 2) |
| `--watch` | Live view: 1 s countdown tick, ≤60 s data refresh (Claude plan ≥180 s) | `--json` (exit 2) |

## Exit codes (FR-013, research R7)

| Code | Meaning |
|---|---|
| 0 | All attempted sources succeeded (unconfigured/unsupported are not failures) |
| 1 | Total failure: every attempted source failed, or internal error |
| 2 | Usage error: unknown subcommand/provider/flag, `--json`+`--watch` |
| 3 | Partial failure: at least one source succeeded and at least one failed |

## Streams

- Result view / JSON → **stdout**. Errors and usage messages → **stderr**.
- stdout not a TTY, or `NO_COLOR` set ⇒ no ANSI styling; content identical (FR-010).
- `--json` output is styling-free by definition; `--watch` requires a TTY (else exit 2 with a
  friendly message).

## Environment inputs (all optional)

| Variable | Role |
|---|---|
| `CLAUDE_CODE_OAUTH_TOKEN` | Claude plan token override |
| `ANTHROPIC_API_KEY` | Claude API-key source |
| `CODEX_HOME` | Codex auth.json directory (default `~/.codex`) |
| `OPENAI_API_KEY` | Codex API-key source (probe costs <$0.001 — noted in help) |
| `KIMI_CODE_API_KEY` | Kimi plan key override |
| `MOONSHOT_API_KEY` | Kimi API-key source (balance only) |
| `MOONSHOT_REGION` | `ai` (default) or `cn` |
| `NO_COLOR` | Disable styling |

## Human view contract (one-shot and watch share layout)

Per provider section: header (name + plan/status detail), then one aligned row per limit
window: `[source] label  bar  used/total or percent  health-marker  resets-in`. Health markers
survive colorless output (FR-005). Failing/unconfigured providers render a one-line reason +
fix hint in place of rows (FR-007). Exact strings are golden-file-tested, not contractual;
the row information set IS contractual (FR-003, FR-006).

## Error message contract (US2 scenario 4, SC-004)

Every user-facing failure names: what failed, why (best known), and one next step. Never a raw
stack trace, HTTP dump, or bare status code. Never credential contents (FR-011).
