package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/phyten/todox/internal/engine"
)

// WriteMarkdownTable renders items as a GitHub Flavored Markdown table.
func WriteMarkdownTable(w io.Writer, items []engine.Item, sel FieldSelection) error {
	headers := Headers(sel.Fields)
	if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(headers, " | ")); err != nil {
		return err
	}
	sep := make([]string, len(headers))
	for i := range sep {
		sep[i] = "---"
	}
	if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(sep, " | ")); err != nil {
		return err
	}
	for _, it := range items {
		row := RowValues(it, sel.Fields)
		for i := range row {
			row[i] = escapeMarkdownCell(row[i])
		}
		if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(row, " | ")); err != nil {
			return err
		}
	}
	return nil
}

func escapeMarkdownCell(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "<br>")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
