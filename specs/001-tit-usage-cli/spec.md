# Feature Specification: tit — Unified AI Usage-Limit Monitor (CLI)

**Feature Branch**: `001-tit-usage-cli`

**Created**: 2026-07-29

**Status**: Draft

**Input**: User description: "build initial version of a command line tool called \"tit\" that has subcommands such as \"tit usage\" etc. the tit usage must show the limits of claude, codex and kimi in a unified way in the terminal. make it look slick. consult mobbin mcp for looks." Clarified: the initial version is a terminal-only command line tool — no web UI, no GUI, nothing beyond the terminal.

## Clarifications

### Session 2026-07-29

- Q: Which kind of limits should `tit usage` monitor, and where does the usage data come from? → A: Both, merged — subscription/coding-plan windows (via each coding agent's existing local credentials) and developer API-key rate limits (via locally available API keys), shown together per provider.
- Q: At what percentage used should a limit window switch from "healthy" to "warning"? → A: 90%.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See all my limits at a glance (Priority: P1)

A developer who uses Claude, Codex, and Kimi throughout the day runs `tit usage` and instantly
sees, in one unified terminal view, how much of each provider's rate limits they have consumed,
what each limit window is (hourly, weekly, or other), and when each window resets. They never
have to open three different dashboards or guess when they'll be unblocked.

**Why this priority**: This is the entire reason the tool exists — one command, one view, all
providers. Everything else is scaffolding around this moment.

**Independent Test**: Can be fully tested by running `tit usage` with all three providers
configured (using recorded/fake provider responses) and verifying every provider's windows,
usage figures, and reset times appear in one coherent view.

**Acceptance Scenarios**:

1. **Given** valid credentials for Claude, Codex, and Kimi, **When** the user runs `tit usage`,
   **Then** the terminal shows one unified view with a clearly labeled section per provider,
   each limit window's consumed/total amounts, percentage used, and time until reset.
2. **Given** a limit window at or above 90% consumed, **When** the user runs `tit usage`,
   **Then** that window is visually distinguished (warning state) so it stands out from healthy
   windows at a glance.
3. **Given** a fully exhausted limit window, **When** the user runs `tit usage`, **Then** the
   view shows it as exhausted with the exact time remaining until the window resets.
4. **Given** one provider is unreachable or errors, **When** the user runs `tit usage`,
   **Then** the other providers' data still renders normally and the failing provider shows a
   friendly, human-readable explanation in its section.

---

### User Story 2 - Discover and trust the tool (Priority: P2)

A first-time user runs `tit` with no arguments (or `tit --help`) and immediately understands
what the tool does, what subcommands exist, and what to do next. If a provider isn't set up,
the tool tells them exactly what's missing instead of failing cryptically.

**Why this priority**: A CLI lives or dies on first contact. The unified view (P1) is useless
if users can't discover it or diagnose why a provider shows nothing.

**Independent Test**: Can be fully tested by running `tit`, `tit --help`, `tit --version`, an
unknown subcommand, and `tit usage` with zero providers configured, and verifying each produces
helpful, well-formatted guidance.

**Acceptance Scenarios**:

1. **Given** a fresh install, **When** the user runs `tit` or `tit --help`, **Then** they see a
   short description, the list of subcommands (including `usage`), and usage examples.
2. **Given** an unknown subcommand (e.g., `tit bogus`), **When** it is run, **Then** the tool
   exits with a non-zero status and a friendly message suggesting valid subcommands.
3. **Given** no providers are configured, **When** the user runs `tit usage`, **Then** the tool
   explains per provider what credential/configuration is missing and how to supply it.
4. **Given** any error condition, **When** it surfaces to the user, **Then** the message is
   human-readable — never a raw stack trace or bare error code.

---

### User Story 3 - Focus and automate (Priority: P3)

A power user checks a single provider (`tit usage claude`) when they only care about one limit,
and pipes machine-readable output (`tit usage --json`) into scripts, status bars, or prompts.

**Why this priority**: Valuable quality-of-life additions once the unified view works, and they
unlock integrations (shell prompts, tmux status lines) that multiply the tool's usefulness.

**Independent Test**: Can be fully tested by running `tit usage <provider>` and `tit usage
--json` and verifying filtered output and well-formed structured output respectively.

**Acceptance Scenarios**:

1. **Given** configured providers, **When** the user runs `tit usage claude`, **Then** only the
   Claude section renders, with the same detail as in the unified view.
2. **Given** configured providers, **When** the user runs `tit usage --json`, **Then** the tool
   emits well-formed structured output containing every provider, window, usage figure, and
   reset time shown in the human view, with no decorative styling.
3. **Given** `--json` output is piped to another program, **When** parsed, **Then** it contains
   machine-usable reset timestamps and numeric usage values.

---

### Edge Cases

- One provider times out or returns garbage: its section shows a friendly failure note; other
  providers render normally; exit status reflects partial failure distinctly from total success.
- Credentials exist but are expired/invalid: the provider's section says so explicitly and
  points at how to re-authenticate — the message never includes the credential itself.
- A provider reports a window the tool doesn't recognize (new window type): it is displayed
  generically (name, usage, reset) rather than dropped.
- A provider reports "unlimited" or absent limits: shown as such, not as 0% or an error.
- Terminal is narrow (e.g., 60 columns): the layout degrades gracefully without broken bars or
  wrapped garbage.
- Output is redirected to a file/pipe, or `NO_COLOR` is set: decorative color/styling is
  disabled automatically; content remains complete and readable.
- All limit windows are fully exhausted across all providers: the view makes the earliest
  upcoming reset obvious so the user knows when they can work again.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The tool MUST be invoked as `tit` and organize functionality into subcommands;
  the initial release MUST include `usage`, `--help`/`help`, and `--version`.
- **FR-002**: `tit usage` MUST display, in a single unified terminal view, the current usage
  and limits for Claude, Codex, and Kimi. For each provider this covers BOTH limit sources,
  merged into the provider's section: (a) subscription/coding-plan windows obtained via the
  provider's coding agent's existing local credentials, and (b) developer API-key rate limits
  obtained via locally available API keys. Each window MUST be labeled with its source (plan
  vs. API key); if only one source is available locally, the section shows that source alone
  without treating the other's absence as an error.
- **FR-003**: For every limit window a provider exposes (hourly, weekly, or other), the view
  MUST show: the window's name/duration, amount consumed, total allowance (when known),
  percentage consumed, and time remaining until the window resets.
- **FR-004**: All providers MUST be presented in a visually consistent, unified format — same
  layout, same ordering of elements, same styling rules — regardless of how each provider
  reports its data.
- **FR-005**: Each limit window MUST have a visual health state: healthy (below 90% consumed),
  warning (at or above 90% consumed), and exhausted (100% consumed or provider reports the
  window as blocked); states MUST be distinguishable by more than color alone (e.g., labels or
  symbols) so they survive colorless output.
- **FR-006**: The view MUST include, per window, a graphical fill indicator (progress-bar-like)
  aligned consistently across providers, with the numeric "consumed / total" and reset
  countdown adjacent to it (visual language modeled on the referenced usage dashboards: aligned
  rows, per-window bars, "x / y" labels, reset chips — all rendered purely as terminal text).
- **FR-007**: A provider that is unconfigured, unreachable, or erroring MUST NOT prevent the
  other providers from rendering; its section MUST show a concise, human-readable reason and,
  where applicable, how to fix it.
- **FR-008**: `tit usage` MUST support filtering to a single named provider.
- **FR-009**: `tit usage` MUST support a machine-readable structured output mode that contains
  all data from the human view (providers, windows, usage, totals, reset times) without
  decorative styling.
- **FR-010**: The tool MUST auto-detect non-interactive output (pipe/redirect) and the
  `NO_COLOR` convention, disabling decorative styling while preserving all information.
- **FR-011**: The tool MUST use each provider's existing locally available authentication and
  MUST NOT require users to paste credentials into the tool; credentials MUST never appear in
  output, logs, or error messages.
- **FR-012**: The tool MUST only read usage/limit metadata; it MUST NOT read, store, or
  transmit prompt or completion content.
- **FR-013**: Exit status MUST distinguish: full success, partial provider failure, and total
  failure, so scripts can react appropriately.
- **FR-014**: Unknown subcommands or flags MUST produce a friendly error, a suggestion of valid
  options, and a non-zero exit status.
- **FR-015**: The tool MUST work without any configuration file; sensible defaults MUST cover
  the first-run experience.

### Key Entities

- **Provider**: An AI service whose limits are monitored (initially Claude, Codex, Kimi).
  Attributes: display name, configuration/authentication state, reachability.
- **Limit Window**: A single rate-limit bucket a provider enforces. Attributes: source
  (subscription/plan vs. API key), name/duration (e.g., hourly, weekly, other), consumed
  amount, total allowance (may be unknown or unlimited), reset time, derived health state
  (healthy/warning/exhausted).
- **Usage Snapshot**: The result of one `tit usage` run — per-provider collections of limit
  windows plus per-provider fetch status (ok / unconfigured / error with reason) and the
  moment the snapshot was taken.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with all three providers configured sees the complete unified view with a
  single command, with results rendered in under 3 seconds on a typical connection.
- **SC-002**: A user can identify their most-constrained limit (which window, how full, when it
  resets) within 5 seconds of the output appearing, without reading any documentation.
- **SC-003**: A first-time user can go from installed binary to understanding what `tit` does
  and running `usage` successfully using only the tool's own help output.
- **SC-004**: 100% of user-facing failure modes (missing credentials, provider outage, unknown
  command) produce a specific, human-readable message that names the problem and a next step.
- **SC-005**: One provider failing never hides another provider's data (verified across all
  single-provider failure combinations).
- **SC-006**: The structured output mode round-trips: every figure visible in the human view is
  recoverable from the machine-readable output by an automated consumer.

## Visual & Experience References

Design language for the unified view, drawn from Mobbin research and translated to terminal
idioms (text, alignment, and color only — this tool is terminal-only; these are references, not
implementation mandates):

- Google AI Studio — Gemini API Rate Limit ([screen](https://mobbin.com/screens/0e18ec73-ca54-4674-b255-a1be428d78d5),
  [tier variant](https://mobbin.com/screens/b1ca0689-a62c-4475-8e1b-603644eb37f2)): one row per
  model with aligned columns per window type and compact "0 / 25"-style bars — the model for
  tit's aligned per-window rows.
- StackAI — Usage ([screen](https://mobbin.com/screens/a2976bbf-d4f1-407f-8c56-88779215f037)):
  segmented fill bars with percentage and right-aligned "consumed / total" labels — the model
  for tit's bar + numbers arrangement.
- Glide — Usage ([screen](https://mobbin.com/screens/ce19934a-3ebb-4376-8eb5-3ecb62da0aac)):
  "Resets in 7 days" chip adjacent to the bar — the model for tit's reset countdown placement.
- Grok — Rate Limits ([screen](https://mobbin.com/screens/2c310b58-3b8f-4130-912f-a5e237b05858)):
  grouping of per-model limits under a clear account-level header — the model for tit's
  provider section headers.

The overall feel MUST be "slick": generous alignment, restrained color used for meaning (health
states), playful but precise copy, and zero visual noise — consistent with the constitution's
"Fun Is a Feature" and "Simple as Fuck" principles.

## Assumptions

- The initial version is exclusively a terminal CLI — no web UI, no GUI, no server component.
- Users already use the three providers' own tooling on the same machine, so provider
  authentication material (session credentials, API keys, or local config) already exists
  locally; tit reuses it read-only and never prompts users to enter secrets.
- "Limits" covers both subscription/coding-plan rate-limit windows and developer API-key rate
  limits (hourly, weekly, and any other windows a provider exposes), merged per provider;
  billing/spend tracking remains out of scope for v1.
- The unified view is a one-shot snapshot per invocation; live auto-refreshing (watch mode) is
  out of scope for v1.
- Provider set is fixed at Claude, Codex, and Kimi for v1; adding providers is a future
  feature.
- The tool targets a single local user per machine; multi-account switching per provider is out
  of scope for v1.
- English-only output for v1.
- If a provider offers no queryable limit data at all, tit reports that honestly in the
  provider's section rather than inventing figures.
