//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

func findChromePath() (string, bool) {
	candidates := []string{
		os.Getenv("CHROME_PATH"),
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"headless-shell",
	}
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if path, err := exec.LookPath(cand); err == nil {
			return path, true
		}
	}
	return "", false
}

func TestRenderEscapingE2E(t *testing.T) {
	chromePath, ok := findChromePath()
	if !ok {
		t.Skip("Chrome/Chromium が見つからないため E2E をスキップします")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML()))
	}))
	defer srv.Close()

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	fixture := map[string]any{
		"has_comment": true,
		"has_message": true,
		"items": []map[string]any{
			{
				"kind":    "TODO",
				"author":  "A&B",
				"email":   "x<y>@example.com",
				"date":    "2025-01-01",
				"commit":  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"file":    "dir/<file>&.txt",
				"line":    12,
				"comment": "TODO: hello <img src=x onerror=alert(1)> & <>",
				"message": "<b>bold</b> & <>",
			},
		},
		"errors": []map[string]any{
			{
				"file":    "err<&>.go",
				"line":    0,
				"stage":   "git<stage>",
				"message": "bad & <>",
			},
		},
	}

	payload, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("fixture の生成に失敗しました: %v", err)
	}

	script := "const data = " + string(payload) + "; document.getElementById('out').innerHTML = render(data);"

	if err := chromedp.Run(ctx,
		chromedp.Navigate(srv.URL),
		chromedp.WaitVisible(`#out`, chromedp.ByID),
		chromedp.Evaluate(script, nil),
	); err != nil {
		t.Fatalf("chromedp 実行エラー: %v", err)
	}

	type cellCheck struct {
		sel  string
		want string
	}

	checks := []cellCheck{
		{sel: "#out tbody tr td:nth-child(2)", want: "A&B"},
		{sel: "#out tbody tr td:nth-child(3)", want: "x<y>@example.com"},
		{sel: "#out tbody tr td:nth-child(5) code", want: "aaaaaaaa"},
		{sel: "#out tbody tr td:nth-child(6) code", want: "dir/<file>&.txt:12"},
		{sel: "#out tbody tr td:nth-child(7)", want: "TODO: hello <img src=x onerror=alert(1)> & <>"},
		{sel: "#out tbody tr td:nth-child(8)", want: "<b>bold</b> & <>"},
		{sel: "#out .errors li code", want: "err<&>.go:—"},
		{sel: "#out .errors li", want: "err<&>.go:— [git<stage>] bad & <>"},
	}

	for _, tc := range checks {
		tc := tc
		var got string
		if err := chromedp.Run(ctx, chromedp.Text(tc.sel, &got, chromedp.ByQuery)); err != nil {
			t.Fatalf("テキスト取得に失敗しました (%s): %v", tc.sel, err)
		}
		if got != tc.want {
			t.Fatalf("テキストが一致しません (%s): got %q want %q", tc.sel, got, tc.want)
		}
	}

	var commentHTML string
	if err := chromedp.Run(ctx, chromedp.InnerHTML("#out tbody tr td:nth-child(7)", &commentHTML, chromedp.ByQuery)); err != nil {
		t.Fatalf("HTML取得に失敗しました: %v", err)
	}
	if !strings.Contains(commentHTML, "&lt;img") || !strings.Contains(commentHTML, "&amp;") {
		t.Fatalf("コメントセルのHTMLに期待するエスケープがありません: %s", commentHTML)
	}

	var nodes []*cdp.Node
	if err := chromedp.Run(ctx, chromedp.Nodes("#out img, #out script", &nodes, chromedp.ByQueryAll)); err != nil {
		t.Fatalf("ノード取得に失敗しました: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("エスケープ失敗により危険な要素が生成されています")
	}
}
