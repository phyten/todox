package termcolor

import (
	"testing"

	"github.com/phyten/todox/internal/colorutil"
)

func TestHeaderStyle(t *testing.T) {
	s := HeaderStyle()
	if !s.Bold || !s.Underline {
		t.Fatalf("header style should enable bold+underline: %+v", s)
	}
}

func TestTypeStyleRespectsScheme(t *testing.T) {
	todoDark := TypeStyle("todo", SchemeDark, ProfileBasic8)
	if todoDark.FGBasic == nil || *todoDark.FGBasic != 3 || !todoDark.Bold {
		t.Fatalf("TODO dark basic style mismatch: %+v", todoDark)
	}
	todoLight := TypeStyle("todo", SchemeLight, ProfileANSI256)
	if todoLight.FG256 == nil || *todoLight.FG256 != 130 {
		t.Fatalf("TODO light 256 color mismatch: %+v", todoLight)
	}
	todoTrue := TypeStyle("todo", SchemeLight, ProfileTrueColor)
	if todoTrue.FGTrue == nil {
		t.Fatalf("TODO light truecolor missing fg: %+v", todoTrue)
	}
	todoRGB := *todoTrue.FGTrue
	todoContrast := colorutil.ContrastRatio(
		colorutil.RGB{R: todoRGB[0], G: todoRGB[1], B: todoRGB[2]},
		colorutil.RGB{R: 249, G: 250, B: 251},
	)
	if todoContrast < 4.5 {
		t.Fatalf("TODO light truecolor contrast %.2f < 4.5 (rgb=%v)", todoContrast, todoRGB)
	}
	fixmeLight := TypeStyle("fixme", SchemeLight, ProfileTrueColor)
	if fixmeLight.FGTrue == nil {
		t.Fatalf("FIXME light truecolor missing fg: %+v", fixmeLight)
	}
	rgb := *fixmeLight.FGTrue
	contrast := colorutil.ContrastRatio(
		colorutil.RGB{R: rgb[0], G: rgb[1], B: rgb[2]},
		colorutil.RGB{R: 249, G: 250, B: 251},
	)
	if contrast < 4.5 {
		t.Fatalf("FIXME light truecolor contrast %.2f < 4.5 (rgb=%v)", contrast, rgb)
	}
	none := TypeStyle("other", SchemeDark, ProfileBasic8)
	if none.FGBasic != nil || none.FG256 != nil || none.FGTrue != nil {
		t.Fatalf("non TODO/FIXME should have no color: %+v", none)
	}
}

func TestAgeStyleBasicBuckets(t *testing.T) {
	tests := []struct {
		age  int
		want int
	}{
		{0, 2},
		{5, 2},
		{10, 3},
		{60, 5},
		{200, 1},
	}
	for _, tc := range tests {
		style := AgeStyle(tc.age, ProfileBasic8, 120)
		if style.FGBasic == nil {
			t.Fatalf("age %d missing basic color", tc.age)
		}
		if *style.FGBasic != tc.want {
			t.Fatalf("age %d expected color %d, got %d", tc.age, tc.want, *style.FGBasic)
		}
	}
}

func TestAgeStyleGradient(t *testing.T) {
	style := AgeStyle(0, ProfileANSI256, 120)
	if style.FG256 == nil || *style.FG256 != rgbToANSI256(0, 255, 0) {
		t.Fatalf("age 0 should map to green in 256 palette, got %+v", style)
	}
	style = AgeStyle(200, ProfileTrueColor, 120)
	if style.FGTrue == nil {
		t.Fatalf("true color style missing value")
	}
	rgb := *style.FGTrue
	if rgb[0] != 255 || rgb[1] != 0 || rgb[2] != 0 {
		t.Fatalf("age beyond max should be red, got %v", rgb)
	}
}
