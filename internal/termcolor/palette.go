package termcolor

import (
	"math"
	"strings"
)

func HeaderStyle() Style {
	return Style{Bold: true, Underline: true}
}

func TypeStyle(kind string) Style {
	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case "TODO":
		color := 4
		return Style{FGBasic: &color}
	case "FIXME":
		color := 1
		return Style{FGBasic: &color}
	default:
		return Style{}
	}
}

func AgeStyle(age int, profile Profile, maxAge float64) Style {
	if age < 0 {
		age = 0
	}
	switch profile {
	case ProfileTrueColor:
		r, g, b := gradientRGB(age, maxAge)
		rgb := [3]uint8{r, g, b}
		return Style{FGTrue: &rgb}
	case ProfileANSI256:
		r, g, b := gradientRGB(age, maxAge)
		idx := rgbToANSI256(r, g, b)
		return Style{FG256: &idx}
	default:
		color := ageBucketColor(age)
		return Style{FGBasic: &color}
	}
}

func gradientRGB(age int, maxAge float64) (uint8, uint8, uint8) {
	if maxAge <= 0 {
		maxAge = 120
	}
	t := float64(age) / maxAge
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	if t <= 0 {
		return 0, 255, 0
	}
	if t >= 1 {
		return 255, 0, 0
	}
	if t < 0.5 {
		ratio := t / 0.5
		r := uint8(math.Round(255 * ratio))
		return r, 255, 0
	}
	ratio := (t - 0.5) / 0.5
	g := uint8(math.Round(255 * (1 - ratio)))
	return 255, g, 0
}

func ageBucketColor(age int) int {
	switch {
	case age <= 7:
		return 2
	case age <= 30:
		return 3
	case age <= 90:
		return 5
	default:
		return 1
	}
}

func rgbToANSI256(r, g, b uint8) int {
	if r == g && g == b {
		if r < 8 {
			return 16
		}
		if r > 248 {
			return 231
		}
		return 232 + (int(r)-8)*24/247
	}
	rr := int(r) * 5 / 255
	gg := int(g) * 5 / 255
	bb := int(b) * 5 / 255
	return 16 + 36*rr + 6*gg + bb
}
