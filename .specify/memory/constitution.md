<!--
Sync Impact Report
- Version change: 1.0.0 → 1.1.0
- Modified principles: none renamed or redefined
- Amended sections:
  - Development Workflow & Quality Gates — added mandatory branch protection rules:
    main is locked, no direct pushes, all changes merge via pull request with passing gates
- Added sections: none
- Removed sections: none
- Templates:
  - ✅ .specify/templates/plan-template.md — Constitution Check gates are derived from this file at
    /speckit-plan time; no edits required
  - ✅ .specify/templates/spec-template.md — no constitution-specific sections; aligned
  - ✅ .specify/templates/tasks-template.md — no constitution-specific sections; aligned
- Follow-up TODOs: none
-->

# Tit Constitution

## Core Principles

### I. Security First

Security is the first consideration in every design and code decision, not a hardening pass at
the end. Non-negotiable rules:

- Secrets (API keys, tokens, credentials) MUST never be committed, logged, or printed. They are
  read from the environment or OS keychain/config files with restrictive permissions (0600).
- All external input (API responses, config files, CLI flags) MUST be validated before use.
- Dependencies MUST be vetted and kept minimal; known-vulnerable dependencies MUST be updated or
  removed before merge (`govulncheck` clean).
- Network calls MUST use TLS; no insecure transport, no certificate-check bypasses.
- A change that weakens any of the above MUST be rejected in review regardless of its benefits.

**Rationale**: This app handles account-scoped API credentials for multiple AI providers; a leak
is the worst possible failure mode and cannot be walked back.

### II. Tests or It Didn't Happen (≥85% Coverage)

Every feature and bugfix ships with tests. Non-negotiable rules:

- Total test coverage MUST be ≥85% (`go test -cover`), enforced as a CI gate; a change that drops
  coverage below 85% MUST NOT merge.
- Tests MUST be written before or alongside implementation, never "later".
- Provider integrations MUST be testable without live credentials (fakes/fixtures for API
  responses).
- A bugfix MUST include a regression test that fails without the fix.

**Rationale**: A monitoring tool that silently reports wrong limits is worse than no tool; tests
are the only proof it works.

### III. Go, and Only Go

Go is the sole implementation language. Non-negotiable rules:

- All application code MUST be Go (latest stable release); no CGo unless technically unavoidable
  and justified in writing.
- The standard library is preferred; every third-party dependency MUST justify its existence.
- Code MUST pass `gofmt`, `go vet`, and build with zero warnings.
- The deliverable is a single static binary; no runtime dependencies, no interpreters.

**Rationale**: One language, one binary, one toolchain — maximum portability and minimum
maintenance for a small tool.

### IV. Fun Is a Feature

The app must be fun to use — delight is a requirement, not a garnish. Non-negotiable rules:

- Output MUST have personality: playful copy, color, and satisfying presentation of limits and
  countdowns are part of the spec, not polish debt.
- Error messages MUST be helpful and human, never raw stack traces or bare error codes.
- Fun MUST never compromise clarity: the numbers (usage, limits, reset times) stay accurate and
  instantly readable.

**Rationale**: People check usage limits many times a day; a tool used that often should spark
joy, or it will be abandoned.

### V. Simple as Fuck

Radical simplicity wins every trade-off that Principles I–IV do not claim first. Non-negotiable
rules:

- The app does one thing: monitor usage and expiration of rate limits. Feature requests outside
  that scope are rejected by default (YAGNI).
- No speculative abstractions: no plugin systems, no interfaces with a single implementation, no
  config options nobody asked for.
- Prefer the boring solution; if two designs work, ship the one with less code.
- Every added abstraction or dependency MUST be justified against a concrete, current need.

**Rationale**: Simplicity is the cheapest form of security, testability, and fun — and small
tools die from complexity, not from missing features.

## Additional Constraints & Product Boundaries

- **Scope**: the app monitors usage and expiration of hourly, weekly, and other rate-limit
  windows for AI coding models. Initial providers: Claude, Codex, Kimi.
- **Extensibility boundary**: adding a provider MUST NOT require touching unrelated providers;
  keep provider code isolated but do not build a plugin framework for it (Principle V).
- **Data handling**: only usage/limit metadata is stored or displayed; no prompt or completion
  content is ever read, stored, or transmitted.
- **Toolchain**: Go (latest stable), `gofmt` + `go vet` + `govulncheck` + `go test -cover` as the
  standard local and CI pipeline.

## Development Workflow & Quality Gates

- **`main` is LOCKED, always.** Direct pushes to `main` are prohibited — no exceptions, not for
  hotfixes, not for docs, not for one-liners. The branch protection setting itself MUST stay
  enabled; disabling it to "push something quickly" is a constitution violation.
- **PR to win merge.** Every change reaches `main` exclusively through a pull request, and a PR
  merges only when all quality gates below pass.
- Every change goes through the Spec Kit flow (specify → plan → tasks → implement) for features;
  trivial fixes may skip straight to tested commits — but still land via PR.
- Merge gates, all mandatory: build passes, `gofmt`/`go vet` clean, `govulncheck` clean, tests
  pass, coverage ≥85%.
- Code review MUST verify compliance with this constitution; a reviewer citing a principle
  overrules preference-based objections.
- Violations found post-merge are fixed forward immediately or the change is reverted.

## Governance

This constitution supersedes all other practices in this repository. Amendments require a
documented rationale, a semantic version bump of this file, and propagation to dependent
templates (`.specify/templates/*.md`).

Versioning policy: MAJOR for principle removals or incompatible redefinitions, MINOR for new or
materially expanded principles/sections, PATCH for clarifications and wording fixes.

Compliance review: every `/speckit-plan` Constitution Check and every code review MUST validate
changes against the principles above; unjustifiable complexity is grounds for rejection.

**Version**: 1.1.0 | **Ratified**: 2026-07-29 | **Last Amended**: 2026-07-29
