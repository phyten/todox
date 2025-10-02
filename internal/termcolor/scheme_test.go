package termcolor

import "testing"

func TestDetectSchemeFromColorfgbg(t *testing.T) {
	if got := DetectScheme(map[string]string{"COLORFGBG": "7;0"}); got != SchemeDark {
		t.Fatalf("expected dark for bg=0, got %v", got)
	}
	if got := DetectScheme(map[string]string{"COLORFGBG": "15;7"}); got != SchemeLight {
		t.Fatalf("expected light for bg=7, got %v", got)
	}
	if got := DetectScheme(map[string]string{"COLORFGBG": "15;15"}); got != SchemeLight {
		t.Fatalf("expected light for bg=15, got %v", got)
	}
}

func TestDetectSchemeFallsBackToTermName(t *testing.T) {
	if got := DetectScheme(map[string]string{"TERM": "xterm-light"}); got != SchemeLight {
		t.Fatalf("expected light for TERM containing light, got %v", got)
	}
	if got := DetectScheme(nil); got != SchemeDark {
		t.Fatalf("nil env should default to dark, got %v", got)
	}
}
