package gitremote

import (
	"context"
	"fmt"
	"testing"
)

func TestParseSSHRemote(t *testing.T) {
	info, err := Parse("git@github.com:owner/repo.git")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.Host != "github.com" || info.Owner != "owner" || info.Repo != "repo" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if scheme := info.NormalizedScheme(); scheme != "https" {
		t.Fatalf("unexpected scheme for ssh remote: %s", scheme)
	}
}

func TestParseHTTPSRemote(t *testing.T) {
	info, err := Parse("https://example.com/org/project.git")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.Host != "example.com" || info.Owner != "org" || info.Repo != "project" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.NormalizedScheme() != "https" {
		t.Fatalf("scheme should be https: %+v", info)
	}
	if got := info.APIBaseURL(); got != "https://example.com/api/v3" {
		t.Fatalf("API base mismatch: %s", got)
	}
}

func TestParseHTTPSRemoteWithPort(t *testing.T) {
	info, err := Parse("https://ghes.local:8443/org/project.git")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.Host != "ghes.local:8443" {
		t.Fatalf("host should include port: %+v", info)
	}
	if got := info.APIBaseURL(); got != "https://ghes.local:8443/api/v3" {
		t.Fatalf("API base mismatch: %s", got)
	}
}

func TestParseHTTPRemoteKeepsScheme(t *testing.T) {
	info, err := Parse("http://git.example.com:8080/org/project.git")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.NormalizedScheme() != "http" {
		t.Fatalf("expected http scheme, got %s", info.NormalizedScheme())
	}
	if got := info.APIBaseURL(); got != "http://git.example.com:8080/api/v3" {
		t.Fatalf("API base mismatch: %s", got)
	}
}

func TestParseSSHRemoteWithExplicitPort(t *testing.T) {
	info, err := Parse("ssh://git@ghes.local:2222/org/project.git")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.Host != "ghes.local:2222" {
		t.Fatalf("host should retain port: %+v", info)
	}
	if info.Owner != "org" || info.Repo != "project" {
		t.Fatalf("owner/repo mismatch: %+v", info)
	}
	if info.NormalizedScheme() != "https" {
		t.Fatalf("ssh remotes should default to https for links: %s", info.NormalizedScheme())
	}
}

func TestParseHTTPSRemoteWithCredentialsAndTrailingSlash(t *testing.T) {
	info, err := Parse("https://deploy@github.example.com/team/repo/")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.Host != "github.example.com" || info.Owner != "team" || info.Repo != "repo" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestParseNormalizesBackslashes(t *testing.T) {
	info, err := Parse("https://example.com/org\\repo.git")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.Owner != "org" || info.Repo != "repo" {
		t.Fatalf("windows-style path not normalized: %+v", info)
	}
}

func TestBlobPathEscapes(t *testing.T) {
	got := BlobPath("dir/sub dir/file name.go")
	want := "dir/sub%20dir/file%20name.go"
	if got != want {
		t.Fatalf("BlobPath mismatch: got=%s want=%s", got, want)
	}
}

func TestDetectRespectsRemoteEnv(t *testing.T) {
	t.Setenv("TODOX_LINK_REMOTE", "upstream")
	runner := detectRunner{}
	info, err := Detect(context.Background(), runner, ".")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if info.Host != "github.example.com:2222" {
		t.Fatalf("expected upstream host with port: %+v", info)
	}
	if info.Owner != "team" || info.Repo != "demo" {
		t.Fatalf("unexpected owner/repo: %+v", info)
	}
}

func TestDetectAppliesSchemeOverride(t *testing.T) {
	t.Setenv("TODOX_LINK_SCHEME", "http")
	runner := detectRunner{}
	info, err := Detect(context.Background(), runner, ".")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if info.NormalizedScheme() != "http" {
		t.Fatalf("scheme override not applied: %+v", info)
	}
}

func TestNormalizedSchemeIgnoresInvalidOverride(t *testing.T) {
	t.Setenv("TODOX_LINK_SCHEME", "ftp")
	info := Info{Scheme: "http"}
	if got := info.NormalizedScheme(); got != "http" {
		t.Fatalf("invalid override should be ignored, got %s", got)
	}
}

type detectRunner struct{}

func (detectRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	if name != "git" || len(args) < 3 || args[0] != "config" || args[1] != "--get" {
		return nil, nil, fmt.Errorf("unexpected command: %s %v", name, args)
	}
	switch args[2] {
	case "remote.origin.url":
		return []byte("https://github.com/example/default.git\n"), nil, nil
	case "remote.upstream.url":
		return []byte("ssh://git@github.example.com:2222/team/demo.git\n"), nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown key: %s", args[2])
	}
}
