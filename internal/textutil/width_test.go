package textutil

import (
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestVisibleWidth(t *testing.T) {
	setEastAsianWidth(t, false)
	cases := []struct {
		name string
		s    string
		want int
	}{
		{name: "ASCII", s: "ABC", want: 3},
		{name: "Hiragana", s: "あいう", want: 6},
		{name: "CombiningMark", s: "e\u0301", want: 1},
		{name: "EmojiSequence", s: "👨🏽‍💻", want: 2},
		{name: "ANSIColored", s: "\x1b[31m赤\x1b[0m", want: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := VisibleWidth(tc.s); got != tc.want {
				t.Fatalf("VisibleWidth(%q) = %d, want %d", tc.s, got, tc.want)
			}
		})
	}
}

func TestTruncateByWidth(t *testing.T) {
	setEastAsianWidth(t, false)
	cases := []struct {
		name     string
		s        string
		width    int
		want     string
		ellipsis string
	}{
		{name: "Japanese", s: "こんにちは世界", width: 6, want: "こん…", ellipsis: "…"},
		{name: "EmojiSafe", s: "👩‍❤️‍💋‍👩テスト", width: 4, want: "👩‍❤️‍💋‍👩…", ellipsis: "…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TruncateByWidth(tc.s, tc.width, tc.ellipsis); got != tc.want {
				t.Fatalf("TruncateByWidth(%q, %d) = %q, want %q", tc.s, tc.width, got, tc.want)
			}
			if width := VisibleWidth(tc.want); width > tc.width {
				t.Fatalf("result width %d exceeds limit %d", width, tc.width)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "plain", want: "plain"},
		{in: "\x1b[31mRed\x1b[0m", want: "Red"},
		{in: "\x1b]8;;https://example.com\x07link\x1b]8;;\x07", want: "link"},
	}
	for _, tc := range cases {
		if got := StripANSI(tc.in); got != tc.want {
			t.Fatalf("StripANSI(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestPadHelpers(t *testing.T) {
	setEastAsianWidth(t, false)
	if got := VisibleWidth(PadRight("あ", 6)); got != 6 {
		t.Fatalf("PadRight did not reach target width: %d", got)
	}
	if got := VisibleWidth(PadLeft("テスト", 8)); got != 8 {
		t.Fatalf("PadLeft did not reach target width: %d", got)
	}
}

func setEastAsianWidth(t *testing.T, eastAsian bool) {
	t.Helper()
	runewidth.EastAsianWidth = eastAsian
	runewidth.DefaultCondition = runewidth.NewCondition()
}
