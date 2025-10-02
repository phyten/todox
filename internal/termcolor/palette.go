package termcolor

import (
	"math"
	"strings"

	"github.com/phyten/todox/internal/colorutil"
)

func HeaderStyle() Style {
	return Style{Bold: true, Underline: true}
}

func TypeStyle(kind string, scheme Scheme, profile Profile) Style {
	normalized := strings.ToUpper(strings.TrimSpace(kind))
	switch normalized {
	case "TODO":
		return todoStyle(scheme, profile)
	case "FIXME":
		return fixmeStyle(scheme, profile)
	default:
		return Style{}
	}
}

func todoStyle(scheme Scheme, profile Profile) Style {
	style := Style{Bold: true}
	switch profile {
	case ProfileTrueColor:
		var rgb [3]uint8
		if scheme == SchemeLight {
			rgb = [3]uint8{146, 64, 14}
		} else {
			rgb = [3]uint8{251, 191, 36}
		}
		rgb = ensureTrueColorContrast(rgb, scheme)
		style.FGTrue = &rgb
	case ProfileANSI256:
		color := 178
		if scheme == SchemeLight {
			color = 130
		}
		style.FG256 = &color
	default:
		color := 3
		style.FGBasic = &color
	}
	return style
}

func fixmeStyle(scheme Scheme, profile Profile) Style {
	style := Style{Bold: true}
	switch profile {
	case ProfileTrueColor:
		var rgb [3]uint8
		if scheme == SchemeLight {
			rgb = [3]uint8{185, 28, 28}
		} else {
			rgb = [3]uint8{239, 68, 68}
		}
		rgb = ensureTrueColorContrast(rgb, scheme)
		style.FGTrue = &rgb
	case ProfileANSI256:
		color := 203
		if scheme == SchemeLight {
			color = 124
		}
		style.FG256 = &color
	default:
		color := 1
		style.FGBasic = &color
	}
	return style
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

func ensureTrueColorContrast(rgb [3]uint8, scheme Scheme) [3]uint8 {
	fg := colorutil.RGB{R: rgb[0], G: rgb[1], B: rgb[2]}
	bg := schemeBackgroundRGB(scheme)
	ensured := colorutil.EnsureContrast(fg, bg, 4.5)
	return [3]uint8{ensured.R, ensured.G, ensured.B}
}

func schemeBackgroundRGB(scheme Scheme) colorutil.RGB {
	if scheme == SchemeLight {
		return colorutil.RGB{R: 249, G: 250, B: 251}
	}
	return colorutil.RGB{R: 17, G: 24, B: 39}
}
