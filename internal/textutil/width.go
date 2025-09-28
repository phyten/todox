package textutil

import (
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// ANSI escape sequences (covers common CSI and OSC forms).
var ansiRe = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

func stripANSI(s string) string {
	if s == "" {
		return ""
	}
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}
	return ansiRe.ReplaceAllString(s, "")
}

// VisibleWidth returns terminal display width (wcwidth-based).
func VisibleWidth(s string) int {
	if s == "" {
		return 0
	}
	t := stripANSI(s)
	g := uniseg.NewGraphemes(t)
	width := 0
	for g.Next() {
		width += runewidth.StringWidth(g.Str())
	}
	return width
}

// TruncateByWidth truncates s to fit width w without breaking graphemes.
// If truncation happens and ellipsis is not empty, append it when it fits.
func TruncateByWidth(s string, w int, ellipsis string) string {
	if s == "" {
		return ""
	}
	if w <= 0 {
		return ""
	}
	if VisibleWidth(s) <= w {
		return s
	}
	t := stripANSI(s)
	g := uniseg.NewGraphemes(t)
	segs := make([]string, 0, len(t))
	widths := make([]int, 0, len(t))
	used := 0
	ellW := runewidth.StringWidth(ellipsis)
	for g.Next() {
		seg := g.Str()
		segW := runewidth.StringWidth(seg)
		if used+segW > w {
			if ellipsis == "" {
				return join(segs)
			}
			if ellW > w {
				return join(segs)
			}
			for len(segs) > 0 && used+ellW > w {
				used -= widths[len(widths)-1]
				segs = segs[:len(segs)-1]
				widths = widths[:len(widths)-1]
			}
			if used+ellW > w {
				return join(segs)
			}
			return join(segs) + ellipsis
		}
		segs = append(segs, seg)
		widths = append(widths, segW)
		used += segW
	}
	return join(segs)
}

func join(segs []string) string {
	if len(segs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, seg := range segs {
		b.WriteString(seg)
	}
	return b.String()
}

// PadRight pads s on the right with spaces so that the visible width equals w.
func PadRight(s string, w int) string {
	pad := w - VisibleWidth(s)
	if pad <= 0 {
		return s
	}
	return s + spaces(pad)
}

// PadLeft pads s on the left with spaces so that the visible width equals w.
func PadLeft(s string, w int) string {
	pad := w - VisibleWidth(s)
	if pad <= 0 {
		return s
	}
	return spaces(pad) + s
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
