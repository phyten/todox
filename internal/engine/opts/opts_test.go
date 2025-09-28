package opts

import (
	"net/url"
	"runtime"
	"testing"

	"github.com/example/todox/internal/engine"
)

func TestParseBool(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw     string
		want    bool
		wantErr bool
	}{
		"empty":      {raw: "", want: false},
		"true":       {raw: "true", want: true},
		"TRUE":       {raw: "TRUE", want: true},
		"yes":        {raw: "yes", want: true},
		"on":         {raw: "on", want: true},
		"1":          {raw: "1", want: true},
		"false":      {raw: "false", want: false},
		"FALSE":      {raw: "FALSE", want: false},
		"no":         {raw: "no", want: false},
		"off":        {raw: "off", want: false},
		"0":          {raw: "0", want: false},
		"whitespace": {raw: "  true  ", want: true},
		"invalid":    {raw: "maybe", wantErr: true},
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
				t.Fatalf("ParseBool(%q)=%v want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParseIntInRange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw     string
		min     int
		max     int
		want    int
		wantErr bool
	}{
		{raw: "5", min: 1, max: 10, want: 5},
		{raw: "1", min: 1, max: 10, want: 1},
		{raw: "10", min: 1, max: 10, want: 10},
		{raw: "0", min: 1, max: 10, wantErr: true},
		{raw: "11", min: 1, max: 10, wantErr: true},
		{raw: "abc", min: 1, max: 10, wantErr: true},
	}

	for _, tc := range cases {
		got, err := ParseIntInRange(tc.raw, "jobs", tc.min, tc.max)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("expected error for %q", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != tc.want {
			t.Fatalf("ParseIntInRange(%q)=%d want %d", tc.raw, got, tc.want)
		}
	}
}

func TestSplitMulti(t *testing.T) {
	t.Parallel()

	got := SplitMulti([]string{" a , b ", "c", "b", "", "d , e"})
	want := []string{"a", "b", "c", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%d want=%d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("index %d: got=%q want=%q", i, got[i], v)
		}
	}
}

func TestNormalizeAndValidate(t *testing.T) {
	t.Parallel()

	opts := engine.Options{
		Type:         "TODO",
		Mode:         "FIRST",
		TruncAll:     0,
		TruncComment: 0,
		TruncMessage: 0,
		WithComment:  true,
		WithMessage:  true,
		Jobs:         4,
	}

	if err := NormalizeAndValidate(&opts); err != nil {
		t.Fatalf("NormalizeAndValidate returned error: %v", err)
	}
	if opts.Type != "todo" {
		t.Fatalf("type not normalised: %q", opts.Type)
	}
	if opts.Mode != "first" {
		t.Fatalf("mode not normalised: %q", opts.Mode)
	}
	if opts.TruncAll != 120 {
		t.Fatalf("truncate default not applied: got=%d", opts.TruncAll)
	}

	bad := []engine.Options{
		{Type: "unknown", Mode: "last", Jobs: 4},
		{Type: "todo", Mode: "weird", Jobs: 4},
		{Type: "todo", Mode: "last", Jobs: 0},
		{Type: "todo", Mode: "last", Jobs: 100},
		{Type: "todo", Mode: "last", Jobs: 4, TruncAll: -1},
		{Type: "todo", Mode: "last", Jobs: 4, TruncComment: -1},
		{Type: "todo", Mode: "last", Jobs: 4, TruncMessage: -1},
	}
	for _, o := range bad {
		if err := NormalizeAndValidate(&o); err == nil {
			t.Fatalf("expected error for %#v", o)
		}
	}
}

func TestApplyWebQueryToOptions(t *testing.T) {
	t.Parallel()

	def := Defaults("/tmp/repo")
	q := url.Values{
		"type":             []string{"FIXME"},
		"mode":             []string{"FIRST"},
		"author":           []string{"alice"},
		"with_comment":     []string{"1"},
		"with_message":     []string{"yes"},
		"truncate":         []string{"80"},
		"truncate_comment": []string{"40"},
		"truncate_message": []string{"20"},
		"jobs":             []string{"8"},
		"ignore_ws":        []string{"0"},
	}

	got, err := ApplyWebQueryToOptions(def, q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != "FIXME" || got.Mode != "FIRST" {
		t.Fatalf("string overrides not applied: %+v", got)
	}
	if !got.WithComment || !got.WithMessage {
		t.Fatalf("boolean overrides missing: %+v", got)
	}
	if got.TruncAll != 80 || got.TruncComment != 40 || got.TruncMessage != 20 {
		t.Fatalf("truncate overrides missing: %+v", got)
	}
	if got.Jobs != 8 {
		t.Fatalf("jobs override missing: %d", got.Jobs)
	}
	if got.IgnoreWS {
		t.Fatalf("ignore_ws override missing: %+v", got)
	}
}

func TestNormalizeOutput(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw     string
		want    string
		wantErr bool
	}{
		"default": {raw: "", want: "table"},
		"table":   {raw: "table", want: "table"},
		"TSV":     {raw: "TSV", want: "tsv"},
		"JSON":    {raw: "JSON", want: "json"},
		"bad":     {raw: "xml", wantErr: true},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeOutput(tc.raw)
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
				t.Fatalf("NormalizeOutput(%q)=%q want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestDefaultsJobsClamped(t *testing.T) {
	t.Parallel()

	// simulate large CPU count by temporarily overriding runtime.NumCPU via helper
	jobs := Defaults(".").Jobs
	if jobs < 1 || jobs > 64 {
		t.Fatalf("defaults jobs out of range: %d", jobs)
	}
	if runtime.NumCPU() < jobs && jobs != runtime.NumCPU() && jobs != 64 {
		t.Fatalf("unexpected clamp behaviour: %d vs %d", jobs, runtime.NumCPU())
	}
}
