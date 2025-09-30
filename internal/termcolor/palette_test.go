package termcolor

import "testing"

func TestHeaderStyle(t *testing.T) {
	s := HeaderStyle()
	if !s.Bold || !s.Underline {
		t.Fatalf("header style should enable bold+underline: %+v", s)
	}
}

func TestTypeStyle(t *testing.T) {
	todo := TypeStyle("todo")
	if todo.FGBasic == nil || *todo.FGBasic != 4 {
		t.Fatalf("TODO style should be blue, got %+v", todo)
	}
	none := TypeStyle("other")
	if none.FGBasic != nil {
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
