package termcolor

import "testing"

func TestApply(t *testing.T) {
	boldRed := Style{Bold: true}
	color := 1
	boldRed.FGBasic = &color
	got := Apply(boldRed, "Hello", true)
	want := "\x1b[1;31mHello\x1b[0m"
	if got != want {
		t.Fatalf("Apply produced %q, want %q", got, want)
	}

	if got := Apply(Style{}, "Hello", true); got != "Hello" {
		t.Fatalf("empty style should return original text, got %q", got)
	}
	if got := Apply(boldRed, "Hello", false); got != "Hello" {
		t.Fatalf("disabled Apply should return original text, got %q", got)
	}
}
