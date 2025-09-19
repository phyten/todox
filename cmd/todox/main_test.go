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
		Items: []engine.Item{{Kind: "TODO", Author: "山田", Email: "yamada@example.com", Date: "2024-01-01", File: "main.go", Line: 42}},
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
		Items: []engine.Item{{Kind: "TODO", Author: "佐藤", Email: "sato@example.com", Date: "2024-02-01", File: "util.go", Line: 10, Comment: "調査中\n要確認"}},
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
