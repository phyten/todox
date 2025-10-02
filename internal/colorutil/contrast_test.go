package colorutil

import "testing"

func TestContrastRatio(t *testing.T) {
	cases := []struct {
		name     string
		fg, bg   RGB
		minRatio float64
	}{
		{"blackOnWhite", RGB{0, 0, 0}, RGB{255, 255, 255}, 4.5},
		{"whiteOnBlack", RGB{255, 255, 255}, RGB{0, 0, 0}, 4.5},
		{"darkRedOnWhite", RGB{185, 28, 28}, RGB{255, 255, 255}, 4.5},
		{"amberOnBlack", RGB{245, 158, 11}, RGB{17, 24, 39}, 4.5},
	}
	for _, tc := range cases {
		ratio := ContrastRatio(tc.fg, tc.bg)
		if ratio < tc.minRatio {
			t.Fatalf("%s contrast ratio %.2f < %.2f", tc.name, ratio, tc.minRatio)
		}
	}
}

func TestAutoTextColor(t *testing.T) {
	cases := []struct {
		name string
		bg   RGB
		want RGB
	}{
		{"lightBackground", RGB{255, 247, 237}, black},
		{"darkBackground", RGB{15, 23, 42}, white},
		{"medium", RGB{120, 113, 108}, white},
	}
	for _, tc := range cases {
		got := AutoTextColor(tc.bg)
		if got != tc.want {
			t.Fatalf("%s AutoTextColor=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestEnsureContrastPrefersAutoWhenNeeded(t *testing.T) {
	bg := RGB{255, 255, 255}
	fg := RGB{255, 0, 0}
	ensured := EnsureContrast(fg, bg, 4.5)
	if ContrastRatio(ensured, bg) < 4.5 {
		t.Fatalf("expected EnsureContrast to meet ratio, got %.2f", ContrastRatio(ensured, bg))
	}
}
