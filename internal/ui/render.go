// Package ui renders a usage.Snapshot as a terminal dashboard. It has no
// provider or network knowledge: it only formats the normalized model.
package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Elderberry-Games/tit/internal/usage"
)

const (
	indent    = "  "
	columnGap = "  "

	barWidthMax = 20
	barWidthMin = 10

	// compactBelow is the width under which the layout switches to a compact
	// two-line-per-window form.
	compactBelow = 50
)

// Render returns the complete dashboard as a string. It is pure: the same
// snapshot, width, and now always produce byte-identical output.
func Render(s usage.Snapshot, width int, now time.Time) string {
	if width <= 0 {
		width = 80
	}
	sections := buildSections(s)
	l := computeLayout(sections, width, now)

	var lines []string
	for i, sec := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, renderHeader(sec, width))
		for _, r := range sec.rows {
			if l.compact {
				lines = append(lines, renderCompactRow(r, now, width, l)...)
			} else {
				lines = append(lines, renderWindowRow(r, now, l))
			}
		}
		for _, n := range sec.notices {
			lines = append(lines, renderNotice(n, width, l.tagW)...)
		}
	}
	if f := renderFooter(s, now); f != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, f)
	}

	// Safety net: no line may ever exceed the requested width.
	for i, ln := range lines {
		if lipgloss.Width(ln) > width {
			lines[i] = ansi.Truncate(ln, width, "")
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// windowRow is one limit window to render, tagged with its source.
type windowRow struct {
	tag    string
	window usage.LimitWindow
}

// notice is a non-OK source rendered as a single calm detail line.
type notice struct {
	tag    string
	detail string
}

// section is everything rendered under one provider header.
type section struct {
	title    string
	subtitle string
	rows     []windowRow
	notices  []notice
}

func buildSections(s usage.Snapshot) []section {
	sections := make([]section, 0, len(s.Providers))
	for _, p := range s.Providers {
		sec := section{title: p.DisplayName}
		if sec.title == "" {
			sec.title = string(p.Provider)
		}
		// A failing source must never set the provider's subtitle while another
		// source is working: showing "not configured" above live numbers reads
		// as a contradiction.
		anyOK := false
		for _, src := range p.Sources {
			if src.Status == usage.StatusOK {
				anyOK = true
				break
			}
		}
		for _, src := range p.Sources {
			tag := tagFor(src.Source)
			switch {
			case src.Status == usage.StatusOK && len(src.Windows) > 0:
				for _, w := range src.Windows {
					sec.rows = append(sec.rows, windowRow{tag: tag, window: w})
				}
				if sec.subtitle == "" && src.Detail != "" {
					sec.subtitle = src.Detail
				}
			case src.Status == usage.StatusOK:
				// OK with no windows: nothing to list, but the detail may
				// still carry the plan name.
				if sec.subtitle == "" && src.Detail != "" {
					sec.subtitle = src.Detail
				}
			default:
				detail := src.Detail
				if detail == "" {
					detail = statusWord(src.Status)
				}
				sec.notices = append(sec.notices, notice{tag: tag, detail: detail})
				if sec.subtitle == "" && !anyOK {
					sec.subtitle = statusWord(src.Status)
				}
			}
		}
		sections = append(sections, sec)
	}
	return sections
}

func tagFor(k usage.SourceKind) string {
	switch k {
	case usage.SourcePlan:
		return "plan"
	case usage.SourceAPIKey:
		return "key"
	default:
		return string(k)
	}
}

func statusWord(s usage.SourceStatus) string {
	switch s {
	case usage.StatusUnconfigured:
		return "not configured"
	case usage.StatusUnauthorized:
		return "unauthorized"
	case usage.StatusUnavailable:
		return "unavailable"
	case usage.StatusUnsupported:
		return "not supported"
	default:
		return string(s)
	}
}

// layout holds the column geometry shared by every row in the snapshot, so
// bars align across providers, not just within one.
type layout struct {
	compact    bool
	barW       int
	showCounts bool
	showChips  bool
	tagW       int
	labelW     int
	pctW       int
	countsW    int
	markerW    int
	chipW      int
}

func computeLayout(sections []section, width int, now time.Time) layout {
	l := layout{barW: barWidthMax, showCounts: true, showChips: true}
	for _, sec := range sections {
		for _, n := range sec.notices {
			l.tagW = max(l.tagW, lipgloss.Width(n.tag))
		}
		for _, r := range sec.rows {
			l.tagW = max(l.tagW, lipgloss.Width(r.tag))
			l.labelW = max(l.labelW, lipgloss.Width(r.window.Label))
			l.pctW = max(l.pctW, lipgloss.Width(pctString(r.window)))
			l.countsW = max(l.countsW, lipgloss.Width(countsString(r.window)))
			l.markerW = max(l.markerW, lipgloss.Width(markerText(r.window)))
			l.chipW = max(l.chipW, lipgloss.Width(chipString(r.window, now)))
		}
	}
	if l.countsW == 0 {
		l.showCounts = false
	}
	if l.chipW == 0 {
		l.showChips = false
	}
	if width < compactBelow {
		l.compact = true
		return l
	}
	// Degrade in order: shrink the bar, drop the counts column, drop chips.
	for l.total() > width && l.barW > barWidthMin {
		l.barW--
	}
	if l.total() > width && l.showCounts {
		l.showCounts = false
	}
	if l.total() > width && l.showChips {
		l.showChips = false
	}
	// Reclaim slack for the bar once columns were dropped.
	for l.total() < width && l.barW < barWidthMax {
		l.barW++
	}
	if l.total() > width {
		// Labels alone do not fit: give up on the table form.
		l.compact = true
	}
	return l
}

func (l layout) total() int {
	w := lipgloss.Width(indent) + l.tagW + 2 + l.labelW + 2 + l.barW + 1 + l.pctW
	if l.showCounts {
		w += 2 + l.countsW
	}
	if l.markerW > 0 {
		w += 2 + l.markerW
	}
	if l.showChips && l.chipW > 0 {
		w += 2 + l.chipW
	}
	return w
}

func renderHeader(sec section, width int) string {
	title := styleProvider.Render(sec.title)
	if sec.subtitle != "" {
		full := title + styleMuted.Render("  ·  "+sec.subtitle)
		if lipgloss.Width(full) <= width {
			return full
		}
	}
	if lipgloss.Width(title) > width {
		return ansi.Truncate(title, width, "…")
	}
	return title
}

func renderWindowRow(r windowRow, now time.Time, l layout) string {
	w := r.window
	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(padRight(r.tag, l.tagW))
	b.WriteString(columnGap)
	b.WriteString(padRight(w.Label, l.labelW))
	b.WriteString(columnGap)
	if w.Unlimited {
		b.WriteString(styleMuted.Render(padRight("unlimited", l.barW+1+l.pctW)))
	} else {
		b.WriteString(renderBar(w, l.barW))
		b.WriteString(" ")
		b.WriteString(padLeft(pctString(w), l.pctW))
	}
	if l.showCounts {
		b.WriteString(columnGap)
		b.WriteString(padLeft(countsString(w), l.countsW))
	}
	if l.markerW > 0 {
		b.WriteString(columnGap)
		b.WriteString(padRight(renderMarker(w), l.markerW))
	}
	if l.showChips {
		if chip := chipString(w, now); chip != "" {
			b.WriteString(columnGap)
			b.WriteString(styleMuted.Render(chip))
		}
	}
	return b.String()
}

// renderCompactRow renders one window as two lines: tag and label (with
// counts when they fit), then bar, percentage, marker, and chip.
func renderCompactRow(r windowRow, now time.Time, width int, l layout) []string {
	w := r.window
	line1 := indent + padRight(r.tag, l.tagW) + columnGap + w.Label
	if counts := countsString(w); counts != "" {
		if longer := line1 + columnGap + counts; lipgloss.Width(longer) <= width {
			line1 = longer
		}
	}

	marker := markerText(w)
	chip := chipString(w, now)
	build := func(withChip bool) string {
		if w.Unlimited {
			s := indent + styleMuted.Render("unlimited")
			if withChip && chip != "" {
				s += columnGap + styleMuted.Render(chip)
			}
			return s
		}
		barW := width - lipgloss.Width(indent) - 1 - l.pctW
		if marker != "" {
			barW -= 2 + lipgloss.Width(marker)
		}
		if withChip && chip != "" {
			barW -= 2 + lipgloss.Width(chip)
		}
		barW = min(barW, barWidthMax)
		if barW < 1 {
			barW = 1
		}
		s := indent + renderBar(w, barW) + " " + padLeft(pctString(w), l.pctW)
		if marker != "" {
			s += columnGap + renderMarker(w)
		}
		if withChip && chip != "" {
			s += columnGap + styleMuted.Render(chip)
		}
		return s
	}
	line2 := build(true)
	if lipgloss.Width(line2) > width {
		line2 = build(false)
	}
	return []string{line1, line2}
}

// renderNotice renders a non-OK source as calm information: the detail text,
// word-wrapped to the width with aligned continuation lines.
func renderNotice(n notice, width, tagW int) []string {
	prefix := indent + padRight(n.tag, tagW) + columnGap
	cont := strings.Repeat(" ", lipgloss.Width(prefix))
	textWidth := width - lipgloss.Width(prefix)
	if textWidth < 1 {
		textWidth = 1
	}
	parts := wrapWords(n.detail, textWidth)
	lines := make([]string, len(parts))
	for i, part := range parts {
		if i == 0 {
			lines[i] = prefix + styleMuted.Render(part)
		} else {
			lines[i] = cont + styleMuted.Render(part)
		}
	}
	return lines
}

// renderFooter names the earliest upcoming reset when every window in the
// snapshot is exhausted.
func renderFooter(s usage.Snapshot, now time.Time) string {
	total, exhausted := 0, 0
	for _, p := range s.Providers {
		for _, src := range p.Sources {
			if src.Status != usage.StatusOK {
				continue
			}
			for _, w := range src.Windows {
				total++
				if w.Health() == usage.HealthExhausted {
					exhausted++
				}
			}
		}
	}
	if total == 0 || exhausted != total {
		return ""
	}
	msg := "All limits are spent."
	if t := s.EarliestReset(); t != nil {
		if d := t.Sub(now); d > 0 {
			msg = "All limits are spent — the first one comes back in " + humanize(d) + "."
		} else {
			msg = "All limits are spent — the first one comes back now."
		}
	}
	return styleWarning.Render(msg)
}

func renderBar(w usage.LimitWindow, width int) string {
	if width < 1 {
		width = 1
	}
	filled := int(math.Round(w.UsedPercent / 100 * float64(width)))
	filled = max(0, min(filled, width))
	style := styleHealthy
	switch w.Health() {
	case usage.HealthWarning:
		style = styleWarning
	case usage.HealthExhausted:
		style = styleExhausted
	}
	return style.Render(strings.Repeat("█", filled)) + styleMuted.Render(strings.Repeat("░", width-filled))
}

// markerText is the plain-text health marker. It must remain readable with
// all styling stripped, so it never relies on color.
func markerText(w usage.LimitWindow) string {
	switch w.Health() {
	case usage.HealthWarning:
		return "⚠ WARN"
	case usage.HealthExhausted:
		return "✗ FULL"
	default:
		return ""
	}
}

func renderMarker(w usage.LimitWindow) string {
	switch w.Health() {
	case usage.HealthWarning:
		return styleWarning.Render(markerText(w))
	case usage.HealthExhausted:
		return styleExhausted.Render(markerText(w))
	default:
		return ""
	}
}

func pctString(w usage.LimitWindow) string {
	return fmt.Sprintf("%.0f%%", w.UsedPercent)
}

func countsString(w usage.LimitWindow) string {
	switch {
	case w.Used != nil && w.Limit != nil:
		return formatCount(*w.Used) + "/" + formatCount(*w.Limit)
	case w.Used != nil:
		return formatCount(*w.Used)
	case w.Limit != nil:
		return "/" + formatCount(*w.Limit)
	default:
		return ""
	}
}

func chipString(w usage.LimitWindow, now time.Time) string {
	if w.ResetAt == nil {
		return ""
	}
	if d := w.ResetAt.Sub(now); d > 0 {
		return "resets in " + humanize(d)
	}
	return "resets now"
}

// humanize renders a duration compactly: "45s", "12m", "2h 14m", "3d 4h".
func humanize(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		if h == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, h)
	}
}

// formatCount renders an absolute count with thousands separators.
func formatCount(f float64) string {
	n := int64(math.Round(f))
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// wrapWords breaks s on word boundaries so no line exceeds width. A single
// word longer than width is hard-broken rather than left overflowing.
func wrapWords(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := words[0]
	for _, word := range words[1:] {
		if lipgloss.Width(cur)+1+lipgloss.Width(word) <= width {
			cur += " " + word
		} else {
			lines = append(lines, cur)
			cur = word
		}
	}
	lines = append(lines, cur)
	var out []string
	for _, ln := range lines {
		out = append(out, hardBreak(ln, width)...)
	}
	return out
}

func hardBreak(s string, width int) []string {
	var out []string
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > width {
			out = append(out, b.String())
			b.Reset()
			w = 0
		}
		b.WriteRune(r)
		w += rw
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// padRight and padLeft pad with spaces to a display width. Measurement goes
// through lipgloss.Width, so ANSI sequences and wide runes are handled.
func padRight(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func padLeft(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return strings.Repeat(" ", d) + s
	}
	return s
}
