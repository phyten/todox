package termcolor

import (
	"os"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := []struct {
		input string
		want  ColorMode
		err   bool
	}{
		{"", ModeAuto, false},
		{"auto", ModeAuto, false},
		{"always", ModeAlways, false},
		{"never", ModeNever, false},
		{"ALWAYS", ModeAlways, false},
		{"invalid", ModeAuto, true},
	}
	for _, tc := range cases {
		got, err := ParseMode(tc.input)
		if tc.err {
			if err == nil {
				t.Fatalf("ParseMode(%q) expected error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseMode(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("ParseMode(%q)=%v want %v", tc.input, got, tc.want)
		}
	}
}

func TestDetectModeEnvironmentOverrides(t *testing.T) {
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() {
		_ = w.Close()
	}()

	env := map[string]string{"NO_COLOR": "1"}
	if got := DetectMode(w, env); got != ModeNever {
		t.Fatalf("NO_COLOR should force never, got %v", got)
	}

	env = map[string]string{"NO_COLOR": "1", "CLICOLOR": "0"}
	if got := DetectMode(w, env); got != ModeNever {
		t.Fatalf("NO_COLOR and CLICOLOR=0 should still yield never, got %v", got)
	}

	env = map[string]string{"CLICOLOR_FORCE": "1"}
	if got := DetectMode(w, env); got != ModeAlways {
		t.Fatalf("CLICOLOR_FORCE should force always, got %v", got)
	}

	env = map[string]string{"CLICOLOR_FORCE": "2"}
	if got := DetectMode(w, env); got != ModeAlways {
		t.Fatalf("CLICOLOR_FORCE=2 should force always, got %v", got)
	}

	env = map[string]string{"NO_COLOR": "1", "CLICOLOR_FORCE": "1"}
	if got := DetectMode(w, env); got != ModeNever {
		t.Fatalf("NO_COLOR must override force flags, got %v", got)
	}

	env = map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"}
	if got := DetectMode(w, env); got != ModeNever {
		t.Fatalf("NO_COLOR must override FORCE_COLOR, got %v", got)
	}

	env = map[string]string{"CLICOLOR": "0"}
	if got := DetectMode(w, env); got != ModeNever {
		t.Fatalf("CLICOLOR=0 should disable colors, got %v", got)
	}

	env = map[string]string{"FORCE_COLOR": "2"}
	if got := DetectMode(w, env); got != ModeAlways {
		t.Fatalf("FORCE_COLOR=2 should force always, got %v", got)
	}

	env = map[string]string{"TERM": "dumb"}
	if got := DetectMode(w, env); got != ModeNever {
		t.Fatalf("TERM=dumb should disable colors, got %v", got)
	}

	env = map[string]string{"TERM": "dumb", "FORCE_COLOR": "1"}
	if got := DetectMode(w, env); got != ModeNever {
		t.Fatalf("TERM=dumb must override FORCE_COLOR, got %v", got)
	}
}

func TestEnabled(t *testing.T) {
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() {
		_ = w.Close()
	}()

	if !Enabled(ModeAlways, nil) {
		t.Fatal("ModeAlways should be enabled even with nil stdout")
	}
	if Enabled(ModeNever, w) {
		t.Fatal("ModeNever should be disabled")
	}
	if Enabled(ModeAuto, w) {
		t.Fatal("ModeAuto with non-tty stdout should be disabled")
	}
}

func TestDetectProfile(t *testing.T) {
	env := map[string]string{"COLORTERM": "truecolor"}
	if got := DetectProfile(env); got != ProfileTrueColor {
		t.Fatalf("COLORTERM truecolor should yield TrueColor, got %v", got)
	}
	env = map[string]string{"TERM": "xterm-256color"}
	if got := DetectProfile(env); got != ProfileANSI256 {
		t.Fatalf("TERM 256color should yield ANSI256, got %v", got)
	}
	env = map[string]string{}
	if got := DetectProfile(env); got != ProfileBasic8 {
		t.Fatalf("default profile should be Basic8, got %v", got)
	}
}

func TestEnvMap(t *testing.T) {
	env := EnvMap([]string{"FOO=bar", "BAZ", "QUX=1=2"})
	if env["FOO"] != "bar" {
		t.Fatalf("expected FOO=bar, got %q", env["FOO"])
	}
	if env["BAZ"] != "" {
		t.Fatalf("expected BAZ empty, got %q", env["BAZ"])
	}
	if env["QUX"] != "1=2" {
		t.Fatalf("expected QUX=1=2, got %q", env["QUX"])
	}
}
