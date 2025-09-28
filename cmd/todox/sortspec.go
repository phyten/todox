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
			return SortSpec{}, fmt.Errorf("invalid sort spec: empty key")
		}
		desc := false
		if strings.HasPrefix(token, "-") || strings.HasPrefix(token, "+") {
			desc = strings.HasPrefix(token, "-")
			token = strings.TrimSpace(token[1:])
			if token == "" {
				return SortSpec{}, fmt.Errorf("invalid sort spec: empty key")
			}
		}
		name := strings.ToLower(token)
		switch name {
		case "age", "age_days":
			name = "age"
		case "date":
			name = "age"
			desc = !desc
		case "author", "email", "type", "file", "line", "commit":
			// accepted as-is
		case "location":
			keys = append(keys, SortKey{Name: "file", Desc: desc}, SortKey{Name: "line", Desc: desc})
			continue
		default:
			return SortSpec{}, fmt.Errorf("unknown sort key: %s", token)
		}
		keys = append(keys, SortKey{Name: name, Desc: desc})
	}
	return SortSpec{Keys: keys}, nil
}

func ApplySort(items []engine.Item, spec SortSpec) {
	if len(spec.Keys) == 0 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		for _, key := range spec.Keys {
			switch key.Name {
			case "age":
				if left.AgeDays != right.AgeDays {
					if key.Desc {
						return left.AgeDays > right.AgeDays
					}
					return left.AgeDays < right.AgeDays
				}
			case "author":
				if left.Author != right.Author {
					if key.Desc {
						return left.Author > right.Author
					}
					return left.Author < right.Author
				}
			case "email":
				if left.Email != right.Email {
					if key.Desc {
						return left.Email > right.Email
					}
					return left.Email < right.Email
				}
			case "type":
				if left.Kind != right.Kind {
					if key.Desc {
						return left.Kind > right.Kind
					}
					return left.Kind < right.Kind
				}
			case "file":
				if left.File != right.File {
					if key.Desc {
						return left.File > right.File
					}
					return left.File < right.File
				}
			case "line":
				if left.Line != right.Line {
					if key.Desc {
						return left.Line > right.Line
					}
					return left.Line < right.Line
				}
			case "commit":
				if left.Commit != right.Commit {
					if key.Desc {
						return left.Commit > right.Commit
					}
					return left.Commit < right.Commit
				}
			}
		}
		if left.File != right.File {
			return left.File < right.File
		}
		return left.Line < right.Line
	})
}
