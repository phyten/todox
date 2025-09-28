package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/example/todox/internal/engine"
)

type FieldName string

const (
	FieldType     FieldName = "type"
	FieldAuthor   FieldName = "author"
	FieldEmail    FieldName = "email"
	FieldDate     FieldName = "date"
	FieldAge      FieldName = "age"
	FieldCommit   FieldName = "commit"
	FieldLocation FieldName = "location"
	FieldComment  FieldName = "comment"
	FieldMessage  FieldName = "message"
)

type FieldLayout struct {
	Fields      []FieldName
	ShowComment bool
	ShowMessage bool
	ShowAge     bool
}

var allowedFields = map[string]FieldName{
	"type":     FieldType,
	"author":   FieldAuthor,
	"email":    FieldEmail,
	"date":     FieldDate,
	"age":      FieldAge,
	"commit":   FieldCommit,
	"location": FieldLocation,
	"comment":  FieldComment,
	"message":  FieldMessage,
}

func resolveFields(raw string, withComment, withMessage, withAge bool) (FieldLayout, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		fields := []FieldName{FieldType, FieldAuthor, FieldEmail, FieldDate}
		if withAge {
			fields = append(fields, FieldAge)
		}
		fields = append(fields, FieldCommit, FieldLocation)
		if withComment {
			fields = append(fields, FieldComment)
		}
		if withMessage {
			fields = append(fields, FieldMessage)
		}
		return FieldLayout{Fields: fields, ShowComment: withComment, ShowMessage: withMessage, ShowAge: withAge}, nil
	}

	parts := strings.Split(raw, ",")
	fields := make([]FieldName, 0, len(parts))
	var showComment, showMessage, showAge bool
	for _, part := range parts {
		token := strings.ToLower(strings.TrimSpace(part))
		if token == "" {
			return FieldLayout{}, fmt.Errorf("invalid field: empty segment")
		}
		field, ok := allowedFields[token]
		if !ok {
			return FieldLayout{}, fmt.Errorf("invalid field: %s", token)
		}
		fields = append(fields, field)
		switch field {
		case FieldComment:
			showComment = true
		case FieldMessage:
			showMessage = true
		case FieldAge:
			showAge = true
		}
	}
	return FieldLayout{Fields: fields, ShowComment: showComment, ShowMessage: showMessage, ShowAge: showAge}, nil
}

func fieldHeader(name FieldName) string {
	switch name {
	case FieldType:
		return "TYPE"
	case FieldAuthor:
		return "AUTHOR"
	case FieldEmail:
		return "EMAIL"
	case FieldDate:
		return "DATE"
	case FieldAge:
		return "AGE"
	case FieldCommit:
		return "COMMIT"
	case FieldLocation:
		return "LOCATION"
	case FieldComment:
		return "COMMENT"
	case FieldMessage:
		return "MESSAGE"
	default:
		return strings.ToUpper(string(name))
	}
}

func fieldValue(it engine.Item, field FieldName) string {
	switch field {
	case FieldType:
		return it.Kind
	case FieldAuthor:
		return it.Author
	case FieldEmail:
		return it.Email
	case FieldDate:
		return it.Date
	case FieldAge:
		return strconv.Itoa(it.AgeDays)
	case FieldCommit:
		return short(it.Commit)
	case FieldLocation:
		return fmt.Sprintf("%s:%d", it.File, it.Line)
	case FieldComment:
		return it.Comment
	case FieldMessage:
		return it.Message
	default:
		return ""
	}
}
