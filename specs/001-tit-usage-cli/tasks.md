---

description: "Task list for 001-tit-usage-cli"
---

# Tasks: tit — Unified AI Usage-Limit Monitor (CLI)

**Input**: Design documents from `/specs/001-tit-usage-cli/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/](contracts/)

**Tests**: REQUIRED. Constitution Principle II ("Tests or It Didn't Happen") mandates ≥85%
coverage as a merge gate, with tests written before or alongside implementation. Every
implementation task below is paired with a test task, and the test task comes first.

**Organization**: Tasks are grouped by user story so each story is independently implementable
and testable. Parallelism is maximized: the three provider adapters are fully independent
tracks (separate packages, separate files, no shared state), and within each provider the
credential, plan-source, and API-key-source files are independent of each other.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel — different files, no dependency on an incomplete task
- **[Story]**: US1 / US2 / US3 (user story phases only)
- File paths are exact and relative to repository root

## Path Conventions

Single Go module at repository root (per plan.md Structure Decision): `cmd/tit/`,
`internal/{usage,provider,ui,version}/`. Go convention places tests beside the code they test
(`foo_test.go`), and fixtures in `testdata/` — there is no top-level `tests/` directory.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, toolchain, and CI gates

- [ ] T001 Initialize Go module `github.com/Elderberry-Games/tit` with latest stable Go in `go.mod` at repository root
- [ ] T002 [P] Add pinned dependencies to `go.mod`: `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`, `github.com/charmbracelet/x/term`, and test-only `github.com/charmbracelet/x/exp/golden`
- [ ] T003 [P] Create `.gitignore` for Go (binaries, coverage output, `dist/`)
- [ ] T004 [P] Create `Makefile` with `build`, `test`, `cover`, `lint`, `vuln`, and `all` targets matching the constitution merge gates
- [ ] T005 [P] Create `.github/workflows/ci.yml` running `gofmt -l .`, `go vet ./...`, `govulncheck ./...`, and `go test -race -cover ./...` with a hard fail below 85% total coverage
- [ ] T006 [P] Create `scripts/check-coverage.sh` that parses `go test -coverprofile` output and exits non-zero below the 85% threshold

**Checkpoint**: `make all` runs green on an empty module

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The shared data model, provider contract, and orchestration every user story
builds on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T007 [P] Create core types `Snapshot`, `ProviderResult`, `SourceResult`, `LimitWindow`, `InfoItem`, and the `ProviderID`/`SourceKind`/`SourceStatus` enums in `internal/usage/model.go` per [data-model.md](data-model.md)
- [ ] T008 [P] Create injectable clock type (`Clock func() time.Time`) with a fixed-clock test helper in `internal/usage/clock.go`
- [ ] T009 [P] Create friendly-message templates (missing credential, expired credential, provider unreachable, unsupported source) in `internal/usage/messages.go` — templates take file paths and provider names only, never credential values (FR-011)
- [ ] T010 [P] Write table-driven tests for `Health()` boundaries (89.9% healthy, 90% warning, 100% exhausted, >100% exhausted, unlimited never exhausted) in `internal/usage/health_test.go`
- [ ] T011 [P] Write tests for `ResetIn()` including nil `ResetAt`, past reset times, and sub-second rounding in `internal/usage/reset_test.go`
- [ ] T012 Implement `Health()` and `ResetIn()` derivations in `internal/usage/health.go` making T010 and T011 pass (depends on T007)
- [ ] T013 [P] Define the `Provider` interface (`ID()`, `DisplayName()`, `Fetch(ctx, Clock) []SourceResult`) and the fixed three-entry registry in `internal/provider/provider.go`
- [ ] T014 [P] Create version variable with `ldflags` injection point and a fallback value in `internal/version/version.go`
- [ ] T015 [P] Write tests for concurrent fetch orchestration using fake providers: all-success, one-failure-partial, all-failure, per-provider timeout, and unconfigured-is-not-a-failure in `internal/usage/fetch_test.go`
- [ ] T016 Implement concurrent fetch orchestration with per-provider `context.WithTimeout` (2.5 s) and exit-code derivation (0/1/3 per [contracts/cli.md](contracts/cli.md)) in `internal/usage/fetch.go` making T015 pass (depends on T007, T013)

**Checkpoint**: Data model, health rules, and orchestration are tested and green — all three
provider tracks and the renderer can now proceed in parallel

---

## Phase 3: User Story 1 - See all my limits at a glance (Priority: P1) 🎯 MVP

**Goal**: `tit usage` renders one unified terminal view containing every configured provider's
plan and API-key limit windows, with bars, percentages, health states, and reset countdowns.

**Independent Test**: Run `tit usage` with all three providers backed by fixture servers and
verify every provider's windows, usage figures, and reset times appear in one coherent view;
kill one provider and verify the others still render (quickstart V1, V2).

### Provider track A — Claude (fully parallel with tracks B and C)

- [ ] T017 [P] [US1] Add response fixtures (success with all four windows, null Opus/Sonnet blocks, 401 expired, 429 rate-limited, malformed JSON) in `internal/provider/claude/testdata/`
- [ ] T018 [P] [US1] Write credential-discovery tests covering env override, `~/.claude/.credentials.json`, missing file, and expired `expiresAt` (epoch ms) using a temp `$HOME` in `internal/provider/claude/creds_test.go`
- [ ] T019 [P] [US1] Write plan-source client tests against `httptest` servers replaying every fixture, asserting the `claude-code` User-Agent and `anthropic-beta` headers are sent, in `internal/provider/claude/plan_test.go`
- [ ] T020 [P] [US1] Write API-key-source tests parsing `anthropic-ratelimit-*` headers into windows in `internal/provider/claude/apikey_test.go`
- [ ] T021 [US1] Implement credential discovery (`CLAUDE_CODE_OAUTH_TOKEN` → credentials file → macOS keychain shell-out) in `internal/provider/claude/creds.go` making T018 pass
- [ ] T022 [US1] Implement plan-source client for `GET /api/oauth/usage` with lenient decoding of `five_hour`/`seven_day`/`seven_day_opus`/`seven_day_sonnet` into percent-only windows in `internal/provider/claude/plan.go` making T019 pass
- [ ] T023 [US1] Implement API-key source using the free `count_tokens` probe and header parsing in `internal/provider/claude/apikey.go` making T020 pass
- [ ] T024 [US1] Implement the `Provider` adapter merging both sources with per-source status in `internal/provider/claude/claude.go` (depends on T021, T022, T023)

### Provider track B — Codex (fully parallel with tracks A and C)

- [ ] T025 [P] [US1] Add response fixtures (current `rate_limit.primary_window` shape, legacy `rate_limits.primary` shape, 401, malformed JSON) in `internal/provider/codex/testdata/`
- [ ] T026 [P] [US1] Write credential-discovery tests for `$CODEX_HOME/auth.json` including `account_id` fallback to the `id_token` JWT claim and missing-file (keyring mode) handling in `internal/provider/codex/creds_test.go`
- [ ] T027 [P] [US1] Write plan-source client tests asserting both payload shapes decode identically and that `ChatGPT-Account-Id` is sent, in `internal/provider/codex/plan_test.go`
- [ ] T028 [P] [US1] Write API-key-source tests parsing `x-ratelimit-*` headers including Go-duration reset strings with `d` units in `internal/provider/codex/apikey_test.go`
- [ ] T029 [US1] Implement credential discovery and unverified JWT claim extraction (`chatgpt_plan_type`, `chatgpt_account_id`) in `internal/provider/codex/creds.go` making T026 pass
- [ ] T030 [US1] Implement plan-source client for `GET /backend-api/wham/usage` with dual-shape tolerance in `internal/provider/codex/plan.go` making T027 pass
- [ ] T031 [US1] Implement API-key source with the minimal chat-completions probe and lenient duration parser in `internal/provider/codex/apikey.go` making T028 pass
- [ ] T032 [US1] Implement the `Provider` adapter merging both sources in `internal/provider/codex/codex.go` (depends on T029, T030, T031)

### Provider track C — Kimi (fully parallel with tracks A and B)

- [ ] T033 [P] [US1] Add response fixtures (string-encoded numbers, 5-hour `TIME_UNIT_MINUTE` window, nanosecond `resetTime`, absent `used`, 404 wrong-key-type, expired token) in `internal/provider/kimi/testdata/`
- [ ] T034 [P] [US1] Write credential-discovery tests for `KIMI_CODE_API_KEY`, `~/.kimi-code/credentials/kimi-code.json`, the legacy `~/.kimi/` path, and the 15-minute-expiry path producing `unauthorized` with a re-login hint, in `internal/provider/kimi/creds_test.go`
- [ ] T035 [P] [US1] Write plan-source client tests asserting `json.Number` handling, `used = limit − remaining` fallback, and weekly-window synthesis in `internal/provider/kimi/plan_test.go`
- [ ] T036 [P] [US1] Write Moonshot balance-source tests asserting `unsupported` status with a populated balance `InfoItem` in `internal/provider/kimi/apikey_test.go`
- [ ] T037 [US1] Implement credential discovery with expiry detection in `internal/provider/kimi/creds.go` making T034 pass
- [ ] T038 [US1] Implement plan-source client for `GET /coding/v1/usages` in `internal/provider/kimi/plan.go` making T035 pass
- [ ] T039 [US1] Implement Moonshot balance source with `MOONSHOT_REGION` host selection in `internal/provider/kimi/apikey.go` making T036 pass
- [ ] T040 [US1] Implement the `Provider` adapter merging both sources in `internal/provider/kimi/kimi.go` (depends on T037, T038, T039)

### Rendering track (parallel with all provider tracks)

- [ ] T041 [P] [US1] Implement lipgloss styles, health glyphs (`●` / `◐ WARN` / `○ FULL`), and bar configuration in `internal/ui/styles.go`
- [ ] T042 [P] [US1] Create golden-file test fixtures covering a healthy snapshot, mixed health states, a failed provider, an unconfigured provider, and an all-exhausted snapshot in `internal/ui/testdata/`
- [ ] T043 [P] [US1] Write golden tests for `renderDashboard` at fixed width 80 with a fixed clock in `internal/ui/render_test.go`
- [ ] T044 [US1] Implement the pure `renderDashboard(snapshot, width) string` with aligned per-window rows (source label, bar, counts-or-percent, health marker, reset chip) in `internal/ui/render.go` making T043 pass (depends on T041)
- [ ] T045 [P] [US1] Write narrow-terminal tests at widths 60 and 40 asserting no broken bars or wrapped garbage in `internal/ui/render_narrow_test.go`
- [ ] T046 [US1] Implement responsive width degradation in `internal/ui/render.go` making T045 pass (depends on T044)

### Wiring

- [ ] T047 [US1] Implement `cmd/tit/main.go` with subcommand dispatch and the one-shot `usage` path: TTY/width detection via `x/term`, fetch, render through `lipgloss.Println`, exit-code propagation (depends on T016, T024, T032, T040, T044)
- [ ] T048 [P] [US1] Write end-to-end tests running the `usage` command against fixture-backed providers asserting exit codes 0 and 3 and provider isolation in `cmd/tit/main_test.go`

**Checkpoint**: `tit usage` renders the full unified view; MVP is functional and independently
testable (quickstart V1, V2)

---

## Phase 4: User Story 2 - Discover and trust the tool (Priority: P2)

**Goal**: Help, version, unknown-command handling, and per-provider "not configured" guidance
make the tool self-explanatory on first contact.

**Independent Test**: Run `tit`, `tit --help`, `tit --version`, `tit bogus`, and `tit usage`
with zero credentials; verify each produces helpful, correctly-exiting output (quickstart V3).

- [ ] T049 [P] [US2] Write tests for root help output (description, subcommand list, examples, exit 0) and `--version` in `cmd/tit/help_test.go`
- [ ] T050 [P] [US2] Write tests asserting unknown subcommands and unknown flags exit 2 with a suggestion and write to stderr in `cmd/tit/errors_test.go`
- [ ] T051 [P] [US2] Write tests asserting a zero-credential run lists every provider with a specific "how to configure" hint and still exits 0 in `cmd/tit/unconfigured_test.go`
- [ ] T052 [US2] Implement root help text and `usage --help` (including the disclosed sub-cent OpenAI probe cost) in `cmd/tit/help.go` making T049 pass
- [ ] T053 [US2] Implement unknown-subcommand/flag handling with suggestions and exit code 2 in `cmd/tit/main.go` making T050 pass
- [ ] T054 [US2] Implement per-provider unconfigured rendering with fix hints in `internal/ui/render.go` making T051 pass
- [ ] T055 [P] [US2] Audit every error path for raw stack traces, bare status codes, and credential leakage; add a test asserting no error message contains a token-shaped substring in `internal/usage/messages_test.go`

**Checkpoint**: First-contact experience is complete; US1 and US2 both work independently

---

## Phase 5: User Story 3 - Focus and automate (Priority: P3)

**Goal**: Single-provider filtering, machine-readable `--json` output, and the live `--watch`
view.

**Independent Test**: Run `tit usage claude`, `tit usage --json | jq`, and `tit usage --watch`;
verify filtered output, schema-valid JSON, and a live view that exits cleanly (quickstart V4,
V6).

- [ ] T056 [P] [US3] Write tests for case-insensitive provider filtering and unknown-provider exit 2 in `cmd/tit/filter_test.go`
- [ ] T057 [P] [US3] Write JSON encoder tests validating output against `contracts/usage-json.schema.json` and asserting round-trip of every human-view figure (SC-006) in `internal/usage/json_test.go`
- [ ] T058 [P] [US3] Write tests asserting `--json` combined with `--watch` exits 2 with a friendly message in `cmd/tit/flags_test.go`
- [ ] T059 [P] [US3] Write watch-model unit tests driving `Update` directly with tick, data, error, resize, and `q`/`ctrl+c` messages in `internal/ui/watch_test.go`
- [ ] T060 [US3] Implement provider filtering in `cmd/tit/main.go` making T056 pass
- [ ] T061 [US3] Implement the schema-conformant JSON encoder in `internal/usage/json.go` making T057 pass (depends on T007)
- [ ] T062 [US3] Implement flag conflict validation and the non-TTY `--watch` rejection in `cmd/tit/main.go` making T058 pass
- [ ] T063 [US3] Implement the Bubble Tea watch model (1 s tick, ≤60 s refresh with per-provider `MinPollInterval` honoring Claude's 180 s floor, alt-screen view, clean quit) in `internal/ui/watch.go` making T059 pass (depends on T044)
- [ ] T064 [US3] Wire `--watch` into the usage command in `cmd/tit/main.go` (depends on T063)
- [ ] T065 [P] [US3] Add a `teatest/v2` smoke test asserting the watch program starts, renders, and quits on `q` in `internal/ui/watch_smoke_test.go`

**Checkpoint**: All three user stories are independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T066 [P] Write tests asserting `NO_COLOR=1` and piped (non-TTY) output contain no ANSI escapes while preserving all content in `internal/ui/nocolor_test.go`
- [ ] T067 [P] Write `README.md` covering install, `tit usage`, flags, per-provider credential prerequisites, and the security posture (read-only, never refreshes tokens)
- [ ] T068 [P] Add build metadata injection (version, commit) to the `Makefile` and CI release step
- [ ] T069 Verify total coverage ≥85% via `make cover` and add targeted tests for any package below threshold
- [ ] T070 Run the full quickstart validation (V1–V6 in [quickstart.md](quickstart.md)) and record results
- [ ] T071 [P] Run `govulncheck ./...` and resolve any findings
- [ ] T072 [P] Verify SC-001 (<3 s one-shot render) with a timing test using deliberately slow fixture servers in `cmd/tit/perf_test.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — start immediately
- **Foundational (Phase 2)**: depends on Setup — **blocks all user stories**
- **US1 (Phase 3)**: depends on Foundational
- **US2 (Phase 4)**: depends on Foundational; T054 touches `internal/ui/render.go`, so it must
  follow T044
- **US3 (Phase 5)**: depends on Foundational; T063 depends on the renderer (T044)
- **Polish (Phase 6)**: depends on the stories you intend to ship

### User Story Dependencies

- **US1 (P1)**: independent — the MVP
- **US2 (P2)**: independent of US1 in logic; shares two files (`cmd/tit/main.go`,
  `internal/ui/render.go`), so sequence those edits rather than running them concurrently
- **US3 (P3)**: independent of US2; reuses US1's renderer and fetch layer

### Within Each User Story

- Test tasks precede their implementation task and must fail first
- Model/types before services before wiring
- Provider tracks A, B, and C never touch each other's files

### Parallel Opportunities

- **Phase 1**: T002–T006 all parallel after T001
- **Phase 2**: T007, T008, T009, T010, T011, T013, T014, T015 all parallel; then T012 and T016
- **Phase 3 — the big win**: three provider tracks (T017–T024, T025–T032, T033–T040) and the
  rendering track (T041–T046) run as **four fully independent lanes**. Within each provider
  lane, the four test tasks are mutually parallel, as are the three source implementations
  (creds/plan/apikey) before the adapter merges them
- **Phase 4**: T049, T050, T051, T055 parallel
- **Phase 5**: T056, T057, T058, T059 parallel; implementations T060–T062 touch
  `cmd/tit/main.go` and must be sequenced
- **Phase 6**: T066, T067, T068, T071, T072 parallel

---

## Parallel Example: Phase 3 with four workers

```bash
# Worker A — Claude lane
T017 → (T018, T019, T020 in parallel) → (T021, T022, T023 in parallel) → T024

# Worker B — Codex lane
T025 → (T026, T027, T028 in parallel) → (T029, T030, T031 in parallel) → T032

# Worker C — Kimi lane
T033 → (T034, T035, T036 in parallel) → (T037, T038, T039 in parallel) → T040

# Worker D — Rendering lane
(T041, T042 in parallel) → T043 → T044 → T045 → T046

# Join: T047 (wiring) requires all four lanes, then T048
```

Maximum useful concurrency is **4 lanes** in Phase 3 (12 concurrent tasks at the test-writing
step). Phases 1, 2, 4, and 6 are mostly single-lane with parallel bursts.

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Phase 1 Setup → Phase 2 Foundational
2. Phase 3 with four parallel lanes
3. **STOP and VALIDATE**: quickstart V1 and V2
4. Ship: `tit usage` already delivers the core promise

**Optional narrower MVP**: implement only the plan sources (T021–T022, T029–T030, T037–T038)
and defer the API-key sources (T023, T031, T039) to a follow-up pass. FR-002 requires both for
completeness, but plan-only is a coherent, demoable checkpoint.

### Incremental Delivery

1. Setup + Foundational → foundation ready
2. US1 → validate → ship MVP
3. US2 → validate → ship (first-contact polish)
4. US3 → validate → ship (automation + watch)
5. Polish → coverage gate, README, quickstart sign-off

### Parallel Team Strategy

Four workers map cleanly onto Phase 3's lanes (Claude, Codex, Kimi, rendering). Before that,
Phases 1–2 are best done by one worker since they define the shared contracts everyone builds
against. Afterward, US2 and US3 can be split between two workers provided edits to
`cmd/tit/main.go` are serialized.

---

## Notes

- `[P]` = different files, no dependency on an incomplete task
- Every provider fixture set must include a malformed-response case — the fragility register in
  [research.md](research.md) is the reason these endpoints get lenient parsing
- No test may require live credentials or network access
- Commit after each task or logical group; every merge to `main` goes through a PR
  (constitution: `main` is LOCKED)
