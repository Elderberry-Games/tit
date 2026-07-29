package main

import (
	"fmt"
	"io"
)

const rootHelp = `tit — see every AI coding limit in one place.

Usage:
  tit <command> [flags]

Commands:
  usage      Show your Claude, Codex, and Kimi limits
  help       Show this help
  version    Show the version

Examples:
  tit usage                 Everything, in one view
  tit usage claude          Just one provider
  tit usage --json          Machine-readable, for scripts and status bars
  tit usage --watch         Live view that keeps ticking

tit reads the credentials your provider CLIs already store. It never asks you to
paste a token, never writes to those files, and only ever reads usage metadata.
`

const usageHelp = `tit usage — show your Claude, Codex, and Kimi limits.

Usage:
  tit usage [provider] [flags]

Providers:
  claude, codex, kimi       Show only that provider (case-insensitive)

Flags:
  --json                    Emit a JSON snapshot instead of the dashboard
  --watch                   Live view: countdowns tick, data refreshes
  -h, --help                Show this help

Exit codes:
  0   everything worked
  1   every source that was tried failed
  2   you asked for something that does not exist
  3   some sources worked, some failed

Credentials (all optional, all read-only):
  CLAUDE_CODE_OAUTH_TOKEN   overrides the cached Claude credential
  ANTHROPIC_API_KEY         adds Claude API-key limits
  CODEX_HOME                where to find Codex's auth.json (default ~/.codex)
  OPENAI_API_KEY            adds Codex API-key limits (costs a fraction of a
                            cent per run: it sends a 1-token probe request)
  KIMI_CODE_API_KEY         overrides the cached Kimi credential
  MOONSHOT_API_KEY          adds your Moonshot balance
  MOONSHOT_REGION           "cn" for the mainland host, otherwise global
  NO_COLOR                  turn off styling
`

func printRootHelp(w io.Writer) {
	fmt.Fprint(w, rootHelp)
}

func printUsageHelp(w io.Writer) {
	fmt.Fprint(w, usageHelp)
}
