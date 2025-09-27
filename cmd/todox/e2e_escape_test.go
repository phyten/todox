//go:build e2e

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func hasHeadlessChrome() bool {
	candidates := []string{
		"google-chrome", "google-chrome-stable", "chrome", "chromium", "chromium-browser", "headless-shell",
	}
	for _, name := range candidates {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func TestRenderはDOMに危険なノードを挿入しない(t *testing.T) {
	if !hasHeadlessChrome() {
		t.Skip("ヘッドレスChromeが見つからないためスキップします")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := io.WriteString(w, webAppHTML); err != nil {
			t.Fatalf("HTMLの書き込みに失敗しました: %v", err)
		}
	}))
	defer srv.Close()

	allocOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	allocOpts = append(allocOpts,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	fixtureScript := `(function(){
const fixture={
 has_comment:true,
 has_message:true,
 items:[{
  kind:'TODO',
  author:'Alice & Bob',
  email:'alice<bob>@example.com',
  date:'2025-01-01',
  commit:'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  file:'dir/<file>&.txt',
  line:12,
  comment:'call <img src=x onerror=alert(1)> & <>',
  message:'<b>bold</b> & <>'
 }],
 errors:[{
  file:'err<&>.go',
  line:0,
  stage:'git<stage>',
  message:'bad <msg> & stuff'
 }]
};
document.getElementById('out').innerHTML=render(fixture);
return true;
})();`

	var cellTexts []string
	var commentHTML string
	var locationHTML string
	var errorCodeHTML string
	var suspiciousCount int

	if err := chromedp.Run(ctx,
		chromedp.Navigate(srv.URL),
		chromedp.WaitVisible("#out", chromedp.ByQuery),
		chromedp.Evaluate(fixtureScript, nil),
		chromedp.WaitVisible("#out table", chromedp.ByQuery),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('#out tbody tr td'), el => el.textContent)`, &cellTexts),
		chromedp.Evaluate(`document.querySelector('#out tbody tr td:nth-child(7)').innerHTML`, &commentHTML),
		chromedp.Evaluate(`document.querySelector('#out tbody tr td:nth-child(6)').innerHTML`, &locationHTML),
		chromedp.Evaluate(`document.querySelector('.errors code').innerHTML`, &errorCodeHTML),
		chromedp.Evaluate(`document.querySelectorAll('#out img, #out script').length`, &suspiciousCount),
	); err != nil {
		t.Fatalf("chromedpの実行に失敗しました: %v", err)
	}

	if len(cellTexts) != 8 {
		t.Fatalf("セル数が想定外です: %d", len(cellTexts))
	}

	expected := []string{
		"TODO",
		"Alice & Bob",
		"alice<bob>@example.com",
		"2025-01-01",
		"aaaaaaaa",
		"dir/<file>&.txt:12",
		"call <img src=x onerror=alert(1)> & <>",
		"<b>bold</b> & <>",
	}
	for i, got := range cellTexts {
		if got != expected[i] {
			t.Fatalf("セル%dの内容が一致しません: got=%q want=%q", i, got, expected[i])
		}
	}

	if suspiciousCount != 0 {
		t.Fatalf("危険なノードが挿入されています: %d", suspiciousCount)
	}

	if !strings.Contains(commentHTML, "&lt;img src=x onerror=alert(1)&gt;") {
		t.Fatalf("コメントセルがHTMLエスケープされていません: %q", commentHTML)
	}

	if !strings.Contains(locationHTML, "dir/&lt;file&gt;&amp;.txt:12") {
		t.Fatalf("ロケーションセルのエスケープ結果が不正です: %q", locationHTML)
	}

	if errorCodeHTML != "err&lt;&amp;&gt;.go:—" {
		t.Fatalf("エラー位置のエスケープが不正です: %q", errorCodeHTML)
	}
}
