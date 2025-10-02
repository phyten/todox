package termcolor

import (
	"strconv"
	"strings"
)

type Scheme int

const (
	SchemeUnknown Scheme = iota
	SchemeDark
	SchemeLight
)

func DetectScheme(env map[string]string) Scheme {
	if env == nil {
		return SchemeDark
	}
	raw := strings.TrimSpace(env["COLORFGBG"])
	if raw != "" {
		parts := strings.Split(raw, ";")
		bgRaw := strings.TrimSpace(parts[len(parts)-1])
		if bgRaw == "" && len(parts) >= 2 {
			bgRaw = strings.TrimSpace(parts[len(parts)-2])
		}
		if bg, err := strconv.Atoi(bgRaw); err == nil {
			if bg >= 7 {
				return SchemeLight
			}
			if bg >= 0 {
				return SchemeDark
			}
		}
	}
	termName := strings.ToLower(strings.TrimSpace(env["TERM"]))
	if strings.Contains(termName, "light") {
		return SchemeLight
	}
	return SchemeDark
}
