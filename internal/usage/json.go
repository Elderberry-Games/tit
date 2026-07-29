package usage

import (
	"encoding/json"
	"io"
	"math"
	"time"
)

// The wire types below mirror contracts/usage-json.schema.json exactly. They are
// kept separate from the domain types so that renaming an internal field can
// never silently change the published output format.

type jsonSnapshot struct {
	TakenAt   string         `json:"taken_at"`
	Providers []jsonProvider `json:"providers"`
}

type jsonProvider struct {
	Provider    ProviderID   `json:"provider"`
	DisplayName string       `json:"display_name"`
	Sources     []jsonSource `json:"sources"`
}

type jsonSource struct {
	Source  SourceKind   `json:"source"`
	Status  SourceStatus `json:"status"`
	Detail  string       `json:"detail,omitempty"`
	Windows []jsonWindow `json:"windows"`
	Info    []jsonInfo   `json:"info"`
}

type jsonWindow struct {
	Label           string   `json:"label"`
	DurationSeconds *int64   `json:"duration_seconds"`
	UsedPercent     float64  `json:"used_percent"`
	Used            *float64 `json:"used"`
	Limit           *float64 `json:"limit"`
	Unit            string   `json:"unit"`
	Unlimited       bool     `json:"unlimited"`
	ResetAt         *string  `json:"reset_at"`
	ResetInSeconds  *int64   `json:"reset_in_seconds"`
	Health          Health   `json:"health"`
}

type jsonInfo struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// WriteJSON encodes the snapshot to w in the documented output format.
//
// Every figure visible in the human view is recoverable from this output: the
// contract requires the machine-readable mode to round-trip the dashboard.
func WriteJSON(w io.Writer, s Snapshot) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(toJSON(s))
}

func toJSON(s Snapshot) jsonSnapshot {
	out := jsonSnapshot{
		TakenAt:   s.TakenAt.UTC().Format(time.RFC3339),
		Providers: make([]jsonProvider, 0, len(s.Providers)),
	}

	for _, p := range s.Providers {
		jp := jsonProvider{
			Provider:    p.Provider,
			DisplayName: p.DisplayName,
			Sources:     make([]jsonSource, 0, len(p.Sources)),
		}

		for _, src := range p.Sources {
			js := jsonSource{
				Source:  src.Source,
				Status:  src.Status,
				Detail:  src.Detail,
				Windows: make([]jsonWindow, 0, len(src.Windows)),
				Info:    make([]jsonInfo, 0, len(src.Info)),
			}

			for _, win := range src.Windows {
				js.Windows = append(js.Windows, toJSONWindow(win, s.TakenAt))
			}
			for _, i := range src.Info {
				js.Info = append(js.Info, jsonInfo{Label: i.Label, Value: i.Value})
			}

			jp.Sources = append(jp.Sources, js)
		}

		out.Providers = append(out.Providers, jp)
	}

	return out
}

func toJSONWindow(w LimitWindow, now time.Time) jsonWindow {
	jw := jsonWindow{
		Label:       w.Label,
		UsedPercent: round2(w.UsedPercent),
		Used:        w.Used,
		Limit:       w.Limit,
		Unit:        w.Unit,
		Unlimited:   w.Unlimited,
		Health:      w.Health(),
	}

	if w.Duration > 0 {
		secs := int64(w.Duration.Seconds())
		jw.DurationSeconds = &secs
	}

	if w.ResetAt != nil {
		ts := w.ResetAt.UTC().Format(time.RFC3339)
		jw.ResetAt = &ts

		if in := w.ResetIn(now); in != nil {
			secs := int64(in.Seconds())
			jw.ResetInSeconds = &secs
		}
	}

	return jw
}

// round2 keeps percentages readable without losing meaningful precision.
func round2(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return math.Round(f*100) / 100
}
