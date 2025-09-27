//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/chromedp/chromedp"
)

func TestRenderは危険な文字をエスケープする(t *testing.T) {
	browserPath, ok := findBrowser()
	if !ok {
		t.Skip("Chrome/Chromium が見つからないためスキップします")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveIndexHTML(w)
	}))
	defer srv.Close()

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
	)...)
	defer cancel()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	payload := map[string]any{
		"items": []map[string]any{
			{
				"kind":    "TODO",
				"author":  "A&B",
				"email":   "x<y>@example.com",
				"date":    "2025-01-01",
				"commit":  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"file":    "dir/<file>&.txt",
				"line":    12,
				"comment": "hello <img src=x onerror=alert(1)> & <>",
				"message": "<b>bold</b> & <>",
			},
		},
		"errors": []map[string]any{
			{
				"file":    "dir/<file>&.txt",
				"line":    0,
				"stage":   "git&blame",
				"message": "<fail> & reason",
			},
		},
	}
	js, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("fixture の JSON 化に失敗しました: %v", err)
	}

	script := fmt.Sprintf(`(function(){
const data=%s;
document.getElementById('out').innerHTML=render(data);
})();`, js)

	if err := chromedp.Run(ctx,
		chromedp.Navigate(srv.URL),
		chromedp.WaitVisible("#f", chromedp.ByID),
		chromedp.Evaluate(script, nil),
		chromedp.WaitVisible("#out table", chromedp.ByQuery),
	); err != nil {
		t.Fatalf("chromedp 操作に失敗しました: %v", err)
	}

	var (
		authorText   string
		emailText    string
		commentText  string
		messageText  string
		locationText string
	)

	tasks := chromedp.Tasks{
		chromedp.Text(`#out tbody tr td:nth-child(2)`, &authorText, chromedp.NodeVisible, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(3)`, &emailText, chromedp.NodeVisible, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(7)`, &commentText, chromedp.NodeVisible, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(8)`, &messageText, chromedp.NodeVisible, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(6)`, &locationText, chromedp.NodeVisible, chromedp.ByQuery),
	}

	if err := chromedp.Run(ctx, tasks); err != nil {
		t.Fatalf("テキストの取得に失敗しました: %v", err)
	}

	if authorText != "A&B" {
		t.Fatalf("著者テキストが変換されています: %q", authorText)
	}
	if emailText != "x<y>@example.com" {
		t.Fatalf("メールテキストが変換されています: %q", emailText)
	}
	if commentText != "hello <img src=x onerror=alert(1)> & <>" {
		t.Fatalf("コメントテキストが変換されています: %q", commentText)
	}
	if messageText != "<b>bold</b> & <>" {
		t.Fatalf("メッセージテキストが変換されています: %q", messageText)
	}
	if locationText != "dir/<file>&.txt:12" {
		t.Fatalf("ロケーションテキストが変換されています: %q", locationText)
	}

	var (
		authorHTML   string
		commentHTML  string
		locationHTML string
		errorLocHTML string
		unsafeNodes  int
	)

	if err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Evaluate(`document.querySelector('#out tbody tr td:nth-child(2)').innerHTML`, &authorHTML),
		chromedp.Evaluate(`document.querySelector('#out tbody tr td:nth-child(7)').innerHTML`, &commentHTML),
		chromedp.Evaluate(`document.querySelector('#out tbody tr td:nth-child(6)').innerHTML`, &locationHTML),
		chromedp.Evaluate(`document.querySelector('#out .errors li code').innerHTML`, &errorLocHTML),
		chromedp.Evaluate(`document.querySelectorAll('#out img, #out script').length`, &unsafeNodes),
	}); err != nil {
		t.Fatalf("HTML の確認に失敗しました: %v", err)
	}

	if authorHTML != "A&amp;B" {
		t.Fatalf("著者HTMLのエスケープが不足しています: %q", authorHTML)
	}
	if commentHTML != "hello &lt;img src=x onerror=alert(1)&gt; &amp; &lt;&gt;" {
		t.Fatalf("コメントHTMLのエスケープが不足しています: %q", commentHTML)
	}
	if locationHTML != "dir/&lt;file&gt;&amp;.txt:12" {
		t.Fatalf("ロケーションHTMLのエスケープが不足しています: %q", locationHTML)
	}
	if errorLocHTML != "dir/&lt;file&gt;&amp;.txt:—" {
		t.Fatalf("エラー位置HTMLのエスケープが不足しています: %q", errorLocHTML)
	}
	if unsafeNodes != 0 {
		t.Fatalf("危険なノードが生成されています: %d", unsafeNodes)
	}
}

func findBrowser() (string, bool) {
	candidates := []string{
		"google-chrome-stable",
		"google-chrome",
		"chromium",
		"chromium-browser",
	}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	return "", false
}
