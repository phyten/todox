package opts

import (
	"math"
	"net/url"
	"testing"

	"github.com/example/todox/internal/engine"
)

func TestParseBoolVariants(t *testing.T) {
	trueVals := []string{"1", "true", "TRUE", "yes", "On"}
	falseVals := []string{"0", "false", "FALSE", "no", "OFF"}

	for _, tc := range trueVals {
		t.Run("true/"+tc, func(t *testing.T) {
			got, err := ParseBool(tc, "flag")
			if err != nil {
				t.Fatalf("ParseBool(%q) error: %v", tc, err)
			}
			if !got {
				t.Fatalf("ParseBool(%q) = false, want true", tc)
			}
		})
	}

	for _, tc := range falseVals {
		t.Run("false/"+tc, func(t *testing.T) {
			got, err := ParseBool(tc, "flag")
			if err != nil {
				t.Fatalf("ParseBool(%q) error: %v", tc, err)
			}
			if got {
				t.Fatalf("ParseBool(%q) = true, want false", tc)
			}
		})
	}

	if _, err := ParseBool("maybe", "flag"); err == nil {
		t.Fatal("ParseBool should reject unknown values")
	}
}

func TestParseIntInRange(t *testing.T) {
	got, err := ParseIntInRange("42", "jobs", 1, 64)
	if err != nil {
		t.Fatalf("ParseIntInRange error: %v", err)
	}
	if got != 42 {
		t.Fatalf("ParseIntInRange = %d, want 42", got)
	}

	if _, err := ParseIntInRange("-1", "truncate", 0, math.MinInt); err == nil {
		t.Fatal("ParseIntInRange should reject negative values when min=0")
	}

	if _, err := ParseIntInRange("65", "jobs", 1, 64); err == nil {
		t.Fatal("ParseIntInRange should reject values above max")
	}
}

func TestNormalizeAndValidate(t *testing.T) {
	o := engine.Options{Type: "TODO", Mode: "FIRST", Jobs: 8}
	if err := NormalizeAndValidate(&o); err != nil {
		t.Fatalf("NormalizeAndValidate error: %v", err)
	}
	if o.Type != "todo" {
		t.Fatalf("Type normalized incorrectly: %q", o.Type)
	}
	if o.Mode != "first" {
		t.Fatalf("Mode normalized incorrectly: %q", o.Mode)
	}

	bad := engine.Options{Type: "maybe", Mode: "last", Jobs: 4}
	if err := NormalizeAndValidate(&bad); err == nil {
		t.Fatal("NormalizeAndValidate should fail for invalid type")
	}

	jobs := engine.Options{Type: "todo", Mode: "last", Jobs: 1024}
	if err := NormalizeAndValidate(&jobs); err == nil {
		t.Fatal("NormalizeAndValidate should fail for invalid jobs")
	}
}

func TestApplyWebQueryToOptions(t *testing.T) {
	def := Defaults("/repo")
	q := url.Values{}
	q.Set("type", "TODO")
	q.Set("mode", "FIRST")
	q.Set("with_comment", "yes")
	q.Set("jobs", "4")

	got, err := ApplyWebQueryToOptions(def, q)
	if err != nil {
		t.Fatalf("ApplyWebQueryToOptions error: %v", err)
	}
	if got.Type != "todo" {
		t.Fatalf("Type mismatch: %q", got.Type)
	}
	if got.Mode != "first" {
		t.Fatalf("Mode mismatch: %q", got.Mode)
	}
	if !got.WithComment {
		t.Fatal("WithComment should be true")
	}
	if got.Jobs != 4 {
		t.Fatalf("Jobs mismatch: %d", got.Jobs)
	}
}

func TestSplitMulti(t *testing.T) {
	vals := []string{"a,b", " c ", "", ",d"}
	got := SplitMulti(vals)
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("SplitMulti length mismatch: got=%d want=%d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("SplitMulti mismatch at %d: got=%q want=%q", i, got[i], v)
		}
	}
}
