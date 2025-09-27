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

func TestAPIScanHandlerはJSONをエスケープせず返す(t *testing.T) {
	repoDir := t.TempDir()

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	git("init")
	git("config", "user.name", "テストユーザー")
	git("config", "user.email", "tester@example.com")

	src := "package main\n\n// TODO: <script>alert('xss')</script> & <>\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("ファイルの作成に失敗しました: %v", err)
	}

	git("add", ".")
	git("commit", "-m", "feat: <b>bold</b> & <>")

	handler := apiScanHandler(repoDir)
	req := httptest.NewRequest("GET", "/api/scan?with_comment=1&with_message=1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("予期しないステータス: %d\n%s", rr.Code, rr.Body.String())
	}

	var res engine.Result
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("JSONのデコードに失敗しました: %v", err)
	}

	if len(res.Items) == 0 {
		t.Fatalf("TODO項目が見つかりません: %+v", res)
	}

	item := res.Items[0]
	if !strings.Contains(item.Comment, "<script>alert('xss')</script> & <>") {
		t.Fatalf("コメントがエスケープされて返却されました: %q", item.Comment)
	}
	if !strings.Contains(item.Message, "<b>bold</b> & <>") {
		t.Fatalf("コミットメッセージがエスケープされて返却されました: %q", item.Message)
	}
}
