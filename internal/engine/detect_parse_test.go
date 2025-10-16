package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/phyten/todox/internal/model"
)

func TestExpandLineMatchesRespectsLineNumbers(t *testing.T) {
	matches := []match{
		{file: "foo.txt", line: 5, text: "    // todo lower"},
		{file: "foo.txt", line: 11, text: "    # FIXME upper"},
	}
	out := expandLineMatches(matches, []string{"TODO", "FIXME"})
	if len(out) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(out))
	}
	if out[0].Span.StartLine != 5 {
		t.Fatalf("first match line mismatch: got %d want %d", out[0].Span.StartLine, 5)
	}
	if out[1].Span.StartLine != 11 {
		t.Fatalf("second match line mismatch: got %d want %d", out[1].Span.StartLine, 11)
	}
}

func TestExpandLineMatchesFallbackUsesKindOf(t *testing.T) {
	matches := []match{
		{file: "foo.txt", line: 3, text: "no markers here"},
	}
	out := expandLineMatches(matches, []string{"TODO"})
	if len(out) != 1 {
		t.Fatalf("expected 1 match, got %d", len(out))
	}
	if out[0].Span.StartLine != 3 || out[0].Span.EndLine != 3 {
		t.Fatalf("unexpected span lines: %d-%d", out[0].Span.StartLine, out[0].Span.EndLine)
	}
	if out[0].Tag != "" {
		t.Fatalf("expected empty tag for fallback, got %q", out[0].Tag)
	}
	if out[0].Span.StartCol != 1 {
		t.Fatalf("expected start column 1, got %d", out[0].Span.StartCol)
	}
	if out[0].Span.EndCol < out[0].Span.StartCol {
		t.Fatalf("end column should not be less than start: %d < %d", out[0].Span.EndCol, out[0].Span.StartCol)
	}
}

func TestGitGrepCaseInsensitive(t *testing.T) {
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	runGit("init")
	lower := filepath.Join(repo, "notes.txt")
	if err := os.WriteFile(lower, []byte("this line has todo in lowercase\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit("add", "notes.txt")

	pattern := "(TODO|FIXME)"
	matches, err := gitGrepMatches(repo, pattern, nil, nil, false)
	if err != nil {
		t.Fatalf("gitGrepMatches error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].line != 1 {
		t.Fatalf("unexpected line number: got %d want 1", matches[0].line)
	}

	files, err := gitGrepFiles(repo, pattern, nil, nil, false)
	if err != nil {
		t.Fatalf("gitGrepFiles error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0] != "notes.txt" {
		t.Fatalf("unexpected file: got %s want notes.txt", files[0])
	}
}

func TestScanWithStyleHandlesEscapedQuotes(t *testing.T) {
	tags := normalizeTags([]string{"TODO"})
	data := []byte(`const s = "escaped \"TODO\" marker"` + "\n")
	matches := scanWithStyle("app.js", data, tags, "javascript", styleJS, true)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Kind != model.MatchKindString {
		t.Fatalf("expected string kind, got %s", matches[0].Kind)
	}
}

func TestScanWithStyleHandlesTemplateBlocks(t *testing.T) {
	tags := normalizeTags([]string{"TODO"})
	data := []byte("const tpl = `\nTODO inside template\n`;\n")
	matches := scanWithStyle("app.ts", data, tags, "typescript", styleJS, true)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	span := matches[0].Span
	if span.StartLine != 2 || span.EndLine != 2 {
		t.Fatalf("template literal span mismatch: %v", span)
	}
	if matches[0].Kind != model.MatchKindString {
		t.Fatalf("expected string kind, got %s", matches[0].Kind)
	}
}

func TestScanWithStyleGoRawStringMultiline(t *testing.T) {
	tags := normalizeTags([]string{"TODO"})
	data := []byte("value := `\nTODO inside raw string\n`\n")
	matches := scanWithStyle("main.go", data, tags, "go", styleGo, true)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Kind != model.MatchKindString {
		t.Fatalf("expected string kind, got %s", matches[0].Kind)
	}
	if matches[0].Span.StartLine != 2 || matches[0].Span.EndLine != 2 {
		t.Fatalf("unexpected span: %+v", matches[0].Span)
	}
}

func TestScanWithStyleRubyIndentedBlock(t *testing.T) {
	tags := normalizeTags([]string{"TODO"})
	data := []byte("  =begin\n  TODO: fix\n  =end\n")
	matches := scanWithStyle("note.rb", data, tags, "ruby", styleRuby, false)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Kind != model.MatchKindComment {
		t.Fatalf("expected comment kind, got %s", matches[0].Kind)
	}
	if matches[0].Span.StartLine != 2 {
		t.Fatalf("expected start line 2, got %d", matches[0].Span.StartLine)
	}
}

func TestScanWithStyleSQLStrings(t *testing.T) {
	tags := normalizeTags([]string{"TODO"})
	data := []byte("SELECT 'todo fix';\n")
	matches := scanWithStyle("query.sql", data, tags, "sql", styleSQL, true)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Kind != model.MatchKindString {
		t.Fatalf("expected string kind, got %s", matches[0].Kind)
	}
}

func TestParseFileDetectLangsFallback(t *testing.T) {
	dir := t.TempDir()
	rel := "app.js"
	content := []byte("const x = 1; // TODO check\n")
	if err := os.WriteFile(filepath.Join(dir, rel), content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	opts := Options{RepoDir: dir, DetectLangs: []string{"go"}, IncludeStrings: true}
	tags := normalizeTags([]string{"TODO"})
	if matches, _ := parseFile(rel, opts, tags, false); len(matches) != 0 {
		t.Fatalf("expected skip without fallback, got %d", len(matches))
	}
	if matches, _ := parseFile(rel, opts, tags, true); len(matches) == 0 {
		t.Fatalf("expected fallback matches, got 0")
	}
}

func TestParseFileMaxFileBytesFallback(t *testing.T) {
	dir := t.TempDir()
	rel := "big.txt"
	content := strings.Repeat("x", 1024) + " TODO large\n"
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	opts := Options{RepoDir: dir, MaxFileBytes: 16}
	tags := normalizeTags([]string{"TODO"})
	matches, _ := parseFile(rel, opts, tags, false)
	if len(matches) == 0 {
		t.Fatalf("expected fallback matches, got 0")
	}
	if matches[0].Kind != model.MatchKindUnknown {
		t.Fatalf("expected unknown kind from plain-text fallback, got %s", matches[0].Kind)
	}
}

func TestParseFileSkipsBinary(t *testing.T) {
	dir := t.TempDir()
	rel := "blob.bin"
	content := []byte{0x00, 0x01, 0x02, 0x03}
	if err := os.WriteFile(filepath.Join(dir, rel), content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	opts := Options{RepoDir: dir}
	tags := normalizeTags([]string{"TODO"})
	if matches, _ := parseFile(rel, opts, tags, true); len(matches) != 0 {
		t.Fatalf("expected no matches for binary file, got %d", len(matches))
	}
}

func TestScanWithStylePythonTripleQuoteSpan(t *testing.T) {
	tags := normalizeTags([]string{"TODO"})
	data := []byte("\"\"\"\nTODO item\n\"\"\"\n")
	matches := scanWithStyle("note.py", data, tags, "python", stylePython, true)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	span := matches[0].Span
	if span.StartLine != 2 || span.EndLine != 2 {
		t.Fatalf("unexpected span lines: %d-%d", span.StartLine, span.EndLine)
	}
	if span.StartCol != 1 {
		t.Fatalf("expected column 1, got %d", span.StartCol)
	}
}

func TestComputeLineOffsetsNoDuplicateSentinel(t *testing.T) {
	cases := []struct {
		name string
		data string
		want []int
	}{
		{name: "with trailing newline", data: "a\nbc\n", want: []int{0, 2, 5}},
		{name: "without trailing newline", data: "a\n", want: []int{0, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeLineOffsets([]byte(tc.data))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("computeLineOffsets mismatch: got=%v want=%v", got, tc.want)
			}
		})
	}
}
