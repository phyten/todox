package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/todox/internal/engine"
)

func TestPrintTSVは出力をフラッシュする(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	res := &engine.Result{
		HasComment: true,
		HasMessage: true,
		Items:      []engine.Item{{Kind: "TODO", Author: "山田", Email: "yamada@example.com", Date: "2024-01-01", File: "main.go", Line: 42}},
	}

	sel, err := ResolveFields("", true, true, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	printTSV(res, sel)
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}

	if !strings.Contains(string(out), "TYPE\tAUTHOR") {
		t.Fatalf("TSVヘッダーが出力されていません: %q", string(out))
	}
}

func TestPrintTSVはコメント改行を可視化して保持する(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	res := &engine.Result{
		HasComment: true,
		Items:      []engine.Item{{Kind: "TODO", Author: "佐藤", Email: "sato@example.com", Date: "2024-02-01", File: "util.go", Line: 10, Comment: "調査中\n要確認"}},
	}

	sel, err := ResolveFields("", true, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	printTSV(res, sel)
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("改行が期待より多いです: %q", text)
	}
	if strings.Contains(lines[1], "\n") {
		t.Fatalf("コメント中の改行が残っています: %q", lines[1])
	}
	if !strings.Contains(lines[1], "調査中⏎要確認") {
		t.Fatalf("改行が可視文字に置換されていません: %q", lines[1])
	}
}

func TestPrintTableは制御文字を無害化する(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	res := &engine.Result{
		HasComment: true,
		Items: []engine.Item{{
			Kind:    "FIXME",
			Author:  "田中",
			Email:   "tanaka@example.com",
			Date:    "2024-03-01",
			File:    "handler.go",
			Line:    99,
			Comment: "1行目\r\n2行目\t継続\r3行目",
		}},
	}

	sel, err := ResolveFields("", true, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	printTable(res, sel)
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	if strings.ContainsAny(text, "\r\t") {
		t.Fatalf("テーブル出力に制御文字が残っています: %q", text)
	}
	if !strings.Contains(text, "1行目⏎2行目 継続3行目") {
		t.Fatalf("制御文字が期待通りに置換されていません: %q", text)
	}
}

func TestPrintTSVはAGE列を表示できる(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	res := &engine.Result{
		Items: []engine.Item{{
			Kind:    "TODO",
			Author:  "Age Tester",
			Email:   "age@example.com",
			Date:    "2024-04-01",
			AgeDays: 42,
			Commit:  "1234567890abcdef",
			File:    "main.go",
			Line:    7,
		}},
	}

	sel, err := ResolveFields("", false, false, true)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	res.HasAge = sel.ShowAge

	printTSV(res, sel)
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "AGE") {
		t.Fatalf("AGE ヘッダーが含まれていません: %q", text)
	}
	if !strings.Contains(text, "\t42\t") {
		t.Fatalf("AGE 列の値が期待通りではありません: %q", text)
	}
}

func TestPrintTableはAGE列を表示できる(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	res := &engine.Result{
		Items: []engine.Item{{
			Kind:    "FIXME",
			Author:  "Table Tester",
			Email:   "table@example.com",
			Date:    "2024-05-10",
			AgeDays: 5,
			Commit:  "abcdef0123456789",
			File:    "handler.go",
			Line:    12,
		}},
	}

	sel, err := ResolveFields("", false, false, true)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	res.HasAge = sel.ShowAge

	printTable(res, sel)
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "AGE") {
		t.Fatalf("AGE ヘッダーが含まれていません: %q", text)
	}
	if !strings.Contains(text, "  5    abcdef01") {
		t.Fatalf("AGE 列の値が期待通りではありません: %q", text)
	}
}

func TestApplySortは年齢順に並び替える(t *testing.T) {
	items := []engine.Item{
		{AgeDays: 1, File: "b.go", Line: 30},
		{AgeDays: 10, File: "a.go", Line: 20},
		{AgeDays: 10, File: "a.go", Line: 10},
		{AgeDays: 3, File: "c.go", Line: 5},
	}

	spec, err := ParseSortSpec("-age")
	if err != nil {
		t.Fatalf("ParseSortSpec failed: %v", err)
	}

	ApplySort(items, spec)

	if items[0].Line != 10 || items[1].Line != 20 {
		t.Fatalf("AGE 降順＋ファイル/行のタイブレークが期待通りではありません: %+v", items[:2])
	}
	if items[len(items)-1].AgeDays != 1 {
		t.Fatalf("最も若い項目が末尾に来ていません: %+v", items)
	}
}

func TestReportErrorsは標準エラーに概要を出力する(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	res := &engine.Result{
		ErrorCount: 3,
		Errors: []engine.ItemError{
			{File: "a.go", Line: 10, Stage: "git blame", Message: "exit status 1"},
			{File: "b.go", Line: 20, Stage: "git show", Message: "no commit"},
			{File: "", Line: 0, Stage: "", Message: "mystery"},
		},
	}

	reportErrors(res)
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "3 error(s)") {
		t.Fatalf("エラー件数が出力されていません: %q", text)
	}
	if !strings.Contains(text, "a.go:10 [git blame] exit status 1") {
		t.Fatalf("詳細行が出力されていません: %q", text)
	}
	if !strings.Contains(text, "(unknown location) [git] mystery") {
		t.Fatalf("不明位置の行が期待通りではありません: %q", text)
	}
}

func TestParseBoolParamは受け入れ値を解釈する(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		value   string
		want    bool
		wantErr bool
	}{
		"未指定は偽":    {want: false},
		"空文字は偽":    {value: "", want: false},
		"1は真":      {value: "1", want: true},
		"trueは真":   {value: "true", want: true},
		"TRUEは真":   {value: "TRUE", want: true},
		"yesは真":    {value: "yes", want: true},
		"onは真":     {value: "on", want: true},
		"0は偽":      {value: "0", want: false},
		"falseは偽":  {value: "false", want: false},
		"FALSEは偽":  {value: "FALSE", want: false},
		"noは偽":     {value: "no", want: false},
		"offは偽":    {value: "off", want: false},
		"無効値はエラー":  {value: "maybe", wantErr: true},
		"前後空白はトリム": {value: "  true  ", want: true},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			q := map[string][]string{}
			if tc.value != "" || (tc.value == "" && name == "空文字は偽") {
				q["flag"] = []string{tc.value}
			}

			got, err := parseBoolParam(q, "flag")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("エラーを期待しましたが nil でした")
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got != tc.want {
				t.Fatalf("結果が一致しません: got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestParseIntParamは整数入力を検証する(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		value   string
		want    int
		wantErr bool
	}{
		"未指定は0":   {want: 0},
		"空文字は0":   {value: "", want: 0},
		"空白だけは0":  {value: "   ", want: 0},
		"正の数":     {value: "120", want: 120},
		"負の数も許容":  {value: "-1", want: -1},
		"無効値はエラー": {value: "abc", wantErr: true},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			q := map[string][]string{}
			if tc.value != "" || strings.Contains(name, "空文字") {
				q["n"] = []string{tc.value}
			}

			got, err := parseIntParam(q, "n")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("エラーを期待しましたが nil でした")
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got != tc.want {
				t.Fatalf("結果が一致しません: got=%d want=%d", got, tc.want)
			}
		})
	}
}

func TestAPIScanHandlerは不正なtruncateで400を返す(t *testing.T) {
	t.Parallel()

	handler := apiScanHandler(".")

	req := httptest.NewRequest(http.MethodGet, "/api/scan?truncate=abc", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	if body := rr.Body.String(); !strings.Contains(body, "truncate") {
		t.Fatalf("エラーメッセージにキー名が含まれていません: %q", body)
	}
}

func TestAPIScanHandlerは不正なsortで400を返す(t *testing.T) {
	t.Parallel()

	handler := apiScanHandler(".")
	req := httptest.NewRequest(http.MethodGet, "/api/scan?sort=unknown", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	if body := rr.Body.String(); !strings.Contains(body, "unknown") {
		t.Fatalf("エラーメッセージが具体的ではありません: %q", body)
	}
}

func TestAPIScanHandlerは不正なfieldsで400を返す(t *testing.T) {
	t.Parallel()

	handler := apiScanHandler(".")
	req := httptest.NewRequest(http.MethodGet, "/api/scan?fields=foo", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	if body := rr.Body.String(); !strings.Contains(body, "unknown field") {
		t.Fatalf("エラーメッセージが期待通りではありません: %q", body)
	}
}

func TestAPIScanHandlerはfields指定でHasAgeを設定する(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Tester")
	runGit(t, repoDir, "config", "user.email", "tester@example.com")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("ファイル作成に失敗しました: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	handler := apiScanHandler(repoDir)
	req := httptest.NewRequest(http.MethodGet, "/api/scan?fields=type,age,location", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("ステータスコードが一致しません: got=%d want=%d", rr.Code, http.StatusOK)
	}

	var res engine.Result
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}
	if !res.HasAge {
		t.Fatalf("HasAge が true ではありません: %+v", res)
	}
	if res.HasComment || res.HasMessage {
		t.Fatalf("HasComment/HasMessage が false ではありません: %+v", res)
	}
}
