package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

	printTSV(res, engine.Options{})
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}

	if !strings.Contains(string(out), "TYPE\tAUTHOR") {
		t.Fatalf("TSVヘッダーが出力されていません: %q", string(out))
	}
}

func TestPrintTSVWithAge列を追加する(t *testing.T) {
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
			Author:  "A",
			Email:   "a@example.com",
			Date:    "2024-01-01",
			AgeDays: 12,
			File:    "main.go",
			Line:    5,
		}},
	}

	printTSV(res, engine.Options{WithAge: true})
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		t.Fatalf("TSV出力の行数が不足しています: %q", string(out))
	}
	header := lines[0]
	if !strings.Contains(header, "\tAGE") {
		t.Fatalf("ヘッダーにAGE列が挿入されていません: %q", header)
	}
	if strings.Index(header, "DATE") > strings.Index(header, "AGE") {
		t.Fatalf("AGE列の位置がDATEより前になっています: %q", header)
	}
	if !strings.Contains(lines[1], "\t12\t") {
		t.Fatalf("AGE列の値が出力されていません: %q", lines[1])
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

	printTSV(res, engine.Options{})
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

	printTable(res, engine.Options{})
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

func TestPrintTableWithAge列を追加する(t *testing.T) {
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
			Author:  "B",
			Email:   "b@example.com",
			Date:    "2024-02-01",
			AgeDays: 7,
			File:    "util.go",
			Line:    20,
		}},
	}

	printTable(res, engine.Options{WithAge: true})
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("出力の読み込みに失敗しました: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "AGE") {
		t.Fatalf("ヘッダーにAGE列がありません: %q", text)
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) < 2 {
		t.Fatalf("テーブル出力の行数が不足しています: %q", text)
	}
	header := strings.Fields(lines[0])
	idx := -1
	for i, v := range header {
		if v == "AGE" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("ヘッダーにAGE列が見つかりません: %v", header)
	}
	row := strings.Fields(lines[1])
	if len(row) <= idx || row[idx] != "7" {
		t.Fatalf("AGE列の値が期待通りではありません: %v", row)
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

func TestSortItemsByAge優先度(t *testing.T) {
	items := []engine.Item{
		{File: "b.go", Line: 10, AgeDays: 1},
		{File: "a.go", Line: 5, AgeDays: 10},
		{File: "a.go", Line: 3, AgeDays: 10},
		{File: "c.go", Line: 1, AgeDays: 0},
	}

	sortItems(items, "-age")

	if items[0].File != "a.go" || items[0].Line != 3 {
		t.Fatalf("最古の項目が先頭ではありません: %+v", items[0])
	}
	if items[1].File != "a.go" || items[1].Line != 5 {
		t.Fatalf("同じ日数のタイブレークが期待と異なります: %+v", items[1])
	}
	if items[2].File != "b.go" {
		t.Fatalf("3番目の要素が期待と異なります: %+v", items[2])
	}
	if items[3].File != "c.go" {
		t.Fatalf("末尾の項目が期待と異なります: %+v", items[3])
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
