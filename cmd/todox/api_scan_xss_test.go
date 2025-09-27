package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIScanはJSONでHTML特殊文字を保持する(t *testing.T) {
	repo := t.TempDir()

	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "テストユーザー")
	runGit(t, repo, "config", "user.email", "tester@example.com")

	src := "package main\n\nfunc sample() {\n    // TODO: <script>alert('xss')</script> & review <>\n}\n"
	if err := os.WriteFile(filepath.Join(repo, "sample.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("ファイル作成に失敗しました: %v", err)
	}

	runGit(t, repo, "add", ".")
	commitMsg := "feat: guard <b>bold</b> & <test>"
	runGit(t, repo, "commit", "-m", commitMsg)

	handler := apiScanHandler(repo)
	req := httptest.NewRequest("GET", "/api/scan?with_comment=1&with_message=1&type=todo&mode=last", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != 200 {
		t.Fatalf("ステータスコードが200ではありません: %d", rr.Code)
	}

	var payload struct {
		Items []struct {
			Comment string `json:"comment"`
			Message string `json:"message"`
		} `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("JSONデコードに失敗しました: %v", err)
	}

	if len(payload.Items) == 0 {
		t.Fatalf("TODOが検出されませんでした")
	}

	got := payload.Items[0]
	if got.Comment == "" || !strings.Contains(got.Comment, "<script>") || !strings.Contains(got.Comment, "&") {
		t.Fatalf("コメントにHTML特殊文字が保持されていません: %q", got.Comment)
	}
	if got.Message == "" || !strings.Contains(got.Message, "<b>bold</b>") {
		t.Fatalf("メッセージにHTML特殊文字が保持されていません: %q", got.Message)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v に失敗しました: %v\n%s", args, err, out)
	}
}
