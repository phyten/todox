package link

import (
	"strings"
	"testing"

	"github.com/phyten/todox/internal/gitremote"
)

func TestBlobMarkdownAddsPlain(t *testing.T) {
	info := gitremote.Info{Host: "github.com", Owner: "owner", Repo: "repo"}
	got := Blob(info, "abcdef", "docs/readme.md", 10)
	if !strings.Contains(got, "?plain=1#L10") {
		t.Fatalf("expected markdown link to include plain parameter: %s", got)
	}
}

func TestCommitURL(t *testing.T) {
	info := gitremote.Info{Host: "example.com", Owner: "org", Repo: "proj", Scheme: "http"}
	got := Commit(info, "abcdef1234567890")
	want := "http://example.com/org/proj/commit/abcdef1234567890"
	if got != want {
		t.Fatalf("commit URL mismatch: got=%s want=%s", got, want)
	}
}

func TestBlobUsesCustomSchemeAndPort(t *testing.T) {
	info := gitremote.Info{Host: "ghes.local:8443", Owner: "team", Repo: "demo", Scheme: "https"}
	got := Blob(info, "abcdef", "src/main.go", 42)
	if !strings.HasPrefix(got, "https://ghes.local:8443/team/demo/blob/abcdef/src/main.go#L42") {
		t.Fatalf("unexpected blob URL: %s", got)
	}
}

func TestBlobAndCommitReturnEmptyForInvalidInput(t *testing.T) {
	info := gitremote.Info{Host: "github.com", Owner: "org", Repo: "repo"}
	if got := Blob(info, "", "file.go", 10); got != "" {
		t.Fatalf("empty sha should yield empty link: %s", got)
	}
	if got := Blob(info, "abcdef", "", 10); got != "" {
		t.Fatalf("empty file should yield empty link: %s", got)
	}
	if got := Blob(info, "abcdef", "file.go", 0); got != "" {
		t.Fatalf("non-positive line should yield empty link: %s", got)
	}
	if got := Commit(info, ""); got != "" {
		t.Fatalf("empty commit should yield empty link: %s", got)
	}
}
