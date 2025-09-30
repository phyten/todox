package termcolor

import (
	"fmt"
	"strings"
)

type Style struct {
	Bold      bool
	Underline bool
	Dim       bool
	FGBasic   *int
	FG256     *int
	FGTrue    *[3]uint8
}

func Apply(s Style, text string, enabled bool) string {
	if !enabled || text == "" {
		return text
	}
	codes := sgrCodes(s)
	if len(codes) == 0 {
		return text
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + "\x1b[0m"
}

func sgrCodes(s Style) []string {
	codes := make([]string, 0, 6)
	if s.Bold {
		codes = append(codes, "1")
	}
	if s.Dim {
		codes = append(codes, "2")
	}
	if s.Underline {
		codes = append(codes, "4")
	}
	if s.FGTrue != nil {
		rgb := *s.FGTrue
		codes = append(codes, fmt.Sprintf("38;2;%d;%d;%d", rgb[0], rgb[1], rgb[2]))
	} else if s.FG256 != nil {
		codes = append(codes, fmt.Sprintf("38;5;%d", *s.FG256))
	} else if s.FGBasic != nil {
		codes = append(codes, fmt.Sprintf("3%d", *s.FGBasic))
	}
	return codes
}
