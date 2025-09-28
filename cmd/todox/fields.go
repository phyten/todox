package main

import (
	"fmt"
	"strings"
)

type FieldSelection struct {
	Fields     []string
	Provided   bool
	HasComment bool
	HasMessage bool
	HasAge     bool
}

var fieldHeaders = map[string]string{
	"type":     "TYPE",
	"author":   "AUTHOR",
	"email":    "EMAIL",
	"date":     "DATE",
	"age":      "AGE",
	"commit":   "COMMIT",
	"location": "LOCATION",
	"comment":  "COMMENT",
	"message":  "MESSAGE",
}

func ParseFieldSpec(raw string) (FieldSelection, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return FieldSelection{}, nil
	}
	parts := strings.Split(raw, ",")
	fields := make([]string, 0, len(parts))
	seen := map[string]bool{}
	sel := FieldSelection{Provided: true}
	for _, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			return FieldSelection{}, fmt.Errorf("invalid field name: empty segment")
		}
		canonical, ok := canonicalFieldName(name)
		if !ok {
			return FieldSelection{}, fmt.Errorf("unknown field: %s", part)
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		switch canonical {
		case "comment":
			sel.HasComment = true
		case "message":
			sel.HasMessage = true
		case "age":
			sel.HasAge = true
		}
		fields = append(fields, canonical)
	}
	sel.Fields = fields
	return sel, nil
}

func canonicalFieldName(name string) (string, bool) {
	switch name {
	case "age_days":
		name = "age"
	}
	_, ok := fieldHeaders[name]
	return name, ok
}

func DetermineFields(sel FieldSelection, hasComment, hasMessage bool, withAge bool) ([]string, bool) {
	if sel.Provided {
		return sel.Fields, sel.HasAge
	}
	fields := []string{"type", "author", "email", "date"}
	if withAge {
		fields = append(fields, "age")
	}
	fields = append(fields, "commit", "location")
	if hasComment {
		fields = append(fields, "comment")
	}
	if hasMessage {
		fields = append(fields, "message")
	}
	return fields, withAge
}

func containsField(fields []string, name string) bool {
	for _, f := range fields {
		if f == name {
			return true
		}
	}
	return false
}

func headerForField(name string) string {
	if h, ok := fieldHeaders[name]; ok {
		return h
	}
	return strings.ToUpper(name)
}
