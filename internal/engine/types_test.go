package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPullRequestRefJSONIncludesOptionalFields(t *testing.T) {
	ref := PullRequestRef{
		Number: 7,
		State:  "open",
		URL:    "https://example.com/pr/7",
		Title:  "Fix race condition",
		Body:   "This PR fixes a race condition in the worker pool.",
	}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("failed to marshal pull request ref: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "\"title\":\"Fix race condition\"") {
		t.Fatalf("JSON is missing title field: %s", text)
	}
	if !strings.Contains(text, "\"body\":\"This PR fixes a race condition in the worker pool.\"") {
		t.Fatalf("JSON is missing body field: %s", text)
	}
}

func TestPullRequestRefJSONOmitsEmptyOptionalFields(t *testing.T) {
	ref := PullRequestRef{Number: 9, State: "closed", URL: "https://example.com/pr/9"}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("failed to marshal pull request ref without optional fields: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "\"title\":") {
		t.Fatalf("title should be omitted when empty: %s", text)
	}
	if strings.Contains(text, "\"body\":") {
		t.Fatalf("body should be omitted when empty: %s", text)
	}
}
