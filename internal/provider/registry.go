// Package provider assembles the fixed set of providers tit knows about.
//
// The set is deliberately hardcoded: three providers do not justify a plugin
// system, and each adapter stays isolated in its own package.
package provider

import (
	"sort"
	"strings"

	"github.com/Elderberry-Games/tit/internal/provider/claude"
	"github.com/Elderberry-Games/tit/internal/provider/codex"
	"github.com/Elderberry-Games/tit/internal/provider/kimi"
	"github.com/Elderberry-Games/tit/internal/usage"
)

// All returns every provider, in the order they should be displayed.
func All() []usage.Provider {
	return []usage.Provider{
		claude.New(),
		codex.New(),
		kimi.New(),
	}
}

// Lookup resolves a provider by its case-insensitive name.
func Lookup(name string) (usage.Provider, bool) {
	want := usage.ProviderID(strings.ToLower(strings.TrimSpace(name)))
	for _, p := range All() {
		if p.ID() == want {
			return p, true
		}
	}
	return nil, false
}

// Names returns the sorted list of valid provider names, for error messages.
func Names() []string {
	var names []string
	for _, p := range All() {
		names = append(names, string(p.ID()))
	}
	sort.Strings(names)
	return names
}
