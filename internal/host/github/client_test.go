package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"testing"

	"github.com/phyten/todox/internal/gitremote"
)

type fakeRunner struct {
	calls  [][]string
	stdout []byte
	stderr []byte
	err    error
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, []byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.stdout, f.stderr, f.err
}

type notFoundRunner struct{}

func (notFoundRunner) Run(_ context.Context, _ string, _ string, _ ...string) ([]byte, []byte, error) {
	return nil, nil, &exec.Error{Name: "gh", Err: exec.ErrNotFound}
}

func TestFindPullRequestsByHeadIncludesBodyFromCLI(t *testing.T) {
	runner := &fakeRunner{
		stdout: []byte(`[{"number":5,"title":"Add feature","state":"OPEN","url":"https://example.com/pr/5","mergedAt":null,"body":"Detailed body"}]`),
	}
	client := &Client{
		info:       gitremote.Info{Owner: "acme", Repo: "proj"},
		runner:     runner,
		httpClient: &http.Client{},
	}
	prs, err := client.FindPullRequestsByHead(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Body != "Detailed body" {
		t.Fatalf("body not propagated: %+v", prs[0])
	}
	if !strings.EqualFold(prs[0].State, "open") {
		t.Fatalf("state should be normalized (case-insensitive): %+v", prs[0])
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected single gh invocation, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call[0] != "gh" {
		t.Fatalf("expected gh command, got %s", call[0])
	}
	joined := strings.Join(call, " ")
	if !strings.Contains(joined, "--json") || !strings.Contains(joined, "body") {
		t.Fatalf("--json argument should request body field: %s", joined)
	}
}

func TestFindPullRequestsByHeadHydratesBodyWhenMissingFromCLI(t *testing.T) {
	var detailCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/repos/acme/proj/pulls/5" {
			detailCalls++
			payload := map[string]any{"body": "Hydrated body"}
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Fatalf("failed to encode detail response: %v", err)
			}
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}

	runner := &fakeRunner{
		stdout: []byte(`[{"number":5,"title":"Add feature","state":"OPEN","url":"https://example.com/pr/5","mergedAt":null,"body":""}]`),
	}
	client := &Client{
		info:       gitremote.Info{Host: parsed.Host, Owner: "acme", Repo: "proj", Scheme: parsed.Scheme},
		runner:     runner,
		httpClient: server.Client(),
	}
	prs, err := client.FindPullRequestsByHead(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Body != "Hydrated body" {
		t.Fatalf("body was not hydrated: %+v", prs[0])
	}
	if detailCalls != 1 {
		t.Fatalf("expected exactly one detail fetch, got %d", detailCalls)
	}
}

func TestFindPullRequestsByCommitFetchesMissingBody(t *testing.T) {
	var commitCalls, detailCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/acme/proj/commits/abc123/pulls":
			commitCalls++
			payload := []map[string]any{{
				"number":   42,
				"title":    "Fix bug",
				"state":    "closed",
				"html_url": "https://example.com/pr/42",
				"body":     "",
			}}
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Fatalf("failed to encode commit response: %v", err)
			}
		case "/api/v3/repos/acme/proj/pulls/42":
			detailCalls++
			payload := map[string]any{"body": "Fetched body"}
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Fatalf("failed to encode detail response: %v", err)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	client := &Client{
		info:       gitremote.Info{Host: parsed.Host, Owner: "acme", Repo: "proj", Scheme: parsed.Scheme},
		runner:     notFoundRunner{},
		httpClient: server.Client(),
	}
	prs, err := client.FindPullRequestsByCommit(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commitCalls != 1 {
		t.Fatalf("commit endpoint should be called exactly once, got %d", commitCalls)
	}
	if detailCalls != 1 {
		t.Fatalf("detail endpoint should be called exactly once, got %d", detailCalls)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Body != "Fetched body" {
		t.Fatalf("body was not hydrated: %+v", prs[0])
	}
}
