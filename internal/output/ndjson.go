package output

import (
	"encoding/json"
	"io"

	"github.com/phyten/todox/internal/engine"
)

// WriteNDJSON streams items as newline-delimited JSON objects.
func WriteNDJSON(w io.Writer, items []engine.Item) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return nil
}
