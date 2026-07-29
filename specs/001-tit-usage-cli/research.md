# Phase 0 Research: tit — Unified AI Usage-Limit Monitor

**Date**: 2026-07-29 | **Spec**: [spec.md](spec.md)

All NEEDS CLARIFICATION items from Technical Context are resolved below. Provider endpoint
details were researched from official docs, provider CLI source code, and prior-art tools
(onWatch, CodexBar, ccusage, codex-reset-watcher, smithers). Fragile/undocumented surfaces are
flagged — they drive the "parse leniently, fail friendly" design rules.

## R1. CLI framework

- **Decision**: Standard library `flag` package with manual subcommand dispatch (`tit usage`,
  `tit help`, `tit version`). No cobra/urfave.
- **Rationale**: Constitution Principle V (Simple as Fuck) and III (stdlib preferred). One
  subcommand in v1 does not justify a CLI framework dependency.
- **Alternatives considered**: spf13/cobra (rejected: heavy dependency tree for one
  subcommand); urfave/cli (rejected: same reason).

## R2. TUI stack (TC-001: Bubble Tea mandated)

- **Decision**: Charm v2 stack on `charm.land` vanity imports:
  - `charm.land/bubbletea/v2` (v2.0.8) — `--watch` mode only
  - `charm.land/lipgloss/v2` (v2.0.5) — all styling, both modes
  - `charm.land/bubbles/v2` (v2.1.1) — `progress.ViewAs` for bars (pure, works outside a program)
  - `github.com/charmbracelet/x/term` (v0.2.2) — TTY detection + width for one-shot mode
- **Rationale**: v2 is the stable current major (v1 is legacy). TC-001 mandates Bubble Tea and
  permits companion libraries. `bubbles/progress.ViewAs(pct)` renders static bars with no
  Init/Update, so the same component serves one-shot and watch modes.
- **One-shot rendering decision**: skip `tea.Program` entirely in default mode. A pure function
  `renderDashboard(snapshot, width) string` is printed via lipgloss v2 writers
  (`lipgloss.Println`), which auto-downsample color to terminal capability and strip ANSI when
  stdout is not a TTY — satisfying FR-010 (NO_COLOR / pipe handling) for free. This is the
  idiomatic Charm pattern (cf. `gum style`, bubbletea `examples/progress-static`);
  `tea.WithoutRenderer` is for daemons, not one-shot prints.
- **Watch mode decision**: full-screen `tea.Program` (`View.AltScreen = true`), canonical
  `tea.Tick` 1-second heartbeat (re-returned each tick), async provider fetch as `tea.Cmd`,
  quit on `q`/`ctrl+c` via `tea.KeyPressMsg.String()`. Same `renderDashboard` core.
- **Alternatives considered**: bubbletea v1 (rejected: superseded); hand-rolled bars
  (rejected: bubbles is already justified by TC-001's ecosystem allowance).

## R3. Claude provider

- **Plan-window source** (subscription):
  - Credentials: `CLAUDE_CODE_OAUTH_TOKEN` env → `~/.claude/.credentials.json` (Linux, mode
    600) → macOS Keychain item `Claude Code-credentials` (via `security
    find-generic-password`). JSON: `claudeAiOauth.accessToken`, `.expiresAt` (epoch **ms**).
  - Endpoint: `GET https://api.anthropic.com/api/oauth/usage` with `Authorization: Bearer`,
    `anthropic-beta: oauth-2025-04-20`, and a `User-Agent: claude-code/<version>` header
    (load-bearing: without it, aggressive 429s). **Undocumented — parse leniently.**
  - Response: `five_hour` / `seven_day` / `seven_day_opus` / `seven_day_sonnet` objects with
    `utilization` (0–100) and `resets_at` (ISO-8601); nullable per plan. Percent-only — no
    absolute counts; the data model must support percent-only windows.
  - Poll discipline: ≥180 s between calls (community-established safe interval); no
    `Retry-After` on its 429s → cache + long backoff in watch mode.
- **API-key source**: `ANTHROPIC_API_KEY` → probe `POST /v1/messages/count_tokens` (free,
  carries headers) and read `anthropic-ratelimit-{requests,tokens,input-tokens,output-tokens}-
  {limit,remaining,reset}` (reset is RFC 3339). Documented and stable.
- **Rejected**: parsing `~/.claude/projects/**/*.jsonl` (ccusage approach) — yields token
  counts, not authoritative window utilization; more code for worse data. Admin Rate Limits
  API — org-admin keys only, static config not live usage.

## R4. Codex provider

- **Plan-window source** (ChatGPT subscription):
  - Credentials: `$CODEX_HOME/auth.json`, default `~/.codex/auth.json`; use
    `tokens.access_token` (JWT) + `tokens.account_id` (fallback: `chatgpt_account_id` claim
    from `id_token`). Plan name from `chatgpt_plan_type` claim. Keyring storage mode exists on
    newer versions → treat missing file as "unconfigured", not an error.
  - Endpoint: `GET https://chatgpt.com/backend-api/wham/usage` with `Authorization: Bearer` +
    `ChatGPT-Account-Id: <account_id>`. Read-only, zero quota cost. **Undocumented; shape has
    already changed once — tolerate both `rate_limit.primary_window` (current) and legacy
    `rate_limits.primary` shapes.**
  - Response: `primary_window` (5 h, `limit_window_seconds: 18000`) and `secondary_window`
    (weekly, 604800) with `used_percent` (0–100) and `reset_at` (epoch seconds); plus
    `plan_type`, `credits`. Percent-only.
- **API-key source**: `OPENAI_API_KEY` → cheapest header-carrying probe: minimal
  `POST /v1/chat/completions` (cheapest model, `max_completion_tokens: 1`, cost ≈ fractions of
  a cent). Headers: `x-ratelimit-{limit,remaining,reset}-{requests,tokens}`; reset values are
  Go-style duration strings (`6m0s`, may include `d` units — lenient parser needed).
  Documented. Probe cost is disclosed in help text; `GET /v1/models` is free but carries no
  rate-limit headers.
- **Never refresh** the OAuth token (rotation would corrupt the Codex CLI session): on 401,
  show "run `codex` to re-authenticate".

## R5. Kimi provider

- **Plan-window source** (Kimi for Coding):
  - Credentials: `KIMI_CODE_API_KEY` env → `~/.kimi-code/credentials/kimi-code.json` (current
    CLI) → `~/.kimi/credentials/kimi-code.json` (legacy). JSON: `access_token`, `expires_at`
    (unix seconds float; heuristic: >1e12 → ms).
  - Endpoint: `GET https://api.kimi.com/coding/v1/usages`, `Authorization: Bearer`. 404 ⇒ key
    is not a Kimi-for-Coding key. **Undocumented but implemented by both official CLIs.**
  - Response quirks: numbers arrive as **decimal strings** (use `json.Number`); `usage` block
    = weekly quota (window omitted — synthesize 7 d); `limits[]` = windowed limits (5 h window
    arrives as `duration: 300, timeUnit: TIME_UNIT_MINUTE`); `resetTime` has nanosecond
    precision (RFC3339Nano); `used` may be absent → compute `limit − remaining`. Absolute
    counts available (unlike Claude/Codex).
  - Read-only token policy (CodexBar pattern): never refresh (refresh tokens rotate and would
    invalidate the CLI's chain); on expiry → "run `kimi` to re-login".
  - **Access tokens expire after 15 minutes** (`expires_in: 900`). Consequence: the cached
    token is valid only if the Kimi CLI ran very recently, so Kimi's plan source will often
    report `unauthorized` with a re-login hint. Accepted trade-off (see R11); implementing the
    refresh flow ourselves is explicitly rejected.
- **API-key source** (Moonshot open platform): `MOONSHOT_API_KEY` (+ `MOONSHOT_REGION` for
  `.ai` vs `.cn` host) → `GET /v1/users/me/balance` (documented). No rate-limit headers and no
  tier/RPM query exist → report balance as an informational entry and state honestly that
  windowed limits aren't queryable (spec assumption covers this).

## R6. Fetch orchestration, caching, timeouts

- **Decision**: fetch all configured providers concurrently (goroutine per provider,
  `errgroup`-style but stdlib `sync.WaitGroup`); per-provider HTTP timeout 2.5 s
  (`context.WithTimeout`) so the one-shot view meets SC-001's 3-second budget even when one
  provider hangs. Partial results render per FR-007.
- **Watch mode refresh**: data re-fetch every 60 s (FR-016 ceiling) **except** Claude plan
  endpoint at 180 s minimum (R3 poll discipline) — per-provider `MinPollInterval` honored by
  the refresh scheduler; countdown tick every 1 s is display-only.
- **No on-disk cache** in v1 (Simple as Fuck): one-shot always fetches live; watch caches in
  memory only.

## R7. Exit codes (deferred item resolved)

- **Decision**: `0` = all attempted providers succeeded; `3` = partial (≥1 succeeded, ≥1
  failed); `1` = total failure (all attempted failed) or internal error; `2` = CLI usage error
  (unknown subcommand/flag, `--watch` + `--json`). Unconfigured providers are informational
  and never affect the exit code.
- **Rationale**: FR-013 requires the three outcome classes; `2` for usage errors follows Unix
  convention; sysexits rejected as over-engineering.

## R8. Provider filter syntax (deferred item resolved)

- **Decision**: positional argument, case-insensitive: `tit usage claude|codex|kimi`. Unknown
  name → friendly error listing valid providers, exit 2.

## R9. Testing strategy (Constitution II: ≥85%)

- **Decision**:
  - Provider clients tested against `net/http/httptest` fake servers replaying recorded
    response fixtures (success, 401, 429, garbage, legacy shapes) — no live credentials in
    tests (spec US1 independent-test requirement).
  - Credential discovery tested against temp dirs with fixture files (`$HOME` overridden).
  - Rendering tested as pure-function golden files:
    `github.com/charmbracelet/x/exp/golden`, fixed width 80, deterministic fake clock for
    countdowns.
  - Watch model tested by driving `Update` directly with synthetic messages (tick, data,
    keypress); `teatest/v2` reserved for one smoke test (experimental — pinned pseudo-version).
  - CI gate: `go test -race -cover` with 85% threshold check, `gofmt`, `go vet`, `govulncheck`.
- **Rationale**: pure `renderDashboard(snapshot, width)` makes golden testing trivial;
  injected `now()` clock makes countdowns deterministic.

## R10. Security posture (Constitution I)

- Credentials are read from their existing locations, never written, never refreshed, never
  logged; error messages name the credential *source* (file path) but never token contents.
- All endpoints HTTPS; TLS verification never disabled; `http.Client` with explicit timeout.
- The `User-Agent: claude-code/<version>` header on the Claude plan endpoint is required for
  functional access to the user's own data (avoids a hostile rate-limit bucket) — accessing
  only the invoking user's own account with their own token.
- OpenAI API-key probe spends real (sub-cent) money — documented in `tit usage --help`.
- `govulncheck` clean is a merge gate (constitution); dependencies limited to the Charm stack
  (TC-001) + stdlib.

## R11. Source strategy decision (2026-07-29)

- **Decision**: cached OAuth credentials are the v1 source strategy, exactly as specified in
  R3–R5 — tit reads each harness's existing credential store read-only and calls the provider
  endpoints itself. No token refresh, no harness process invocation in v1.
- **Rationale**: zero user configuration (credentials already exist locally), fast (single
  HTTP call per source, no CLI startup cost), and structurally simple (constitution V).
- **Alternatives investigated and deferred** (researched 2026-07-29; viable, documented here as
  future headroom rather than v1 scope):
  - Claude: `claude --print "/usage"` dispatches the usage slash command non-interactively at
    zero quota cost with the harness handling auth/refresh; output is human text (integer
    percentages, localized reset strings) and each call costs ~3–4.5 s of CLI startup.
    Recent-version-dependent — feature-detect via the `slash_commands` list in the
    `--output-format json` init event.
  - Claude: registering as the statusline command yields a pushed `rate_limits` JSON payload
    with decimal percentages and epoch resets — richer than any pull source, but only while an
    interactive session is running, and it requires modifying the user's settings.
  - Codex: `codex app-server` (JSON-RPC over stdio) exposes `account/rateLimits/read`,
    returning structured limits in ~0.85 s cold including spawn, with codex refreshing its own
    token. Requires codex ≥ 0.48.0. This is the most stable contract of any Codex avenue.
  - Kimi: `kimi web --no-open` runs a local REST server exposing
    `GET /api/v1/oauth/usage` (bearer token at `~/.kimi-code/server.token`, running instances
    discoverable via `~/.kimi-code/server/instances/*.json`), which sidesteps the 15-minute
    token TTL entirely.
- **Accepted consequence**: Kimi's plan source will frequently show `unauthorized` because of
  the 15-minute access-token TTL (R5). The friendly hint ("run `kimi` to refresh") makes this
  legible rather than mysterious. If it proves annoying in practice, the `kimi web` avenue
  above is the drop-in remedy in a follow-up feature.

## Fragility register (drives lenient parsing + friendly errors)

| Surface | Status | Mitigation |
|---|---|---|
| Claude `/api/oauth/usage` | Undocumented, 429-happy | UA header, ≥180 s poll, lenient decode, cache |
| Codex `/wham/usage` | Undocumented, shape drifted once | Tolerate both payload shapes |
| Kimi `/coding/v1/usages` | Undocumented (official CLIs use it) | `json.Number`, defensive key aliases |
| macOS keychain reads | Version-dependent | Fall back to file; "unconfigured" not "error" |
| Moonshot `.ai`/`.cn` split | Region-dependent | `MOONSHOT_REGION` env, default `.ai` |

All three plan endpoints may break without notice; FR-007's per-provider failure isolation is
the structural defense.
