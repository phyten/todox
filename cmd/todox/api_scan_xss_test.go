package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/todox/internal/engine"
)

func TestAPIScanHandlerはXSS向けに生文字列を返す(t *testing.T) {
	repoDir := t.TempDir()

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Tester")
	runGit(t, repoDir, "config", "user.email", "tester@example.com")

	if err := os.MkdirAll(filepath.Join(repoDir, "dir"), 0o755); err != nil {
		t.Fatalf("ディレクトリの作成に失敗しました: %v", err)
	}

	source := "package main\n\n// TODO <img src=x onerror=alert(1)> & <>\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "dir", "<file>&.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("ファイルの作成に失敗しました: %v", err)
	}

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "<b>bold</b> & <>")

	req := httptest.NewRequest("GET", "/api/scan?with_comment=1&with_message=1", nil)
	rec := httptest.NewRecorder()

	handler := apiScanHandler(repoDir)
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("ステータスコードが200ではありません: %d", rec.Code)
	}

	var res engine.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("JSONのデコードに失敗しました: %v", err)
	}

	if len(res.Items) != 1 {
		t.Fatalf("TODO項目の件数が想定外です: %d", len(res.Items))
	}

	item := res.Items[0]
	if !strings.Contains(item.Comment, "<img src=x onerror=alert(1)>") {
		t.Fatalf("コメントに生のHTMLが含まれていません: %q", item.Comment)
	}
	if !strings.Contains(item.Comment, "& <>") {
		t.Fatalf("コメント中の&<>が失われています: %q", item.Comment)
	}
	if !strings.Contains(item.Message, "<b>bold</b> & <>") {
		t.Fatalf("コミットメッセージに生のHTMLが含まれていません: %q", item.Message)
	}
	if !strings.Contains(item.File, "<file>&.go") {
		t.Fatalf("ファイル名の特殊文字が保持されていません: %q", item.File)
	}
	if item.Line <= 0 {
		t.Fatalf("行番号が正しく取得できていません: %d", item.Line)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v に失敗しました: %v (out=%s)", args, err, out)
	}
}
