package main

import (
	"os/exec"
	"strings"
	"testing"
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

func TestParseSortSpecRejectsUnknownKey(t *testing.T) {
	if _, err := ParseSortSpec("-unknown"); err == nil {
		t.Fatal("未知キーに対するエラーを期待しました")
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

func runTodox(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, out)
	}
	return string(out)
}
