# tit

See every AI coding limit in one place.

`tit usage` shows your Claude, Codex, and Kimi rate limits — how much you have
burned through, and when each window comes back — in one terminal view, without
opening three dashboards.

```
Claude  ·  Max plan
  plan    5-hour          ███████████░░░░░░░░░  54%              resets in 2h 14m
  plan    Weekly          ██████████████████░░  91%  ⚠ WARN      resets in 3d 4h

Codex  ·  plus
  plan    5-hour          ████░░░░░░░░░░░░░░░░  21%              resets in 1h 12m
  plan    Weekly          ███████░░░░░░░░░░░░░  36%              resets in 5d 8h

Kimi  ·  Allegretto
  plan    5-hour          ██░░░░░░░░░░░░░░░░░░  12%  24/200      resets in 4h 1m
  plan    Weekly          █████░░░░░░░░░░░░░░░  27%  553/2,048   resets in 2d 6h
```

## Install

```sh
go install github.com/Elderberry-Games/tit/cmd/tit@latest
```

Or build from a clone:

```sh
make build   # produces ./tit
```

## Use

```sh
tit usage                 # everything, in one view
tit usage claude          # just one provider
tit usage --json          # machine-readable, for scripts and status bars
tit usage --watch         # live view; countdowns tick, data refreshes
```

Exit codes: `0` everything worked, `1` every source that was tried failed, `2`
you asked for something that does not exist, `3` some worked and some did not.

## Configuration

There is none, by design. tit reads the credentials your provider CLIs already
store, so if `claude`, `codex`, or `kimi` works on this machine, tit works too.

Optional environment variables let you add or override sources:

| Variable | Effect |
| --- | --- |
| `CLAUDE_CODE_OAUTH_TOKEN` | Overrides the cached Claude credential |
| `ANTHROPIC_API_KEY` | Adds Claude API-key rate limits |
| `CODEX_HOME` | Where to find Codex's `auth.json` (default `~/.codex`) |
| `OPENAI_API_KEY` | Adds Codex API-key rate limits |
| `KIMI_CODE_API_KEY` | Overrides the cached Kimi credential |
| `MOONSHOT_API_KEY` | Adds your Moonshot balance |
| `MOONSHOT_REGION` | `cn` for the mainland host, otherwise global |
| `NO_COLOR` | Turns off styling |

A provider with no credential is reported as "not configured" — never as an
error, and never in a way that hides the providers that do work.

One cost note: the `OPENAI_API_KEY` source sends a one-token probe request to
read the rate-limit headers, which costs a fraction of a cent per run. Leave the
variable unset to skip it entirely.

## Security

- **Read-only.** tit reads credential files; it never writes them.
- **Never refreshes tokens.** Refresh tokens rotate, and refreshing behind your
  CLI's back would invalidate its session. When a credential expires, tit tells
  you which command to run.
- **Never prints secrets.** Every user-facing message is built from a fixed set
  of templates that take provider names and file paths only.
- **Usage metadata only.** tit never reads, stores, or transmits prompt or
  completion content.

## Caveats

The subscription-limit endpoints these three providers use are internal and
undocumented. They can change without notice. tit parses them leniently and
isolates failures per provider, so a broken endpoint degrades to one explanatory
line rather than taking the whole view down — but expect the occasional
"provider returned an unreadable response" until an update lands.

Kimi's access tokens expire after about 15 minutes, so its plan source often
needs a fresh `kimi` run before it will report.

## Development

```sh
make all     # gofmt, vet, govulncheck, tests
make cover   # tests with the 85% coverage gate
```

Design documents live in [`specs/001-tit-usage-cli/`](specs/001-tit-usage-cli/);
the project's engineering rules are in
[`.specify/memory/constitution.md`](.specify/memory/constitution.md).
`main` is protected: all changes land through a pull request.
