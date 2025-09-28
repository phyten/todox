package engine

import (
	"path/filepath"
	"regexp"
	"testing"
)

func TestBuildGrepPathspecs_DefaultsToDot(t *testing.T) {
	t.Parallel()

	got := buildGrepPathspecs(nil, nil, false)
	want := []string{"."}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestBuildGrepPathspecsIncludesAndExcludes(t *testing.T) {
	t.Parallel()

	includes := []string{"src", " pkg ", "windows\\path"}
	excludes := []string{"vendor/**", ":(exclude)third_party/**", ":!build/**"}

	got := buildGrepPathspecs(includes, excludes, true)

	expectedHead := []string{"src", "pkg", filepath.ToSlash("windows\\path")}
	for i, want := range expectedHead {
		if i >= len(got) || got[i] != filepath.ToSlash(want) {
			t.Fatalf("include %d mismatch: got=%v want=%v", i, got, expectedHead)
		}
	}

	// typical excludes should follow includes
	typical := typicalExcludePatterns
	start := len(expectedHead)
	if len(got) < start+len(typical) {
		t.Fatalf("expected typical excludes to be appended: %v", got)
	}
	for i, want := range typical {
		if got[start+i] != want {
			t.Fatalf("typical exclude mismatch at %d: got=%q want=%q", start+i, got[start+i], want)
		}
	}

	tail := got[start+len(typical):]
	expectedTail := []string{":(glob,exclude)vendor/**", ":(exclude)third_party/**", ":!build/**"}
	if len(tail) != len(expectedTail) {
		t.Fatalf("exclude length mismatch: got=%v want=%v", tail, expectedTail)
	}
	for i, want := range expectedTail {
		if tail[i] != want {
			t.Fatalf("exclude %d mismatch: got=%q want=%q", i, tail[i], want)
		}
	}
}

func TestCompilePathRegexTrimsAndValidates(t *testing.T) {
	t.Parallel()

	rx, err := compilePathRegex([]string{"  ", "^src/", "(cmd|pkg)"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rx) != 2 {
		t.Fatalf("expected 2 regexps, got %d", len(rx))
	}

	if _, err := compilePathRegex([]string{"["}); err == nil {
		t.Fatal("expected compile error for invalid regexp")
	}
}

func TestFilterByPathRegex(t *testing.T) {
	t.Parallel()

	matches := []match{{file: "src/main.go"}, {file: "pkg/util.go"}, {file: "docs/readme.md"}}
	rx := []*regexp.Regexp{regexp.MustCompile(`^src/`), regexp.MustCompile(`\.go$`)}

	got := filterByPathRegex(matches, rx)
	want := []match{{file: "src/main.go"}, {file: "pkg/util.go"}}
	if len(got) != len(want) {
		t.Fatalf("expected %d matches, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i].file != want[i].file {
			t.Fatalf("match %d mismatch: got=%q want=%q", i, got[i].file, want[i].file)
		}
	}

	empty := filterByPathRegex(matches, nil)
	if len(empty) != len(matches) {
		t.Fatalf("expected original slice when no regex: %d vs %d", len(empty), len(matches))
	}
}
