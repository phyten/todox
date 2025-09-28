package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phyten/todox/internal/engine"
)

func TestAPIScanHandlerはignoreWSクエリで責任コミットを切り替える(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Tester")
	runGit(t, repoDir, "config", "user.email", "tester@example.com")

	initialSource := "package main\n\nfunc main() {\n    // TODO: adjust spacing\n}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(initialSource), 0o644); err != nil {
		t.Fatalf("ファイルの作成に失敗しました: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial todo")

	initialSHA := gitRevParse(t, repoDir, "HEAD")

	updatedSource := "package main\n\nfunc main() {\n        // TODO: adjust spacing\n}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(updatedSource), 0o644); err != nil {
		t.Fatalf("ファイルの更新に失敗しました: %v", err)
	}
	runGit(t, repoDir, "commit", "-am", "whitespace tweak")

	latestSHA := gitRevParse(t, repoDir, "HEAD")
	if latestSHA == initialSHA {
		t.Fatal("コミットのSHAが同じです (whitespace tweak が失敗しています)")
	}

	handler := apiScanHandler(repoDir)

	req := httptest.NewRequest(http.MethodGet, "/api/scan", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusOK)
	}

	var res engine.Result
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("TODO が1件ではありません: %+v", res.Items)
	}
	if res.Items[0].Commit != initialSHA {
		t.Fatalf("ignore_ws=true のときに初回コミットが返っていません: got=%s want=%s", res.Items[0].Commit, initialSHA)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/scan?ignore_ws=0", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusOK)
	}

	res = engine.Result{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("TODO が1件ではありません: %+v", res.Items)
	}
	if res.Items[0].Commit != latestSHA {
		t.Fatalf("ignore_ws=0 のときに最新コミットが返っていません: got=%s want=%s", res.Items[0].Commit, latestSHA)
	}
}

func TestAPIScanHandlerはignoreWSの不正値で400を返す(t *testing.T) {
	t.Parallel()

	handler := apiScanHandler(".")
	req := httptest.NewRequest(http.MethodGet, "/api/scan?ignore_ws=maybe", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	if body := rr.Body.String(); !strings.Contains(body, "ignore_ws") {
		t.Fatalf("エラーメッセージが期待通りではありません: %q", body)
	}
}

func TestAPIScanHandlerはjobsパラメータを検証する(t *testing.T) {
	t.Parallel()

	handler := apiScanHandler(".")

	t.Run("範囲外", func(t *testing.T) {
		t.Parallel()

		cases := []string{"0", "65"}
		for _, raw := range cases {
			raw := raw
			t.Run(raw, func(t *testing.T) {
				t.Parallel()

				req := httptest.NewRequest(http.MethodGet, "/api/scan?jobs="+raw, nil)
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)

				if rr.Code != http.StatusBadRequest {
					t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusBadRequest)
				}
				if body := rr.Body.String(); !strings.Contains(body, "jobs must be between 1 and 64") {
					t.Fatalf("エラーメッセージが期待通りではありません: %q", body)
				}
			})
		}
	})

	t.Run("不正な文字列", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/scan?jobs=foo", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusBadRequest)
		}
		if body := rr.Body.String(); !strings.Contains(body, "invalid integer value for jobs") {
			t.Fatalf("エラーメッセージが期待通りではありません: %q", body)
		}
	})
}

func TestAPIScanHandlerはjobsの境界値を受け付ける(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Tester")
	runGit(t, repoDir, "config", "user.email", "tester@example.com")
	if err := os.WriteFile(filepath.Join(repoDir, "todo.txt"), []byte("// TODO boundary"), 0o644); err != nil {
		t.Fatalf("ファイルの作成に失敗しました: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")

	handler := apiScanHandler(repoDir)

	cases := []string{"1", "64"}
	for _, raw := range cases {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/api/scan?jobs="+raw, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusOK)
			}
		})
	}
}

func TestAPIScanHandlerはパス関連パラメータを適用する(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Tester")
	runGit(t, repoDir, "config", "user.email", "tester@example.com")

	if err := os.MkdirAll(filepath.Join(repoDir, "src"), 0o755); err != nil {
		t.Fatalf("ディレクトリの作成に失敗しました: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "vendor"), 0o755); err != nil {
		t.Fatalf("ディレクトリの作成に失敗しました: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "src", "main.go"), []byte("package main\n// TODO in src\n"), 0o644); err != nil {
		t.Fatalf("ファイル作成に失敗しました: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "vendor", "lib.go"), []byte("package vendor\n// TODO in vendor\n"), 0o644); err != nil {
		t.Fatalf("ファイル作成に失敗しました: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "add todos")

	handler := apiScanHandler(repoDir)

	checkCount := func(t *testing.T, query string, want int, wantFirst string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/scan"+query, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusOK)
		}
		var res engine.Result
		if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
			t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
		}
		if res.Total != want {
			t.Fatalf("ヒット件数が期待と異なります: got=%d want=%d", res.Total, want)
		}
		if want > 0 && res.Items[0].File != wantFirst {
			t.Fatalf("最初のファイルが期待と異なります: got=%q want=%q", res.Items[0].File, wantFirst)
		}
	}

	checkCount(t, "", 2, "src/main.go")
	checkCount(t, "?path=src", 1, "src/main.go")
	checkCount(t, "?exclude=vendor/**", 1, "src/main.go")
	checkCount(t, "?path_regex=^src/", 1, "src/main.go")
	checkCount(t, "?exclude_typical=1", 1, "src/main.go")
}

func TestAPIScanHandlerは不正なpathRegexで400を返す(t *testing.T) {
	t.Parallel()

	handler := apiScanHandler(".")
	req := httptest.NewRequest(http.MethodGet, "/api/scan?path_regex=[", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	if body := rr.Body.String(); !strings.Contains(body, "invalid --path-regex") {
		t.Fatalf("エラーメッセージが期待通りではありません: %q", body)
	}
}

func gitRevParse(t *testing.T, dir string, rev string) string {
	t.Helper()

	cmd := exec.Command("git", "rev-parse", rev)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s に失敗しました: %v", rev, err)
	}
	return strings.TrimSpace(string(out))
}
