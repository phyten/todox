package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phyten/todox/internal/engine"
)

var sampleItems = []engine.Item{
	{
		Kind:    "TODO",
		Author:  "Alice",
		Email:   "alice@example.com",
		Date:    "2024-05-01",
		AgeDays: 12,
		Commit:  "abcdef1234567890",
		File:    "internal/app/main.go",
		Line:    42,
		Comment: "refactor parser, handle \"quotes\"\nand commas",
		Message: "Add feature\nSecond line",
	},
	{
		Kind:    "FIXME",
		Author:  "Bob",
		Email:   "bob@example.com",
		Date:    "2024-04-20",
		AgeDays: 30,
		Commit:  "1234567890abcdef",
		File:    "pkg/util/helpers.go",
		Line:    7,
		Comment: "escape pipes | for markdown",
		Message: "Review <check>",
	},
}

func TestWriteCSV(t *testing.T) {
	sel, err := ResolveFields("type,author,email,location,comment,message", false, false, false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, sampleItems, sel); err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}
	assertGolden(t, "want-csv.csv", buf.String())
	if !strings.Contains(buf.String(), "\r\n") {
		t.Fatal("CSV output should use CRLF line endings")
	}
}

func TestWriteNDJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteNDJSON(&buf, sampleItems); err != nil {
		t.Fatalf("WriteNDJSON failed: %v", err)
	}
	output := buf.String()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != len(sampleItems) {
		t.Fatalf("expected %d lines, got %d", len(sampleItems), len(lines))
	}
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("line %d is not valid JSON: %s", i, line)
		}
		var item engine.Item
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("failed to decode line %d: %v", i, err)
		}
	}
	if strings.Contains(output, "\\u003c") {
		t.Fatal("HTML characters should not be escaped in NDJSON output")
	}
	assertGolden(t, "want-ndjson.ndjson", output)
}

func TestWriteMarkdownTable(t *testing.T) {
	sel, err := ResolveFields("type,author,comment,message", false, false, false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteMarkdownTable(&buf, sampleItems, sel); err != nil {
		t.Fatalf("WriteMarkdownTable failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Add feature<br>Second line") {
		t.Fatal("expected newline conversion to <br> in markdown output")
	}
	if !strings.Contains(output, "escape pipes \\| for markdown") {
		t.Fatal("expected pipe characters to be escaped in markdown output")
	}
	assertGolden(t, "want-md.md", output)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", name, err)
	}
	if diff := diffStrings(string(want), got); diff != "" {
		t.Fatalf("output mismatch (-want +got):\n%s", diff)
	}
}

func diffStrings(want, got string) string {
	if want == got {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("want:\n")
	buf.WriteString(want)
	if !strings.HasSuffix(want, "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString("got:\n")
	buf.WriteString(got)
	return buf.String()
}
