package opts

import (
	"math"
	"net/url"
	"runtime"
	"testing"

	"github.com/example/todox/internal/engine"
)

func TestParseBoolAcceptsCommonSynonyms(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw     string
		want    bool
		wantErr bool
	}{
		"1":       {raw: "1", want: true},
		"true":    {raw: "true", want: true},
		"TRUE":    {raw: "TRUE", want: true},
		"yes":     {raw: "yes", want: true},
		"on":      {raw: "on", want: true},
		"0":       {raw: "0", want: false},
		"false":   {raw: "false", want: false},
		"FALSE":   {raw: "FALSE", want: false},
		"no":      {raw: "no", want: false},
		"off":     {raw: "off", want: false},
		"invalid": {raw: "maybe", wantErr: true},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseBool(tc.raw, "flag")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("result mismatch: got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestParseIntInRangeValidatesBounds(t *testing.T) {
	t.Parallel()

	if _, err := ParseIntInRange("5", "jobs", 1, 64); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if _, err := ParseIntInRange("0", "jobs", 1, 64); err == nil {
		t.Fatal("expected error for lower bound violation")
	}
	if _, err := ParseIntInRange("65", "jobs", 1, 64); err == nil {
		t.Fatal("expected error for upper bound violation")
	}
	if _, err := ParseIntInRange("abc", "jobs", 1, 64); err == nil {
		t.Fatal("expected error for invalid integer")
	}
	if _, err := ParseIntInRange("-1", "truncate", 0, math.MaxInt); err == nil {
		t.Fatal("expected error for negative value")
	}
}

func TestNormalizeAndValidateEnforcesEnumerations(t *testing.T) {
	t.Parallel()

	opt := engine.Options{
		Type:         "TODO",
		Mode:         "FIRST",
		WithComment:  true,
		WithMessage:  false,
		TruncAll:     0,
		TruncComment: 0,
		TruncMessage: 0,
		IgnoreWS:     true,
		Jobs:         8,
		RepoDir:      ".",
	}

	if err := NormalizeAndValidate(&opt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opt.Type != "todo" {
		t.Fatalf("type should be lower-cased: %q", opt.Type)
	}
	if opt.Mode != "first" {
		t.Fatalf("mode should be lower-cased: %q", opt.Mode)
	}

	opt.Type = "unknown"
	if err := NormalizeAndValidate(&opt); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestApplyWebQueryToOptionsParsesValues(t *testing.T) {
	t.Parallel()

	base := Defaults("/repo")
	q := url.Values{}
	q.Set("type", "todo")
	q.Set("mode", "first")
	q.Set("author", "Alice")
	q.Set("with_comment", "yes")
	q.Set("with_message", "1")
	q.Set("truncate", "80")
	q.Set("truncate_comment", "40")
	q.Set("truncate_message", "0")
	q.Set("ignore_ws", "0")
	q.Set("progress", "1")
	q.Set("jobs", "4")

	opt, err := ApplyWebQueryToOptions(base, q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opt.Type != "todo" || opt.Mode != "first" {
		t.Fatalf("type/mode not applied: %+v", opt)
	}
	if !opt.WithComment || !opt.WithMessage {
		t.Fatalf("with flags not parsed: %+v", opt)
	}
	if opt.TruncAll != 80 || opt.TruncComment != 40 || opt.TruncMessage != 0 {
		t.Fatalf("truncate values mismatch: %+v", opt)
	}
	if opt.IgnoreWS {
		t.Fatalf("ignore_ws should be false: %+v", opt)
	}
	if !opt.Progress {
		t.Fatalf("progress should be true: %+v", opt)
	}
	if opt.Jobs != 4 {
		t.Fatalf("jobs not applied: %+v", opt)
	}
}

func TestDefaultsCapsJobs(t *testing.T) {
	t.Parallel()

	got := Defaults(".").Jobs
	if runtime.NumCPU() <= 64 {
		if got != runtime.NumCPU() {
			t.Fatalf("expected jobs=%d got=%d", runtime.NumCPU(), got)
		}
	} else {
		if got != 64 {
			t.Fatalf("expected jobs capped at 64, got %d", got)
		}
	}
}

func TestSplitMultiTrimsAndIgnoresEmpty(t *testing.T) {
	t.Parallel()

	inputs := []string{" foo, bar ", "baz", " , qux,, "}
	got := SplitMulti(inputs)
	want := []string{"foo", "bar", "baz", "qux"}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%d want=%d (%v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("value mismatch at %d: got=%q want=%q", i, got[i], v)
		}
	}
}
