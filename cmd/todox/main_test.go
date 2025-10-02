package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dop251/goja"

	"github.com/phyten/todox/internal/engine"
	"github.com/phyten/todox/internal/termcolor"
	"github.com/phyten/todox/internal/textutil"
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

	sel, err := ResolveFields("", true, true, false, false)
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

	sel, err := ResolveFields("", true, false, false, false)
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

	sel, err := ResolveFields("", true, false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	printTable(res, sel, tableColorConfig{})
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

	sel, err := ResolveFields("", false, false, true, false)
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

func TestPrintTSVは常に非カラー(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	res := &engine.Result{
		Items: []engine.Item{{
			Kind:   "TODO",
			Author: "No Color",
			File:   "main.go",
			Line:   1,
		}},
	}

	sel, err := ResolveFields("type,author", false, false, false, false)
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
	if textutil.StripANSI(text) != text {
		t.Fatalf("TSV 出力に ANSI エスケープが含まれています: %q", text)
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

	sel, err := ResolveFields("", false, false, true, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	res.HasAge = sel.ShowAge

	printTable(res, sel, tableColorConfig{})
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "AGE") {
		t.Fatalf("AGE ヘッダーが含まれていません: %q", text)
	}
	if !strings.Contains(text, "    5  abcdef01") {
		t.Fatalf("AGE 列の値が期待通りではありません: %q", text)
	}
}

func TestPrintTableは全角半角混在でも桁が揃う(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	res := &engine.Result{
		Items: []engine.Item{
			{
				Kind: "TODO", Author: "山田太郎", File: "サービス.go", Line: 12,
			},
			{
				Kind: "TODO", Author: "Alice", File: "main.go", Line: 7,
			},
		},
	}

	sel, err := ResolveFields("type,author,location", false, false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	printTable(res, sel, tableColorConfig{})
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("テーブル出力が想定外です: %q", string(out))
	}

	header := lines[0]
	row1 := lines[1]
	row2 := lines[2]

	locHeaderIdx := strings.Index(header, "LOCATION")
	if locHeaderIdx < 0 {
		t.Fatalf("LOCATION ヘッダーが見つかりません: %q", header)
	}
	idx1 := strings.Index(row1, "サービス.go:12")
	if idx1 < 0 {
		t.Fatalf("全角データ列が見つかりません: %q", row1)
	}
	idx2 := strings.Index(row2, "main.go:7")
	if idx2 < 0 {
		t.Fatalf("半角データ列が見つかりません: %q", row2)
	}

	headerPrefixWidth := textutil.VisibleWidth(header[:locHeaderIdx])
	row1PrefixWidth := textutil.VisibleWidth(row1[:idx1])
	row2PrefixWidth := textutil.VisibleWidth(row2[:idx2])

	if row1PrefixWidth != headerPrefixWidth {
		t.Fatalf("全角行のロケーション列開始幅が揃っていません: got=%d want=%d", row1PrefixWidth, headerPrefixWidth)
	}
	if row2PrefixWidth != headerPrefixWidth {
		t.Fatalf("半角行のロケーション列開始幅が揃っていません: got=%d want=%d", row2PrefixWidth, headerPrefixWidth)
	}

	if w1, w2 := textutil.VisibleWidth(row1), textutil.VisibleWidth(row2); w1 != w2 {
		t.Fatalf("行全体の表示幅が一致しません: row1=%d row2=%d", w1, w2)
	}
}

func TestPrintTableはカラーを有効化するとANSIコードを含む(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	res := &engine.Result{
		Items: []engine.Item{{
			Kind:   "TODO",
			Author: "Color Tester",
			Email:  "color@example.com",
			Date:   "2024-06-01",
			File:   "main.go",
			Line:   3,
		}},
	}

	sel, err := ResolveFields("", false, false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	printTable(res, sel, tableColorConfig{enabled: true, profile: termcolor.ProfileBasic8})
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "\x1b[1;4mTYPE") {
		t.Fatalf("ヘッダーの装飾が付与されていません: %q", text)
	}
	if !strings.Contains(text, "\x1b[1;33mTODO") {
		t.Fatalf("TYPE 列に色が付与されていません: %q", text)
	}
}

func TestTableColorNeverDisablesANSI(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("パイプの作成に失敗しました: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	res := &engine.Result{
		Items: []engine.Item{{
			Kind:   "TODO",
			Author: "Never Color",
			File:   "a.go",
			Line:   1,
		}},
	}

	sel, err := ResolveFields("type,author", false, false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}

	printTable(res, sel, tableColorConfig{enabled: false, profile: termcolor.ProfileBasic8})
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	if textutil.StripANSI(text) != text {
		t.Fatalf("カラー無効時に ANSI エスケープが混入しました: %q", text)
	}
}

func TestTableColorEnvNoColorBeatsForce(t *testing.T) {
	mode := termcolor.DetectMode(os.Stdout, map[string]string{
		"NO_COLOR":    "1",
		"FORCE_COLOR": "1",
	})
	if mode != termcolor.ModeNever {
		t.Fatalf("NO_COLOR should override FORCE_COLOR, got %v", mode)
	}
}

type stubRunner struct{}

func (stubRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	if name == "git" && len(args) >= 3 && args[0] == "config" && args[1] == "--get" {
		key := args[2]
		if key == "remote.origin.url" {
			return []byte("https://github.com/example/demo.git\n"), nil, nil
		}
		if key == "remote.upstream.url" {
			return []byte("ssh://git@github.example.com:2222/team/demo.git\n"), nil, nil
		}
	}
	return nil, nil, fmt.Errorf("unexpected command: %s %v", name, args)
}

type errorRunner struct{}

func (errorRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	return nil, []byte("fatal: not a git repository"), fmt.Errorf("exit status 128")
}

func TestApplyLinkColumnAddsURL(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Commit: "1234567890abcdef1234567890abcdef12345678", File: "docs/readme.md", Line: 7}}}
	sel := FieldSelection{NeedURL: true, ShowURL: true}
	if err := applyLinkColumn(context.Background(), stubRunner{}, ".", res, sel); err != nil {
		t.Fatalf("applyLinkColumn failed: %v", err)
	}
	if !res.HasURL {
		t.Fatalf("HasURL not set: %+v", res)
	}
	if len(res.Items) == 0 || res.Items[0].URL == "" {
		t.Fatalf("URL not populated: %+v", res.Items)
	}
	if !strings.Contains(res.Items[0].URL, "?plain=1#L7") {
		t.Fatalf("markdown URL should include plain mode: %s", res.Items[0].URL)
	}
}

func TestApplyLinkColumnGracefullyHandlesErrors(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Commit: "deadbeef", File: "foo.go", Line: 12}}}
	sel := FieldSelection{NeedURL: true, ShowURL: true}
	if err := applyLinkColumn(context.Background(), errorRunner{}, ".", res, sel); err != nil {
		t.Fatalf("applyLinkColumn should not fail: %v", err)
	}
	if !res.HasURL {
		t.Fatalf("HasURL should still be true: %+v", res)
	}
	if res.Items[0].URL != "" {
		t.Fatalf("URL should be blank on failure: %+v", res.Items)
	}
	if len(res.Errors) == 0 {
		t.Fatalf("expected link error to be recorded: %+v", res)
	}
	if res.Errors[0].Stage != "link" {
		t.Fatalf("unexpected error stage: %+v", res.Errors[0])
	}
	if res.ErrorCount != len(res.Errors) {
		t.Fatalf("error count not updated: %+v", res)
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

func TestWebRenderOmitsURLColumnWhenFlagDisabled(t *testing.T) {
	rt := goja.New()
	for _, fn := range []string{"escText", "escAttr", "render"} {
		if _, err := rt.RunString(extractJSFunction(t, fn)); err != nil {
			t.Fatalf("failed to load %s: %v", fn, err)
		}
	}
	noURLScript := `render({items:[{kind:"TODO",author:"Alice",email:"alice@example.com",date:"2024-01-01",file:"main.go",line:7,commit:"1234567890abcdef"}],errors:[],has_comment:false,has_message:false,has_age:false,has_url:false});`
	yesURLScript := `render({items:[{kind:"TODO",author:"Alice",email:"alice@example.com",date:"2024-01-01",file:"main.go",line:7,commit:"1234567890abcdef",url:"https://example.com/blob"}],errors:[],has_comment:false,has_message:false,has_age:false,has_url:true});`
	noVal, err := rt.RunString(noURLScript)
	if err != nil {
		t.Fatalf("render without URL failed: %v", err)
	}
	yesVal, err := rt.RunString(yesURLScript)
	if err != nil {
		t.Fatalf("render with URL failed: %v", err)
	}
	noHTML := noVal.String()
	yesHTML := yesVal.String()
	if strings.Contains(noHTML, "<th>URL</th>") || strings.Contains(noHTML, "link-icon") {
		t.Fatalf("URL 列は has_url=false では表示されない想定です: %s", noHTML)
	}
	if !strings.Contains(yesHTML, "<th>URL</th>") {
		t.Fatalf("has_url=true で URL ヘッダーが欠けています: %s", yesHTML)
	}
	if !strings.Contains(yesHTML, "aria-label=\"GitHub で開く\"") {
		t.Fatalf("アクセシブルラベルが不足しています: %s", yesHTML)
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

func extractJSFunction(t *testing.T, name string) string {
	marker := "function " + name + "("
	idx := strings.Index(webAppHTML, marker)
	if idx < 0 {
		t.Fatalf("function %s not found", name)
	}
	rest := webAppHTML[idx:]
	end := strings.Index(rest, "\nfunction ")
	if end == -1 {
		end = strings.Index(rest, "\n</script>")
	}
	if end == -1 {
		t.Fatalf("could not determine end of function %s", name)
	}
	return rest[:end]
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
