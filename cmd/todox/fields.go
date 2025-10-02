package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/phyten/todox/internal/engine"
)

type Field struct {
	Key    string
	Header string
}

type FieldSelection struct {
	Fields      []Field
	ShowAge     bool
	ShowComment bool
	ShowMessage bool
	ShowURL     bool
	NeedComment bool
	NeedMessage bool
	NeedURL     bool
}

type fieldMeta struct {
	header    string
	isAge     bool
	isComment bool
	isMessage bool
	isURL     bool
}

var fieldRegistry = map[string]fieldMeta{
	"type":     {header: "TYPE"},
	"author":   {header: "AUTHOR"},
	"email":    {header: "EMAIL"},
	"date":     {header: "DATE"},
	"age":      {header: "AGE", isAge: true},
	"commit":   {header: "COMMIT"},
	"location": {header: "LOCATION"},
	"comment":  {header: "COMMENT", isComment: true},
	"message":  {header: "MESSAGE", isMessage: true},
	"url":      {header: "URL", isURL: true},
}

func ResolveFields(raw string, withComment, withMessage, withAge, withURL bool) (FieldSelection, error) {
	raw = strings.TrimSpace(raw)
	sel := FieldSelection{}
	if raw == "" {
		keys := []string{"type", "author", "email", "date"}
		if withAge {
			keys = append(keys, "age")
		}
		keys = append(keys, "commit", "location")
		if withURL {
			keys = append(keys, "url")
		}
		if withComment {
			keys = append(keys, "comment")
		}
		if withMessage {
			keys = append(keys, "message")
		}
		sel.Fields = make([]Field, 0, len(keys))
		for _, key := range keys {
			meta := fieldRegistry[key]
			sel.Fields = append(sel.Fields, Field{Key: key, Header: meta.header})
		}
		sel.ShowAge = withAge
		sel.ShowComment = withComment
		sel.ShowMessage = withMessage
		sel.ShowURL = withURL
		sel.NeedComment = withComment
		sel.NeedMessage = withMessage
		sel.NeedURL = withURL
		return sel, nil
	}

	parts := strings.Split(raw, ",")
	sel.Fields = make([]Field, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return FieldSelection{}, fmt.Errorf("invalid fields: empty entry")
		}
		key := strings.ToLower(name)
		meta, ok := fieldRegistry[key]
		if !ok {
			return FieldSelection{}, fmt.Errorf("unknown field: %s", name)
		}
		sel.Fields = append(sel.Fields, Field{Key: key, Header: meta.header})
		if meta.isAge {
			sel.ShowAge = true
		}
		if meta.isComment {
			sel.ShowComment = true
		}
		if meta.isMessage {
			sel.ShowMessage = true
		}
		if meta.isURL {
			sel.ShowURL = true
		}
	}
	sel.NeedComment = withComment || sel.ShowComment
	sel.NeedMessage = withMessage || sel.ShowMessage
	sel.NeedURL = withURL || sel.ShowURL
	return sel, nil
}

func formatFieldValue(it engine.Item, key string) string {
	switch key {
	case "type":
		return it.Kind
	case "author":
		return it.Author
	case "email":
		return it.Email
	case "date":
		return it.Date
	case "age":
		return strconv.Itoa(it.AgeDays)
	case "commit":
		return short(it.Commit)
	case "location":
		return fmt.Sprintf("%s:%d", it.File, it.Line)
	case "comment":
		return it.Comment
	case "message":
		return it.Message
	case "url":
		return it.URL
	default:
		return ""
	}
}
