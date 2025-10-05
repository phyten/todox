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
	ShowPRs     bool
	NeedComment bool
	NeedMessage bool
	NeedURL     bool
	NeedPRs     bool
}

type fieldMeta struct {
	header    string
	isAge     bool
	isComment bool
	isMessage bool
	isURL     bool
	isPR      bool
}

var fieldRegistry = map[string]fieldMeta{
	"type":       {header: "TYPE"},
	"author":     {header: "AUTHOR"},
	"email":      {header: "EMAIL"},
	"date":       {header: "DATE"},
	"age":        {header: "AGE", isAge: true},
	"commit":     {header: "COMMIT"},
	"location":   {header: "LOCATION"},
	"comment":    {header: "COMMENT", isComment: true},
	"message":    {header: "MESSAGE", isMessage: true},
	"url":        {header: "URL", isURL: true},
	"commit_url": {header: "URL", isURL: true},
	"pr":         {header: "PR", isPR: true},
	"prs":        {header: "PRS", isPR: true},
	"pr_urls":    {header: "PR_URLS", isPR: true},
}

func ResolveFields(raw string, withComment, withMessage, withAge, withURL, withPRs bool) (FieldSelection, error) {
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
		if withPRs {
			keys = append(keys, "prs")
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
		sel.ShowPRs = withPRs
		sel.NeedComment = withComment
		sel.NeedMessage = withMessage
		sel.NeedURL = withURL
		sel.NeedPRs = withPRs
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
		if meta.isPR {
			sel.ShowPRs = true
		}
	}
	sel.NeedComment = withComment || sel.ShowComment
	sel.NeedMessage = withMessage || sel.ShowMessage
	sel.NeedURL = withURL || sel.ShowURL
	sel.NeedPRs = withPRs || sel.ShowPRs
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
	case "commit_url":
		return it.URL
	case "pr":
		if len(it.PRs) == 0 {
			return ""
		}
		return formatPRSummary(it.PRs[0])
	case "prs":
		if len(it.PRs) == 0 {
			return ""
		}
		return formatPRList(it.PRs)
	case "pr_urls":
		if len(it.PRs) == 0 {
			return ""
		}
		return formatPRURLs(it.PRs)
	default:
		return ""
	}
}

func formatPRSummary(pr engine.PullRequestRef) string {
	state := strings.ToLower(strings.TrimSpace(pr.State))
	if state == "" {
		state = "unknown"
	}
	if pr.Number <= 0 {
		return fmt.Sprintf("(%s)", state)
	}
	return fmt.Sprintf("#%d(%s)", pr.Number, state)
}

func formatPRList(prs []engine.PullRequestRef) string {
	parts := make([]string, 0, len(prs))
	for _, pr := range prs {
		parts = append(parts, formatPRSummary(pr))
	}
	return strings.Join(parts, "; ")
}

func formatPRURLs(prs []engine.PullRequestRef) string {
	parts := make([]string, 0, len(prs))
	for _, pr := range prs {
		if strings.TrimSpace(pr.URL) == "" {
			continue
		}
		parts = append(parts, pr.URL)
	}
	return strings.Join(parts, "; ")
}
