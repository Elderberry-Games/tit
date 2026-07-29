// Command tit shows your AI coding limits in one place.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"

	"github.com/Elderberry-Games/tit/internal/provider"
	"github.com/Elderberry-Games/tit/internal/ui"
	"github.com/Elderberry-Games/tit/internal/usage"
	"github.com/Elderberry-Games/tit/internal/version"
)

// Exit codes. See specs/001-tit-usage-cli/contracts/cli.md.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// defaultWidth is used when stdout is not a terminal.
const defaultWidth = 80

// Indirection so tests can drive the command with stub providers instead of
// reaching the network.
var (
	allProviders   = provider.All
	lookupProvider = provider.Lookup
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printRootHelp(stdout)
		return exitOK
	}

	switch args[0] {
	case "help", "--help", "-h":
		printRootHelp(stdout)
		return exitOK
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version.String())
		return exitOK
	case "usage":
		return runUsage(args[1:], stdout, stderr)
	}

	fmt.Fprintf(stderr, "tit: unknown command %q\n\nAvailable commands: usage, help, version.\nRun `tit help` to see what each one does.\n", args[0])
	return exitUsage
}

func runUsage(args []string, stdout, stderr io.Writer) int {
	var (
		asJSON   bool
		watch    bool
		filter   string
		wantHelp bool
	)

	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "--watch":
			watch = true
		case arg == "-h" || arg == "--help":
			wantHelp = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(stderr, "tit usage: unknown flag %q\n\nSupported flags: --json, --watch, --help.\n", arg)
			return exitUsage
		case filter == "":
			filter = arg
		default:
			fmt.Fprintf(stderr, "tit usage: unexpected argument %q — only one provider can be named at a time.\n", arg)
			return exitUsage
		}
	}

	if wantHelp {
		printUsageHelp(stdout)
		return exitOK
	}

	if asJSON && watch {
		fmt.Fprintln(stderr, "tit usage: --json and --watch cannot be combined — one is a snapshot, the other never stops.")
		return exitUsage
	}

	providers := allProviders()
	if filter != "" {
		p, ok := lookupProvider(filter)
		if !ok {
			fmt.Fprintf(stderr, "tit usage: unknown provider %q\n\nKnown providers: %s.\n", filter, strings.Join(provider.Names(), ", "))
			return exitUsage
		}
		providers = []usage.Provider{p}
	}

	if watch {
		if !isTerminal(stdout) {
			fmt.Fprintln(stderr, "tit usage: --watch needs a terminal — drop the flag when redirecting output.")
			return exitUsage
		}
		if err := ui.Watch(providers); err != nil {
			fmt.Fprintf(stderr, "tit usage: the live view stopped unexpectedly (%v)\n", err)
			return exitFailure
		}
		return exitOK
	}

	snapshot := usage.Fetch(context.Background(), providers, usage.SystemClock, usage.DefaultTimeout)

	if asJSON {
		if err := usage.WriteJSON(stdout, snapshot); err != nil {
			fmt.Fprintf(stderr, "tit usage: could not write JSON (%v)\n", err)
			return exitFailure
		}
		return snapshot.ExitCode()
	}

	out := colorprofile.NewWriter(stdout, os.Environ())
	fmt.Fprintln(out, ui.Render(snapshot, widthOf(stdout), time.Now()))

	return snapshot.ExitCode()
}

// isTerminal reports whether w is an interactive terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

// widthOf returns the usable terminal width, falling back to a sane default
// when the output is redirected.
func widthOf(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(f.Fd()) {
		return defaultWidth
	}
	width, _, err := term.GetSize(f.Fd())
	if err != nil || width <= 0 {
		return defaultWidth
	}
	return width
}
