package provider

import (
	"testing"

	"github.com/Elderberry-Games/tit/internal/usage"
)

func TestAllReturnsEveryProviderInDisplayOrder(t *testing.T) {
	got := All()

	want := []usage.ProviderID{usage.ProviderClaude, usage.ProviderCodex, usage.ProviderKimi}
	if len(got) != len(want) {
		t.Fatalf("All() returned %d providers, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID() != id {
			t.Errorf("All()[%d] = %q, want %q", i, got[i].ID(), id)
		}
		if got[i].DisplayName() == "" {
			t.Errorf("provider %q has an empty display name", id)
		}
	}
}

func TestLookupIsCaseInsensitive(t *testing.T) {
	for _, name := range []string{"claude", "Claude", "CLAUDE", "  codex  ", "Kimi"} {
		t.Run(name, func(t *testing.T) {
			if _, ok := Lookup(name); !ok {
				t.Errorf("Lookup(%q) failed, want a match", name)
			}
		})
	}
}

func TestLookupRejectsUnknownNames(t *testing.T) {
	for _, name := range []string{"gemini", "", "clau"} {
		if p, ok := Lookup(name); ok {
			t.Errorf("Lookup(%q) = %q, want no match", name, p.ID())
		}
	}
}

func TestNamesAreSorted(t *testing.T) {
	got := Names()

	want := []string{"claude", "codex", "kimi"}
	if len(got) != len(want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Names()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestProvidersDeclareAPollInterval(t *testing.T) {
	for _, p := range All() {
		if p.MinPollInterval() <= 0 {
			t.Errorf("provider %q declares a non-positive poll interval", p.ID())
		}
	}
}
