package opts

import (
	"net/url"
	"testing"

	"github.com/phyten/todox/internal/engine"
)

func TestParseBoolAcceptsSynonyms(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		input string
		want  bool
	}{
		"1":      {input: "1", want: true},
		"true":   {input: "true", want: true},
		"TRUE":   {input: "TRUE", want: true},
		"yes":    {input: "yes", want: true},
		"on":     {input: "on", want: true},
		"0":      {input: "0", want: false},
		"false":  {input: "false", want: false},
		"FALSE":  {input: "FALSE", want: false},
		"no":     {input: "no", want: false},
		"off":    {input: "off", want: false},
		"spaced": {input: "  true  ", want: true},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseBool(tc.input, "flag")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("result mismatch: got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestParseBoolRejectsInvalid(t *testing.T) {
	t.Parallel()

	if _, err := ParseBool("maybe", "flag"); err == nil {
		t.Fatal("expected error for invalid literal")
	}
}

func TestParseIntInRange(t *testing.T) {
	t.Parallel()

	t.Run("accepts value within range", func(t *testing.T) {
		got, err := ParseIntInRange("42", "jobs", 1, 64)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 42 {
			t.Fatalf("want 42, got %d", got)
		}
	})

	t.Run("rejects below minimum", func(t *testing.T) {
		if _, err := ParseIntInRange("0", "jobs", 1, 64); err == nil {
			t.Fatal("expected error when below min")
		}
	})

	t.Run("rejects above maximum", func(t *testing.T) {
		if _, err := ParseIntInRange("128", "jobs", 1, 64); err == nil {
			t.Fatal("expected error when above max")
		}
	})
}

func TestNormalizeAndValidate(t *testing.T) {
	t.Parallel()

	opts := engine.Options{
		Type:         "TODO",
		Mode:         "FIRST",
		WithComment:  true,
		WithMessage:  true,
		TruncAll:     0,
		TruncComment: 0,
		TruncMessage: 0,
		IgnoreWS:     true,
		Jobs:         8,
		RepoDir:      "",
	}

	if err := NormalizeAndValidate(&opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Type != "todo" {
		t.Fatalf("type not normalized: %q", opts.Type)
	}
	if opts.Mode != "first" {
		t.Fatalf("mode not normalized: %q", opts.Mode)
	}
	if opts.TruncAll != 120 {
		t.Fatalf("expected truncate default of 120 when both comment/message, got %d", opts.TruncAll)
	}
	if opts.RepoDir != "." {
		t.Fatalf("repo dir should default to '.' when empty: %q", opts.RepoDir)
	}

	bad := engine.Options{Type: "unknown", Mode: "last", Jobs: 1}
	if err := NormalizeAndValidate(&bad); err == nil {
		t.Fatal("expected error for invalid type")
	}

	jobs := engine.Options{Type: "todo", Mode: "last", Jobs: 65}
	if err := NormalizeAndValidate(&jobs); err == nil {
		t.Fatal("expected error for jobs > 64")
	}

	trunc := engine.Options{Type: "todo", Mode: "last", Jobs: 8, TruncComment: -1}
	if err := NormalizeAndValidate(&trunc); err == nil {
		t.Fatal("expected error for negative truncate")
	}
}

func TestApplyWebQueryToOptions(t *testing.T) {
	t.Parallel()

	base := Defaults("/repo")
	q := url.Values{}
	q.Add("type", "TODO")
	q.Add("type", "FIXME")
	q.Add("mode", "LAST")
	q.Add("mode", "FIRST")
	q.Add("with_comment", "0")
	q.Add("with_comment", "1")
	q.Add("with_message", "false")
	q.Add("with_message", "true")
	q.Add("truncate", "40")
	q.Add("truncate", "80")
	q.Add("truncate_comment", "10")
	q.Add("truncate_comment", "20")
	q.Add("truncate_message", "5,15")
	q.Add("jobs", "2")
	q.Add("jobs", "4,6")
	q.Add("ignore_ws", "1")
	q.Add("ignore_ws", "0")
	q.Add("progress", "0")
	q.Add("progress", "1")
	q.Add("author", "Alice")
	q.Add("author", " Bob ")

	got, err := ApplyWebQueryToOptions(base, q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != "FIXME" {
		t.Fatalf("expected type override, got %q", got.Type)
	}
	if got.Mode != "FIRST" {
		t.Fatalf("expected last mode value to win, got %q", got.Mode)
	}
	if !got.WithComment || !got.WithMessage {
		t.Fatalf("expected with_comment and with_message to be true")
	}
	if got.TruncAll != 80 {
		t.Fatalf("expected truncation override to apply")
	}
	if got.TruncComment != 20 {
		t.Fatalf("expected truncate_comment override to use last value, got %d", got.TruncComment)
	}
	if got.TruncMessage != 15 {
		t.Fatalf("expected truncate_message override to pick last literal, got %d", got.TruncMessage)
	}
	if got.Jobs != 6 {
		t.Fatalf("expected jobs override to pick last literal, got %d", got.Jobs)
	}
	if got.IgnoreWS {
		t.Fatal("expected ignore_ws=false when input is 0")
	}
	if !got.Progress {
		t.Fatal("expected progress to be true when last literal is truthy")
	}
	if got.AuthorRegex != "Bob" {
		t.Fatalf("expected author to use last raw value, got %q", got.AuthorRegex)
	}

	q.Set("with_comment", "maybe")
	if _, err := ApplyWebQueryToOptions(base, q); err == nil {
		t.Fatal("expected error for invalid boolean")
	}

	q.Set("jobs", "0")
	if _, err := ApplyWebQueryToOptions(base, q); err == nil {
		t.Fatal("expected error for jobs below range")
	}
}

func TestSplitMulti(t *testing.T) {
	t.Parallel()

	vals := []string{"type, author ", "date", "", ",,line"}
	got := SplitMulti(vals)
	want := []string{"type", "author", "date", "line"}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value mismatch at %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}
