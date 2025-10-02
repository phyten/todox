package colorutil

import "math"

type RGB struct {
	R uint8
	G uint8
	B uint8
}

var (
	black = RGB{0, 0, 0}
	white = RGB{255, 255, 255}
)

func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func luminance(rgb RGB) float64 {
	r := srgbToLinear(float64(rgb.R) / 255.0)
	g := srgbToLinear(float64(rgb.G) / 255.0)
	b := srgbToLinear(float64(rgb.B) / 255.0)
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func ContrastRatio(fg, bg RGB) float64 {
	l1 := luminance(fg)
	l2 := luminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func AutoTextColor(bg RGB) RGB {
	crBlack := ContrastRatio(black, bg)
	crWhite := ContrastRatio(white, bg)
	if crBlack >= 4.5 || crBlack >= crWhite {
		return black
	}
	return white
}

func EnsureContrast(fg, bg RGB, minRatio float64) RGB {
	if minRatio <= 0 {
		minRatio = 4.5
	}
	if ContrastRatio(fg, bg) >= minRatio {
		return fg
	}
	candidate := AutoTextColor(bg)
	if ContrastRatio(candidate, bg) >= minRatio {
		return candidate
	}
	return candidate
}
