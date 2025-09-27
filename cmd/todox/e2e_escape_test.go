//go:build e2e

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestRenderはHTMLエスケープでXSSを防止する(t *testing.T) {
	t.Parallel()

	if !hasBrowser() {
		t.Skip("Chrome/Chromiumが見つからないためスキップします")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML()))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// chromedp navigation can take some time in CI environments.
	ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	fixture := fmt.Sprintf(`({
                items: [{
                        kind: 'TODO',
                        author: 'Alice & Bob',
                        email: 'alice<danger>@example.com',
                        date: '2024-01-02',
                        commit: '%s',
                        file: 'dir/<file>&.txt',
                        line: 12,
                        comment: 'hello <img src=x onerror=alert(1)> & <>',
                        message: '<b>bold</b> & <>',
                }],
                errors: [{
                        file: 'err<file>&',
                        line: 0,
                        stage: 'git<stage>',
                        message: 'failed <script>alert(1)</script>',
                }]
        })`, strings.Repeat("a", 40))

	var kind, author, email, date, commit, location, comment, message string
	var escapedAuthorHTML string
	var nodeCount int
	var commentCellHTML string

	err := chromedp.Run(ctx,
		chromedp.Navigate(srv.URL),
		chromedp.WaitVisible(`#out`, chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('out').innerHTML = '';`, nil),
		chromedp.Evaluate(fmt.Sprintf(`const data = %s; document.getElementById('out').innerHTML = render(data);`, fixture), nil),
		chromedp.Text(`#out tbody tr td:nth-child(1)`, &kind, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(2)`, &author, chromedp.ByQuery),
		chromedp.InnerHTML(`#out tbody tr td:nth-child(2)`, &escapedAuthorHTML, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(3)`, &email, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(4)`, &date, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(5) code`, &commit, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(6) code`, &location, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(7)`, &comment, chromedp.ByQuery),
		chromedp.InnerHTML(`#out tbody tr td:nth-child(7)`, &commentCellHTML, chromedp.ByQuery),
		chromedp.Text(`#out tbody tr td:nth-child(8)`, &message, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelectorAll('#out img, #out script').length`, &nodeCount),
	)
	if err != nil {
		t.Fatalf("chromedpの操作に失敗しました: %v", err)
	}

	if kind != "TODO" {
		t.Fatalf("種別が期待値と異なります: %q", kind)
	}
	if author != "Alice & Bob" {
		t.Fatalf("著者が期待値と異なります: %q", author)
	}
	if !strings.Contains(escapedAuthorHTML, "Alice &amp; Bob") {
		t.Fatalf("著者セルのHTMLがエスケープされていません: %q", escapedAuthorHTML)
	}
	if email != "alice<danger>@example.com" {
		t.Fatalf("メールが期待値と異なります: %q", email)
	}
	if !strings.Contains(email, "<") || !strings.Contains(email, ">") {
		t.Fatalf("メールテキストから特殊文字が消えています: %q", email)
	}
	if date != "2024-01-02" {
		t.Fatalf("日付が期待値と異なります: %q", date)
	}
	if commit != strings.Repeat("a", 8) {
		t.Fatalf("コミットの短縮表示が期待値と異なります: %q", commit)
	}
	if location != "dir/<file>&.txt:12" {
		t.Fatalf("ロケーションが期待値と異なります: %q", location)
	}
	if !strings.Contains(comment, "<img src=x onerror=alert(1)>") || !strings.Contains(comment, "&") {
		t.Fatalf("コメントのテキストが期待値と異なります: %q", comment)
	}
	if !strings.Contains(commentCellHTML, "&lt;img") || !strings.Contains(commentCellHTML, "&amp;") {
		t.Fatalf("コメントセルがエスケープされていません: %q", commentCellHTML)
	}
	if message != "<b>bold</b> & <>" {
		t.Fatalf("メッセージが期待値と異なります: %q", message)
	}
	if nodeCount != 0 {
		t.Fatalf("危険なノードが挿入されています: %d", nodeCount)
	}
}

func hasBrowser() bool {
	candidates := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}
	for _, name := range candidates {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}
