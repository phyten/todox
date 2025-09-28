package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/example/todox/internal/engine"
)

// SortKey は単一カラムのソート条件を表します。
type SortKey struct {
	Name string
	Desc bool
}

// SortSpec はソート条件の並びを表します。
type SortSpec struct {
	Keys []SortKey
}

var sortKeyAliases = map[string]string{
	"age":      "age",
	"age_days": "age",
	"date":     "date",
	"author":   "author",
	"email":    "email",
	"type":     "type",
	"file":     "file",
	"line":     "line",
	"commit":   "commit",
	"location": "location",
}

// ParseSortSpec は --sort / ?sort パラメータを解析し、内部ソート仕様を返します。
func ParseSortSpec(raw string) (SortSpec, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return SortSpec{}, nil
	}

	parts := strings.Split(trimmed, ",")
	spec := SortSpec{}
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			return SortSpec{}, fmt.Errorf("invalid sort spec: empty key")
		}
		desc := false
		switch token[0] {
		case '+':
			token = token[1:]
		case '-':
			desc = true
			token = token[1:]
		}
		if token == "" {
			return SortSpec{}, fmt.Errorf("invalid sort spec: empty key")
		}
		canonical, ok := sortKeyAliases[token]
		if !ok {
			return SortSpec{}, fmt.Errorf("unknown sort key: %s", token)
		}
		switch canonical {
		case "date":
			spec.Keys = append(spec.Keys, SortKey{Name: "age", Desc: !desc})
		case "location":
			spec.Keys = append(spec.Keys,
				SortKey{Name: "file", Desc: desc},
				SortKey{Name: "line", Desc: desc},
			)
		default:
			spec.Keys = append(spec.Keys, SortKey{Name: canonical, Desc: desc})
		}
	}
	return spec, nil
}

// ApplySort は与えられた items を spec に従って安定ソートします。
func ApplySort(items []engine.Item, spec SortSpec) {
	if len(items) == 0 || len(spec.Keys) == 0 {
		// エンジン側は file/line 順で返すため、キーが無ければそのまま利用する。
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

// DisplayField は table / TSV 向けの表示列定義。
type DisplayField struct {
	Name   string
	Header string
	Value  func(engine.Item) string
}

// FieldSelection は列選択結果と補助フラグを表す。
type FieldSelection struct {
	Fields     []DisplayField
	HasComment bool
	HasMessage bool
	HasAge     bool
}

var fieldDefinitions = map[string]DisplayField{
	"type": {
		Name:   "type",
		Header: "TYPE",
		Value: func(it engine.Item) string {
			return it.Kind
		},
	},
	"author": {
		Name:   "author",
		Header: "AUTHOR",
		Value: func(it engine.Item) string {
			return it.Author
		},
	},
	"email": {
		Name:   "email",
		Header: "EMAIL",
		Value: func(it engine.Item) string {
			return it.Email
		},
	},
	"date": {
		Name:   "date",
		Header: "DATE",
		Value: func(it engine.Item) string {
			return it.Date
		},
	},
	"age": {
		Name:   "age",
		Header: "AGE",
		Value: func(it engine.Item) string {
			return fmt.Sprintf("%d", it.AgeDays)
		},
	},
	"commit": {
		Name:   "commit",
		Header: "COMMIT",
		Value: func(it engine.Item) string {
			return short(it.Commit)
		},
	},
	"location": {
		Name:   "location",
		Header: "LOCATION",
		Value: func(it engine.Item) string {
			return fmt.Sprintf("%s:%d", it.File, it.Line)
		},
	},
	"comment": {
		Name:   "comment",
		Header: "COMMENT",
		Value: func(it engine.Item) string {
			return it.Comment
		},
	},
	"message": {
		Name:   "message",
		Header: "MESSAGE",
		Value: func(it engine.Item) string {
			return it.Message
		},
	},
}

var validFieldNames = map[string]struct{}{
	"type":     {},
	"author":   {},
	"email":    {},
	"date":     {},
	"age":      {},
	"commit":   {},
	"location": {},
	"comment":  {},
	"message":  {},
}

// ResolveFields は表示フィールド指定と互換フラグから最終的な列構成を決定します。
func ResolveFields(raw string, withComment, withMessage, withAge bool) (FieldSelection, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		fields := []DisplayField{
			fieldDefinitions["type"],
			fieldDefinitions["author"],
			fieldDefinitions["email"],
			fieldDefinitions["date"],
		}
		if withAge {
			fields = append(fields, fieldDefinitions["age"])
		}
		fields = append(fields,
			fieldDefinitions["commit"],
			fieldDefinitions["location"],
		)
		if withComment {
			fields = append(fields, fieldDefinitions["comment"])
		}
		if withMessage {
			fields = append(fields, fieldDefinitions["message"])
		}
		return FieldSelection{
			Fields:     fields,
			HasComment: withComment,
			HasMessage: withMessage,
			HasAge:     withAge,
		}, nil
	}

	parts := strings.Split(trimmed, ",")
	seen := map[string]bool{}
	fields := make([]DisplayField, 0, len(parts))
	sel := FieldSelection{}
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return FieldSelection{}, fmt.Errorf("invalid fields spec: empty value")
		}
		if _, ok := validFieldNames[name]; !ok {
			return FieldSelection{}, fmt.Errorf("unknown field: %s", name)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		def := fieldDefinitions[name]
		fields = append(fields, def)
		switch name {
		case "comment":
			sel.HasComment = true
		case "message":
			sel.HasMessage = true
		case "age":
			sel.HasAge = true
		}
	}
	sel.Fields = fields
	return sel, nil
}
