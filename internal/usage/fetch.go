package usage

import (
	"context"
	"sort"
	"sync"
	"time"
)

// DefaultTimeout bounds each provider's fetch so that one slow provider cannot
// push the whole view past its 3-second budget.
const DefaultTimeout = 2500 * time.Millisecond

// Fetch queries every provider concurrently and assembles a Snapshot.
//
// Each provider gets its own timeout-bounded context, so a hanging provider
// degrades to an unavailable source rather than stalling the run. Providers are
// returned in the order given, independent of completion order, so output is
// stable.
func Fetch(ctx context.Context, providers []Provider, now Clock, timeout time.Duration) Snapshot {
	if now == nil {
		now = SystemClock
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	results := make([]ProviderResult, len(providers))
	var wg sync.WaitGroup

	for i, p := range providers {
		wg.Add(1)
		go func(i int, p Provider) {
			defer wg.Done()

			pctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			results[i] = ProviderResult{
				Provider:    p.ID(),
				DisplayName: p.DisplayName(),
				Sources:     sortSources(p.Fetch(pctx, now)),
			}
		}(i, p)
	}

	wg.Wait()

	return Snapshot{TakenAt: now(), Providers: results}
}

// sortSources keeps plan sources above API-key sources so every provider
// section reads the same way.
func sortSources(sources []SourceResult) []SourceResult {
	sort.SliceStable(sources, func(i, j int) bool {
		return sources[i].Source == SourcePlan && sources[j].Source != SourcePlan
	})
	return sources
}

// Filter returns a copy of the snapshot containing only the named provider.
// The second return value reports whether the provider was found.
func (s Snapshot) Filter(id ProviderID) (Snapshot, bool) {
	for _, p := range s.Providers {
		if p.Provider == id {
			return Snapshot{TakenAt: s.TakenAt, Providers: []ProviderResult{p}}, true
		}
	}
	return Snapshot{TakenAt: s.TakenAt}, false
}
