package termcolor

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

type ColorMode int

const (
	ModeAuto ColorMode = iota
	ModeAlways
	ModeNever
)

func (m ColorMode) String() string {
	switch m {
	case ModeAlways:
		return "always"
	case ModeNever:
		return "never"
	default:
		return "auto"
	}
}

func ParseMode(v string) (ColorMode, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "auto":
		return ModeAuto, nil
	case "always":
		return ModeAlways, nil
	case "never":
		return ModeNever, nil
	default:
		return ModeAuto, fmt.Errorf("unknown color mode: %s", v)
	}
}

type Profile int

const (
	ProfileBasic8 Profile = iota
	ProfileANSI256
	ProfileTrueColor
)

func EnvMap(values []string) map[string]string {
	env := make(map[string]string, len(values))
	for _, entry := range values {
		if entry == "" {
			continue
		}
		if idx := strings.Index(entry, "="); idx >= 0 {
			env[entry[:idx]] = entry[idx+1:]
		} else {
			env[entry] = ""
		}
	}
	return env
}

// DetectMode determines the effective color mode for auto-detection.
//
// Priority order (first match wins):
//  1. TERM=dumb suppresses colors entirely.
//  2. NO_COLOR disables colors.
//  3. CLICOLOR=0 disables colors.
//  4. CLICOLOR_FORCE / FORCE_COLOR with any non-zero value force-enable colors.
//  5. Otherwise colors are emitted only when stdout is a TTY.
func DetectMode(stdout *os.File, env map[string]string) ColorMode {
	if stdout == nil {
		return ModeNever
	}
	if env != nil {
		if v := strings.ToLower(strings.TrimSpace(env["TERM"])); v == "dumb" {
			return ModeNever
		}
		if v := strings.TrimSpace(env["NO_COLOR"]); v != "" {
			return ModeNever
		}
		if v := strings.TrimSpace(env["CLICOLOR"]); v == "0" {
			return ModeNever
		}
		if forceColor(strings.TrimSpace(env["CLICOLOR_FORCE"])) {
			return ModeAlways
		}
		if forceColor(strings.TrimSpace(env["FORCE_COLOR"])) {
			return ModeAlways
		}
	}
	if isTerminal(stdout) {
		return ModeAlways
	}
	return ModeNever
}

// Enabled reports whether colors should be emitted for the provided mode.
// ModeAlways and ModeNever return constant results, while ModeAuto delegates
// to the TTY check on stdout (stderr is not considered).
func Enabled(mode ColorMode, stdout *os.File) bool {
	switch mode {
	case ModeAlways:
		return true
	case ModeNever:
		return false
	default:
		return isTerminal(stdout)
	}
}

// DetectProfile inspects COLORTERM/TERM to determine the best-fit color profile.
// truecolor/24-bit environments get TrueColor, *256color terminals get ANSI256,
// TERM=dumb and any other terminal names fall back to the basic 8-color profile.
func DetectProfile(env map[string]string) Profile {
	if env != nil {
		if v := strings.ToLower(strings.TrimSpace(env["COLORTERM"])); v != "" {
			if strings.Contains(v, "truecolor") || strings.Contains(v, "24bit") || strings.Contains(v, "24-bit") {
				return ProfileTrueColor
			}
		}
		if v := strings.ToLower(strings.TrimSpace(env["TERM"])); strings.Contains(v, "256color") {
			return ProfileANSI256
		}
	}
	return ProfileBasic8
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func forceColor(v string) bool {
	if v == "" {
		return false
	}
	return v != "0"
}
