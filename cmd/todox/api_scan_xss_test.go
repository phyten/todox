package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/example/todox/internal/engine"
)

func TestAPIScanHandlerはHTMLエスケープせずに生値を返す(t *testing.T) {
	repoDir := t.TempDir()

	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.name", "Tester <&>")
	runGit(t, repoDir, "config", "user.email", "tester@example.com")

	src := `package main

// TODO check <img src=x onerror=alert(1)> & <>
func main() {}
`
	if err := os.MkdirAll(filepath.Join(repoDir, "dir"), 0o755); err != nil {
		t.Fatalf("ディレクトリ作成に失敗しました: %v", err)
	}
	filePath := filepath.Join(repoDir, "dir", "todo<>&.go")
	if err := os.WriteFile(filePath, []byte(src), 0o644); err != nil {
		t.Fatalf("テストファイル作成に失敗しました: %v", err)
	}

	runGit(t, repoDir, "add", "dir/todo<>&.go")
	runGit(t, repoDir, "commit", "-m", "Add TODO <tag> & track")

	req := httptest.NewRequest(http.MethodGet, "/api/scan?with_comment=1&with_message=1", nil)
	rr := httptest.NewRecorder()

	handler := apiScanHandler(repoDir)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("HTTPステータスが200ではありません: got=%d body=%s", rr.Code, rr.Body.String())
	}

	var res engine.Result
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("JSONのデコードに失敗しました: %v", err)
	}

	if len(res.Items) != 1 {
		t.Fatalf("期待する件数と異なります: got=%d", len(res.Items))
	}

	item := res.Items[0]
	if !res.HasComment {
		t.Fatalf("HasComment が真ではありません")
	}
	if !res.HasMessage {
		t.Fatalf("HasMessage が真ではありません")
	}

	if item.File != "dir/todo<>&.go" {
		t.Fatalf("ファイル名がエスケープされている可能性があります: got=%q", item.File)
	}
	if item.Line == 0 {
		t.Fatalf("行番号が取得できていません")
	}
	if item.Comment != "TODO check <img src=x onerror=alert(1)> & <>" {
		t.Fatalf("コメントが変換されています: got=%q", item.Comment)
	}
	if item.Message != "Add TODO <tag> & track" {
		t.Fatalf("コミットメッセージが変換されています: got=%q", item.Message)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v に失敗しました: %v\n%s", args, err, string(out))
	}
}
