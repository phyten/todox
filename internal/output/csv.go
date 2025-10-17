package output

import (
	"encoding/csv"
	"io"

	"github.com/phyten/todox/internal/engine"
)

// WriteCSV renders items as RFC 4180 compliant CSV (including CRLF endings).
func WriteCSV(w io.Writer, items []engine.Item, sel FieldSelection) error {
	writer := csv.NewWriter(w)
	writer.UseCRLF = true
	if err := writer.Write(Headers(sel.Fields)); err != nil {
		return err
	}
	for _, it := range items {
		if err := writer.Write(RowValues(it, sel.Fields)); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
