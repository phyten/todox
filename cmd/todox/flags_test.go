package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/phyten/todox/internal/termcolor"
)

func TestParseScanArgsShortAliases(t *testing.T) {
	cfg, err := parseScanArgs([]string{"-t", "todo", "-m", "first", "-a", "Alice", "-o", "tsv", "--with-snippet"}, "en")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}

	if cfg.opts.Type != "todo" {
		t.Fatalf("Type mismatch: got %q", cfg.opts.Type)
	}
	if cfg.opts.Mode != "first" {
		t.Fatalf("Mode mismatch: got %q", cfg.opts.Mode)
	}
	if cfg.opts.AuthorRegex != "Alice" {
		t.Fatalf("Author regex mismatch: got %q", cfg.opts.AuthorRegex)
	}
	if cfg.output != "tsv" {
		t.Fatalf("Output mismatch: got %q", cfg.output)
	}
	if !cfg.withComment {
		t.Fatalf("withComment should be true when --with-snippet is passed")
	}
}

func TestParseScanArgsHelpLanguageFallback(t *testing.T) {
	cfg, err := parseScanArgs([]string{"-h"}, "ja")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}
	if !cfg.showHelp {
		t.Fatal("showHelp should be true")
	}
	if cfg.helpLang != "ja" {
		t.Fatalf("expected helpLang ja, got %q", cfg.helpLang)
	}
}

func TestParseScanArgsHelpOverridesLanguage(t *testing.T) {
	cfg, err := parseScanArgs([]string{"--lang", "en", "--help=ja"}, "en")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}
	if !cfg.showHelp {
		t.Fatal("showHelp should be true")
	}
	if cfg.helpLang != "ja" {
		t.Fatalf("expected helpLang ja, got %q", cfg.helpLang)
	}
}

func TestParseScanArgsHelpJaFlag(t *testing.T) {
	cfg, err := parseScanArgs([]string{"--help-ja"}, "en")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}
	if !cfg.showHelp {
		t.Fatal("showHelp should be true")
	}
	if cfg.helpLang != "ja" {
		t.Fatalf("expected helpLang ja, got %q", cfg.helpLang)
	}
}

func TestParseScanArgsFullSetsDefaultTrunc(t *testing.T) {
	cfg, err := parseScanArgs([]string{"--full"}, "en")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}
	if cfg.opts.TruncAll != 120 {
		t.Fatalf("expected default truncation 120, got %d", cfg.opts.TruncAll)
	}
	if !cfg.withComment || !cfg.withMessage {
		t.Fatalf("--full should enable both comment and message columns")
	}
}

func TestParseScanArgsWithAgeAndSort(t *testing.T) {
	cfg, err := parseScanArgs([]string{"--with-age", "--sort=-age"}, "en")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}
	if !cfg.withAge {
		t.Fatal("--with-age should enable AGE column")
	}
	if cfg.sortKey != "-age" {
		t.Fatalf("sortKey mismatch: got %q", cfg.sortKey)
	}
}

func TestParseScanArgsPathFilters(t *testing.T) {
	args := []string{
		"--path", "src,pkg",
		"--path", "cmd",
		"--exclude", "vendor/**",
		"--exclude", "dist/**,node_modules/**",
		"--path-regex", "^src/",
		"--path-regex", `\.go$`,
		"--exclude-typical",
	}
	cfg, err := parseScanArgs(args, "en")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}
	wantPaths := []string{"src", "pkg", "cmd"}
	if got := cfg.opts.Paths; !equalSlices(got, wantPaths) {
		t.Fatalf("paths mismatch: got=%v want=%v", got, wantPaths)
	}
	wantExcludes := []string{"vendor/**", "dist/**", "node_modules/**"}
	if got := cfg.opts.Excludes; !equalSlices(got, wantExcludes) {
		t.Fatalf("excludes mismatch: got=%v want=%v", got, wantExcludes)
	}
	wantRegex := []string{"^src/", `\.go$`}
	if got := cfg.opts.PathRegex; !equalSlices(got, wantRegex) {
		t.Fatalf("path regex mismatch: got=%v want=%v", got, wantRegex)
	}
	if !cfg.opts.ExcludeTypical {
		t.Fatal("exclude_typical should be true")
	}
}

func TestParseScanArgsFieldsFlag(t *testing.T) {
	cfg, err := parseScanArgs([]string{"--fields", "type,author,date"}, "en")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}
	if cfg.fields != "type,author,date" {
		t.Fatalf("fields flag not captured: %q", cfg.fields)
	}
}

func TestParseScanArgsColorFlag(t *testing.T) {
	cfg, err := parseScanArgs([]string{"--color", "always"}, "en")
	if err != nil {
		t.Fatalf("parseScanArgs failed: %v", err)
	}
	if cfg.colorMode != termcolor.ModeAlways {
		t.Fatalf("color mode mismatch: got %v", cfg.colorMode)
	}
	if _, err := parseScanArgs([]string{"--color", "rainbow"}, "en"); err == nil {
		t.Fatal("invalid color value should error")
	} else {
		var uerr *usageError
		if !errors.As(err, &uerr) {
			t.Fatalf("invalid color should return usageError, got %T", err)
		}
	}
}
func TestParseScanArgsRejectsInvalidOutput(t *testing.T) {
	if _, err := parseScanArgs([]string{"--output", "csv"}, "en"); err == nil {
		t.Fatal("expected error for invalid output value")
	}
}

func TestCLIRejectsInvalidColor(t *testing.T) {
	out, code := runTodoxExpectError(t, "--color", "rainbow")
	if code == 0 {
		t.Fatalf("command succeeded unexpectedly: %s", out)
	}
	if code != 2 && !strings.Contains(out, "exit status 2") {
		t.Fatalf("exit code mismatch: got=%d want=2 (output=%s)", code, out)
	}
	if !strings.Contains(out, "unknown color mode") {
		t.Fatalf("error output missing detail: %s", out)
	}
	if !strings.Contains(out, "todox —") {
		t.Fatalf("help text not printed: %s", out)
	}
}

func TestHelpOutputEnglish(t *testing.T) {
	output := runTodox(t, "-h")
	if !strings.Contains(output, "todox — Find who wrote TODO/FIXME") {
		t.Fatalf("help output missing heading: %s", output)
	}
}

func TestHelpOutputJapanese(t *testing.T) {
	output := runTodox(t, "-h", "ja")
	if !strings.Contains(output, "todox — リポジトリ内の TODO / FIXME") {
		t.Fatalf("Japanese help output missing heading: %s", output)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func runTodox(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, out)
	}
	return string(out)
}

func runTodoxExpectError(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("command succeeded unexpectedly: %s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected error type: %T (%v)", err, err)
	}
	return string(out), exitErr.ExitCode()
}
