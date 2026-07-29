package main

import (
	"bytes"
	"strings"
	"testing"
)

// exec runs the CLI with the given arguments and captures both streams.
// Only argument-handling paths are exercised here: none of these invocations
// reach the network.
func exec(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestNoArgumentsShowsHelp(t *testing.T) {
	code, stdout, stderr := exec(t)

	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "usage") {
		t.Errorf("help output does not describe the usage command:\n%s", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestHelpAliases(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, _ := exec(t, arg)
			if code != exitOK {
				t.Errorf("exit code = %d, want %d", code, exitOK)
			}
			if !strings.Contains(stdout, "Commands:") {
				t.Errorf("%q did not print the command list", arg)
			}
		})
	}
}

func TestVersionAliases(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, _ := exec(t, arg)
			if code != exitOK {
				t.Errorf("exit code = %d, want %d", code, exitOK)
			}
			if !strings.HasPrefix(stdout, "tit ") {
				t.Errorf("version output = %q, want it to start with %q", stdout, "tit ")
			}
		})
	}
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	code, stdout, stderr := exec(t, "bogus")

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (errors belong on stderr)", stdout)
	}
	if !strings.Contains(stderr, "bogus") {
		t.Errorf("error does not name the offending command:\n%s", stderr)
	}
	if !strings.Contains(stderr, "usage") {
		t.Errorf("error does not suggest valid commands:\n%s", stderr)
	}
}

func TestUnknownFlagIsAUsageError(t *testing.T) {
	code, _, stderr := exec(t, "usage", "--nope")

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "--nope") {
		t.Errorf("error does not name the offending flag:\n%s", stderr)
	}
}

func TestUnknownProviderIsAUsageError(t *testing.T) {
	code, _, stderr := exec(t, "usage", "gemini")

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	for _, want := range []string{"gemini", "claude", "codex", "kimi"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("error message missing %q:\n%s", want, stderr)
		}
	}
}

func TestJSONAndWatchConflict(t *testing.T) {
	code, _, stderr := exec(t, "usage", "--json", "--watch")

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "--json") || !strings.Contains(stderr, "--watch") {
		t.Errorf("error does not explain the conflict:\n%s", stderr)
	}
}

func TestWatchRequiresATerminal(t *testing.T) {
	// stdout here is a buffer, never a TTY.
	code, _, stderr := exec(t, "usage", "--watch")

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "terminal") {
		t.Errorf("error does not explain the terminal requirement:\n%s", stderr)
	}
}

func TestTooManyProvidersIsAUsageError(t *testing.T) {
	code, _, stderr := exec(t, "usage", "claude", "codex")

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "codex") {
		t.Errorf("error does not name the extra argument:\n%s", stderr)
	}
}

func TestUsageHelp(t *testing.T) {
	code, stdout, _ := exec(t, "usage", "--help")

	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	for _, want := range []string{"--json", "--watch", "Exit codes", "NO_COLOR"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("usage help missing %q:\n%s", want, stdout)
		}
	}
}

func TestUsageHelpDisclosesProbeCost(t *testing.T) {
	_, stdout, _ := exec(t, "usage", "--help")

	// The OpenAI API-key source spends a fraction of a cent per run; users must
	// be able to discover that without reading the source.
	if !strings.Contains(stdout, "OPENAI_API_KEY") || !strings.Contains(stdout, "cent") {
		t.Errorf("usage help does not disclose the API-key probe cost:\n%s", stdout)
	}
}

func TestErrorMessagesAreFriendly(t *testing.T) {
	// No user-facing error may look like a stack trace or a bare error value.
	cases := [][]string{
		{"bogus"},
		{"usage", "--nope"},
		{"usage", "gemini"},
		{"usage", "--json", "--watch"},
	}

	for _, args := range cases {
		_, _, stderr := exec(t, args...)
		if stderr == "" {
			t.Errorf("args %v produced no explanation", args)
			continue
		}
		for _, forbidden := range []string{"goroutine", "panic:", "0x"} {
			if strings.Contains(stderr, forbidden) {
				t.Errorf("args %v produced an unfriendly error containing %q:\n%s", args, forbidden, stderr)
			}
		}
		if !strings.HasPrefix(stderr, "tit") {
			t.Errorf("args %v: error should identify the tool:\n%s", args, stderr)
		}
	}
}
