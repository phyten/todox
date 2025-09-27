package main

import (
	"io"
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

func TestPrintTSVはコメント改行を空白に変換する(t *testing.T) {
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
		t.Fatalf("コメント中の改行が除去されていません: %q", lines[1])
	}
	if !strings.Contains(lines[1], "調査中 要確認") {
		t.Fatalf("改行が空白に置換されていません: %q", lines[1])
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
