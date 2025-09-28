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
		p := strings.TrimSpace(part)
		if p == "" {
			return SortSpec{}, fmt.Errorf("invalid sort key: empty segment")
		}
		desc := false
		switch p[0] {
		case '+':
			p = p[1:]
		case '-':
			desc = true
			p = p[1:]
		}
		name := strings.ToLower(p)
		switch name {
		case "age", "age_days":
			keys = append(keys, SortKey{Name: "age", Desc: desc})
		case "date":
			keys = append(keys, SortKey{Name: "age", Desc: !desc})
		case "author", "email", "type", "file", "line", "commit":
			keys = append(keys, SortKey{Name: name, Desc: desc})
		case "location":
			keys = append(keys, SortKey{Name: "file", Desc: desc})
			keys = append(keys, SortKey{Name: "line", Desc: desc})
		default:
			return SortSpec{}, fmt.Errorf("unknown sort key: %s", part)
		}
	}
	return SortSpec{Keys: keys}, nil
}

func ApplySort(items []engine.Item, spec SortSpec) {
	if len(spec.Keys) == 0 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		for _, key := range spec.Keys {
			switch key.Name {
			case "age":
				if items[i].AgeDays != items[j].AgeDays {
					if key.Desc {
						return items[i].AgeDays > items[j].AgeDays
					}
					return items[i].AgeDays < items[j].AgeDays
				}
			case "author":
				if items[i].Author != items[j].Author {
					if key.Desc {
						return items[i].Author > items[j].Author
					}
					return items[i].Author < items[j].Author
				}
			case "email":
				if items[i].Email != items[j].Email {
					if key.Desc {
						return items[i].Email > items[j].Email
					}
					return items[i].Email < items[j].Email
				}
			case "type":
				if items[i].Kind != items[j].Kind {
					if key.Desc {
						return items[i].Kind > items[j].Kind
					}
					return items[i].Kind < items[j].Kind
				}
			case "file":
				if items[i].File != items[j].File {
					if key.Desc {
						return items[i].File > items[j].File
					}
					return items[i].File < items[j].File
				}
			case "line":
				if items[i].Line != items[j].Line {
					if key.Desc {
						return items[i].Line > items[j].Line
					}
					return items[i].Line < items[j].Line
				}
			case "commit":
				if items[i].Commit != items[j].Commit {
					if key.Desc {
						return items[i].Commit > items[j].Commit
					}
					return items[i].Commit < items[j].Commit
				}
			}
		}
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		return items[i].Line < items[j].Line
	})
}
