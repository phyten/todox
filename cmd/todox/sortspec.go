package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/example/todox/internal/engine"
)

type SortKey struct {
	Name string
	Desc bool
}

type SortSpec struct {
	Keys []SortKey
}

func ParseSortSpec(raw string) (SortSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SortSpec{}, nil
	}
	parts := strings.Split(raw, ",")
	keys := make([]SortKey, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			return SortSpec{}, fmt.Errorf("invalid sort key: empty segment")
		}
		desc := false
		switch token[0] {
		case '+':
			token = token[1:]
		case '-':
			desc = true
			token = token[1:]
		}
		token = strings.TrimSpace(token)
		if token == "" {
			return SortSpec{}, fmt.Errorf("invalid sort key: sign without name")
		}
		name := strings.ToLower(token)
		switch name {
		case "age_days":
			name = "age"
		case "date":
			desc = !desc
			name = "age"
		case "location":
			keys = append(keys, SortKey{Name: "file", Desc: desc}, SortKey{Name: "line", Desc: desc})
			continue
		case "age", "author", "email", "type", "file", "line", "commit":
			// accepted as is
		default:
			return SortSpec{}, fmt.Errorf("invalid sort key: %s", token)
		}
		keys = append(keys, SortKey{Name: name, Desc: desc})
	}
	return SortSpec{Keys: keys}, nil
}

func ApplySort(items []engine.Item, spec SortSpec) {
	keys := spec.Keys
	if len(keys) == 0 {
		keys = []SortKey{{Name: "file"}, {Name: "line"}}
	} else {
		keys = append(append([]SortKey{}, keys...), SortKey{Name: "file"}, SortKey{Name: "line"})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a := &items[i]
		b := &items[j]
		for _, key := range keys {
			switch key.Name {
			case "age":
				if a.AgeDays != b.AgeDays {
					if key.Desc {
						return a.AgeDays > b.AgeDays
					}
					return a.AgeDays < b.AgeDays
				}
			case "author":
				if a.Author != b.Author {
					if key.Desc {
						return a.Author > b.Author
					}
					return a.Author < b.Author
				}
			case "email":
				if a.Email != b.Email {
					if key.Desc {
						return a.Email > b.Email
					}
					return a.Email < b.Email
				}
			case "type":
				if a.Kind != b.Kind {
					if key.Desc {
						return a.Kind > b.Kind
					}
					return a.Kind < b.Kind
				}
			case "file":
				if a.File != b.File {
					if key.Desc {
						return a.File > b.File
					}
					return a.File < b.File
				}
			case "line":
				if a.Line != b.Line {
					if key.Desc {
						return a.Line > b.Line
					}
					return a.Line < b.Line
				}
			case "commit":
				if a.Commit != b.Commit {
					if key.Desc {
						return a.Commit > b.Commit
					}
					return a.Commit < b.Commit
				}
			}
		}
		return false
	})
}
