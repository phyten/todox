package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dop251/goja"

	"github.com/phyten/todox/internal/engine"
	ghclient "github.com/phyten/todox/internal/host/github"
	"github.com/phyten/todox/internal/output"
	"github.com/phyten/todox/internal/progress"
	"github.com/phyten/todox/internal/termcolor"
	"github.com/phyten/todox/internal/textutil"
)

var webTemplateHTML = mustReadWebTemplate()

func mustReadWebTemplate() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "internal", "web", "templates", "index.html")
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to load web template: %v", err))
	}
	return string(data)
}

func TestWriteJSONResultDisablesHTMLEscape(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Comment: "<tag>"}}}
	var buf bytes.Buffer
	if err := writeJSONResult(&buf, res); err != nil {
		t.Fatalf("writeJSONResult failed: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "\\u003c") {
		t.Fatalf("JSON output should not escape angle brackets: %s", got)
	}
	if !strings.Contains(got, "<tag>") {
		t.Fatalf("JSON output missing raw angle brackets: %s", got)
	}
}

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

	sel, err := output.ResolveFields("", true, true, false, false, false)
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

	sel, err := output.ResolveFields("", true, false, false, false, false)
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

	sel, err := output.ResolveFields("", true, false, false, false, false)
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

	sel, err := output.ResolveFields("", false, false, true, false, false)
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

	sel, err := output.ResolveFields("type,author", false, false, false, false, false)
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

	sel, err := output.ResolveFields("", false, false, true, false, false)
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

	sel, err := output.ResolveFields("type,author,location", false, false, false, false, false)
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

	sel, err := output.ResolveFields("", false, false, false, false, false)
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

	sel, err := output.ResolveFields("type,author", false, false, false, false, false)
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

type prRunner struct{}

func (prRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	if name == "git" && len(args) >= 3 && args[0] == "config" && args[1] == "--get" {
		key := args[2]
		switch key {
		case "remote.origin.url":
			return []byte("https://github.com/example/demo.git\n"), nil, nil
		case "remote.upstream.url":
			return []byte("ssh://git@github.example.com:2222/team/demo.git\n"), nil, nil
		}
	}
	if name == "gh" && len(args) >= 2 && args[0] == "api" && strings.Contains(args[1], "/pulls") {
		payload := `[{"number":10,"title":"Feature","state":"OPEN","html_url":"https://github.com/example/demo/pull/10"},{"number":4,"title":"Bugfix","state":"closed","html_url":"https://github.com/example/demo/pull/4","merged_at":"2024-01-02T03:04:05Z"}]`
		return []byte(payload), nil, nil
	}
	return nil, nil, fmt.Errorf("unexpected command: %s %v", name, args)
}

type countingRunner struct {
	gitConfigCalls int32
}

func (r *countingRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	if name == "git" && len(args) >= 3 && args[0] == "config" && args[1] == "--get" {
		key := args[2]
		if key == "remote.origin.url" || key == "remote.upstream.url" {
			atomic.AddInt32(&r.gitConfigCalls, 1)
			return []byte("https://github.com/example/demo.git\n"), nil, nil
		}
	}
	if name == "gh" && len(args) >= 2 && args[0] == "api" && strings.Contains(args[1], "/pulls") {
		payload := `[{"number":10,"title":"Feature","state":"OPEN","html_url":"https://github.com/example/demo/pull/10"}]`
		return []byte(payload), nil, nil
	}
	return nil, nil, fmt.Errorf("unexpected command: %s %v", name, args)
}

func (r *countingRunner) Calls() int32 {
	return atomic.LoadInt32(&r.gitConfigCalls)
}

type recordingObserver struct {
	mu        sync.Mutex
	snapshots []progress.Snapshot
	last      progress.Snapshot
	done      bool
}

func (o *recordingObserver) Publish(s progress.Snapshot) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.snapshots = append(o.snapshots, s)
	o.last = s
}

func (o *recordingObserver) Done(s progress.Snapshot) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.last = s
	o.done = true
}

func (o *recordingObserver) Snapshots() []progress.Snapshot {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]progress.Snapshot, len(o.snapshots))
	copy(out, o.snapshots)
	return out
}

func (o *recordingObserver) Last() progress.Snapshot {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.last
}

func (o *recordingObserver) DoneCalled() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.done
}

func TestApplyLinkColumnAddsURL(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Commit: "1234567890abcdef1234567890abcdef12345678", File: "docs/readme.md", Line: 7}}}
	sel := output.FieldSelection{NeedURL: true, ShowURL: true}
	var cache remoteInfoCache
	if err := applyLinkColumn(context.Background(), stubRunner{}, ".", &cache, res, sel); err != nil {
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

func TestApplyLinkColumnSetsHasURLWhenHidden(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Commit: "1234567890abcdef1234567890abcdef12345678", File: "main.go", Line: 42}}}
	sel := output.FieldSelection{NeedURL: true, ShowURL: false}
	var cache remoteInfoCache
	if err := applyLinkColumn(context.Background(), stubRunner{}, ".", &cache, res, sel); err != nil {
		t.Fatalf("applyLinkColumn failed: %v", err)
	}
	if !res.HasURL {
		t.Fatalf("HasURL should reflect NeedURL even when hidden: %+v", res)
	}
	if got := res.Items[0].URL; got == "" {
		t.Fatalf("URL should be populated regardless of ShowURL: %+v", res.Items[0])
	}
}

func TestApplyLinkColumnGracefullyHandlesErrors(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Commit: "deadbeef", File: "foo.go", Line: 12}}}
	sel := output.FieldSelection{NeedURL: true, ShowURL: true}
	var cache remoteInfoCache
	if err := applyLinkColumn(context.Background(), errorRunner{}, ".", &cache, res, sel); err != nil {
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

func TestApplyPRColumnsPopulatesPRs(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Commit: "1234567890abcdef1234567890abcdef12345678"}, {Commit: "1234567890abcdef1234567890abcdef12345678"}}}
	sel := output.FieldSelection{NeedPRs: true, ShowPRs: true}
	opts := prOptions{State: "all", Limit: 1, Prefer: "open", Jobs: 4}
	var cache remoteInfoCache
	if err := applyPRColumns(context.Background(), prRunner{}, ".", &cache, res, sel, opts, nil); err != nil {
		t.Fatalf("applyPRColumns failed: %v", err)
	}
	if !res.HasPRs {
		t.Fatalf("HasPRs should be true: %+v", res)
	}
	for idx, item := range res.Items {
		if len(item.PRs) != 1 {
			t.Fatalf("item %d PRs length mismatch: %+v", idx, item.PRs)
		}
		pr := item.PRs[0]
		if pr.Number != 10 || pr.State != "open" {
			t.Fatalf("unexpected PR data: %+v", pr)
		}
		if !strings.Contains(pr.URL, "/pull/10") {
			t.Fatalf("unexpected PR URL: %s", pr.URL)
		}
	}
	if res.ErrorCount != 0 || len(res.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
}

func TestApplyPRColumnsRecordsErrors(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Commit: "cafebabecafebabecafebabecafebabecafebabe"}}}
	sel := output.FieldSelection{NeedPRs: true, ShowPRs: true}
	var cache remoteInfoCache
	if err := applyPRColumns(context.Background(), errorRunner{}, ".", &cache, res, sel, prOptions{State: "all", Limit: 3, Prefer: "open", Jobs: 2}, nil); err != nil {
		t.Fatalf("applyPRColumns should not return error: %v", err)
	}
	if !res.HasPRs {
		t.Fatalf("HasPRs should remain true when PRs requested: %+v", res)
	}
	if len(res.Errors) == 0 {
		t.Fatalf("expected PR error to be recorded")
	}
	if res.Errors[0].Stage != "pr" {
		t.Fatalf("unexpected error stage: %+v", res.Errors[0])
	}
	if res.ErrorCount != len(res.Errors) {
		t.Fatalf("error count mismatch: %+v", res)
	}
}

func TestApplyPRColumnsPublishesProgressSnapshots(t *testing.T) {
	res := &engine.Result{Items: []engine.Item{{Commit: "1234567890abcdef1234567890abcdef12345678"}, {Commit: "1234567890abcdef1234567890abcdef12345678"}}}
	sel := output.FieldSelection{NeedPRs: true, ShowPRs: true}
	opts := prOptions{State: "all", Limit: 2, Prefer: "open", Jobs: 2}
	var cache remoteInfoCache
	obs := &recordingObserver{}
	if err := applyPRColumns(context.Background(), prRunner{}, ".", &cache, res, sel, opts, obs); err != nil {
		t.Fatalf("applyPRColumns failed: %v", err)
	}
	snaps := obs.Snapshots()
	if len(snaps) == 0 {
		t.Fatalf("expected at least one snapshot to be published")
	}
	if snaps[0].Stage != progress.StagePR {
		t.Fatalf("initial snapshot should use PR stage, got %q", snaps[0].Stage)
	}
	final := obs.Last()
	if final.Stage != progress.StagePR {
		t.Fatalf("final snapshot stage mismatch: %q", final.Stage)
	}
	if final.Total == 0 || final.Done != final.Total {
		t.Fatalf("final snapshot should be complete: %+v", final)
	}
	if !obs.DoneCalled() {
		t.Fatalf("observer Done should be invoked")
	}
}

func TestRemoteDetectionSharedAcrossLinkAndPR(t *testing.T) {
	runner := &countingRunner{}
	res := &engine.Result{Items: []engine.Item{{Commit: "1234567890abcdef1234567890abcdef12345678", File: "main.go", Line: 5}}}
	sel := output.FieldSelection{NeedURL: true, ShowURL: true, NeedPRs: true, ShowPRs: true}
	opts := prOptions{State: "all", Limit: 1, Prefer: "open", Jobs: 1}
	var cache remoteInfoCache
	ctx := context.Background()
	before := runner.Calls()
	if before != 0 {
		t.Fatalf("unexpected initial call count: %d", before)
	}
	if err := applyLinkColumn(ctx, runner, ".", &cache, res, sel); err != nil {
		t.Fatalf("applyLinkColumn failed: %v", err)
	}
	mid := runner.Calls()
	if mid == 0 {
		t.Fatalf("expected git remote detection to run at least once")
	}
	if err := applyPRColumns(ctx, runner, ".", &cache, res, sel, opts, nil); err != nil {
		t.Fatalf("applyPRColumns failed: %v", err)
	}
	after := runner.Calls()
	if after != mid {
		t.Fatalf("remote detection should be cached across link and PR: before=%d mid=%d after=%d", before, mid, after)
	}
}

func TestFilterPRsByStateMerged(t *testing.T) {
	prs := []ghclient.PRInfo{
		{Number: 1, State: "open"},
		{Number: 2, State: "merged"},
		{Number: 3, State: "closed"},
	}
	filtered, err := filterPRsByState(prs, "merged", "--pr-state")
	if err != nil {
		t.Fatalf("filterPRsByState returned error: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Number != 2 {
		t.Fatalf("expected only merged PR to remain: %+v", filtered)
	}
	if _, err := filterPRsByState(prs, "unknown", "--pr-state"); err == nil || !strings.Contains(err.Error(), "--pr-state") {
		t.Fatalf("expected error mentioning --pr-state, got: %v", err)
	}
}

func TestSortPRsByPreferenceNonePreservesOrder(t *testing.T) {
	original := []ghclient.PRInfo{{Number: 5, State: "open"}, {Number: 2, State: "merged"}, {Number: 9, State: "closed"}}
	prs := append([]ghclient.PRInfo(nil), original...)
	sortPRsByPreference(prs, "none")
	for i := range prs {
		if prs[i].Number != original[i].Number || prs[i].State != original[i].State {
			t.Fatalf("PR order changed when prefer=none: got=%+v want=%+v", prs, original)
		}
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
	for _, fn := range []string{"escText", "escAttr", "renderBadge", "collapseWhitespace", "ellipsize", "normalizePRTooltip", "renderPRCell", "buildHeaderMeta", "renderTableCell", "renderResultTable"} {
		if _, err := rt.RunString(extractJSFunction(t, fn)); err != nil {
			t.Fatalf("failed to load %s: %v", fn, err)
		}
	}
	noURLScript := `renderResultTable({items:[{kind:"TODO",author:"Alice",email:"alice@example.com",date:"2024-01-01",file:"main.go",line:7,commit:"1234567890abcdef"}],errors:[],has_comment:false,has_message:false,has_age:false,has_url:false});`
	yesURLScript := `renderResultTable({items:[{kind:"TODO",author:"Alice",email:"alice@example.com",date:"2024-01-01",file:"main.go",line:7,commit:"1234567890abcdef",url:"https://example.com/blob"}],errors:[],has_comment:false,has_message:false,has_age:false,has_url:true});`
	noVal, err := rt.RunString(noURLScript)
	if err != nil {
		t.Fatalf("renderResultTable without URL failed: %v", err)
	}
	yesVal, err := rt.RunString(yesURLScript)
	if err != nil {
		t.Fatalf("renderResultTable with URL failed: %v", err)
	}
	noHTML := noVal.String()
	yesHTML := yesVal.String()
	if strings.Contains(noHTML, "data-key=\"url\"") || strings.Contains(noHTML, "link-icon") {
		t.Fatalf("URL 列は has_url=false では表示されない想定です: %s", noHTML)
	}
	if !strings.Contains(yesHTML, "data-key=\"url\"") {
		t.Fatalf("has_url=true で URL ヘッダーが欠けています: %s", yesHTML)
	}
	if !strings.Contains(yesHTML, ">URL</button>") {
		t.Fatalf("URL ヘッダーのボタンラベルが欠落しています: %s", yesHTML)
	}
	if !strings.Contains(yesHTML, "aria-label=\"GitHub で開く\"") {
		t.Fatalf("アクセシブルラベルが不足しています: %s", yesHTML)
	}
}

func TestWebRenderIncludesPRColumn(t *testing.T) {
	rt := goja.New()
	for _, fn := range []string{"escText", "escAttr", "renderBadge", "collapseWhitespace", "ellipsize", "normalizePRTooltip", "renderPRCell", "buildHeaderMeta", "renderTableCell", "renderResultTable"} {
		if _, err := rt.RunString(extractJSFunction(t, fn)); err != nil {
			t.Fatalf("failed to load %s: %v", fn, err)
		}
	}
	script := `renderResultTable({items:[{kind:"TODO",author:"Bob",email:"bob@example.com",date:"2024-01-02",file:"main.go",line:8,commit:"abcdef1234567890",prs:[{number:12,state:"open",url:"https://example.com/pull/12",title:"Example PR",body:"This PR fixes bug\\nand adds tests."}]}],errors:[],has_comment:false,has_message:false,has_age:false,has_url:false,has_prs:true});`
	value, err := rt.RunString(script)
	if err != nil {
		t.Fatalf("renderResultTable with PRs failed: %v", err)
	}
	html := value.String()
	if !strings.Contains(html, "data-key=\"prs\"") {
		t.Fatalf("PRS header missing: %s", html)
	}
	if !strings.Contains(html, "href=\"https://example.com/pull/12\"") {
		t.Fatalf("PR link missing: %s", html)
	}
	if !strings.Contains(html, "#12</a> Example PR (open)") {
		t.Fatalf("PR summary missing: %s", html)
	}
	normalized := strings.Join(strings.Fields(strings.NewReplacer("\r", " ", "\n", " ", "\\n", " ", "\\r", " ").Replace(html)), " ")
	if !strings.Contains(normalized, "title=\"This PR fixes bug and adds tests.\"") {
		t.Fatalf("PR tooltip missing: %s", html)
	}
}

func TestWebRenderAppliesAriaSortAttributes(t *testing.T) {
	rt := goja.New()
	for _, fn := range []string{"escText", "escAttr", "renderBadge", "collapseWhitespace", "ellipsize", "normalizePRTooltip", "renderPRCell", "buildHeaderMeta", "renderTableCell", "renderResultTable"} {
		if _, err := rt.RunString(extractJSFunction(t, fn)); err != nil {
			t.Fatalf("failed to load %s: %v", fn, err)
		}
	}

	dataLiteral := `{items:[{kind:"TODO",author:"Alice",email:"alice@example.com",date:"2024-01-01",file:"main.go",line:7,commit:"1234567890abcdef"}],errors:[],has_comment:false,has_message:false,has_age:false,has_url:false,has_prs:false}`

	noSortScript := `(()=>{const data=` + dataLiteral + `;return renderResultTable(data,{rows:data.items});})()`
	noSortVal, err := rt.RunString(noSortScript)
	if err != nil {
		t.Fatalf("renderResultTable without sort failed: %v", err)
	}
	noSortHTML := noSortVal.String()
	if strings.Contains(noSortHTML, "aria-sort=\"ascending\"") || strings.Contains(noSortHTML, "aria-sort=\"descending\"") {
		t.Fatalf("unsorted table should not mark ascending/descending: %s", noSortHTML)
	}
	if !strings.Contains(noSortHTML, "<th aria-sort=\"none\"><button type=\"button\" class=\"sort-btn\" data-key=\"kind\">TYPE</button></th>") {
		t.Fatalf("aria-sort=none missing for unsorted headers: %s", noSortHTML)
	}

	sortedScript := `(()=>{const data=` + dataLiteral + `;return renderResultTable(data,{rows:data.items,sortKey:"author",sortDesc:false});})()`
	sortedVal, err := rt.RunString(sortedScript)
	if err != nil {
		t.Fatalf("renderResultTable with sort failed: %v", err)
	}
	sortedHTML := sortedVal.String()
	if !strings.Contains(sortedHTML, "<th aria-sort=\"ascending\"><button type=\"button\" class=\"sort-btn asc\" data-key=\"author\">AUTHOR</button></th>") {
		t.Fatalf("sorted column should expose aria-sort=ascending: %s", sortedHTML)
	}
	if !strings.Contains(sortedHTML, "<th aria-sort=\"none\"><button type=\"button\" class=\"sort-btn\" data-key=\"kind\">TYPE</button></th>") {
		t.Fatalf("non-active headers should retain aria-sort=none: %s", sortedHTML)
	}
}

func TestWebSortKeepsEmptyValuesLastDescending(t *testing.T) {
	rt := goja.New()
	for _, fn := range []string{"compareStrings", "compareNumbers", "compareLocation", "comparePRs", "compareRows", "isValueEmptyForKey", "sortRows"} {
		if _, err := rt.RunString(extractJSFunction(t, fn)); err != nil {
			t.Fatalf("failed to load %s: %v", fn, err)
		}
	}

	authorScript := `(()=>{
const rows=[{author:"Bob"},{author:""},{},{author:"Carol"}];
const sorted=sortRows(rows,"author",true);
return sorted.map(r=>{const v=r&&r.author;return v?String(v):(v===""?"(empty)":"(none)");}).join("|");
})()`
	authorVal, err := rt.RunString(authorScript)
	if err != nil {
		t.Fatalf("author sort script failed: %v", err)
	}
	authorParts := strings.Split(authorVal.String(), "|")
	if len(authorParts) != 4 {
		t.Fatalf("unexpected author sort result length: %v", authorParts)
	}
	if authorParts[0] != "Carol" || authorParts[1] != "Bob" {
		t.Fatalf("descending author sort should rank non-empty values first, got: %v", authorParts)
	}
	if (authorParts[2] != "(empty)" && authorParts[2] != "(none)") || (authorParts[3] != "(empty)" && authorParts[3] != "(none)") {
		t.Fatalf("empty author values should appear at the end, got: %v", authorParts)
	}
	if authorParts[2] == authorParts[3] {
		// require that both sentinel categories are represented
		t.Fatalf("expected both empty and none markers, got: %v", authorParts)
	}

	ageScript := `(()=>{
const rows=[{age_days:10},{},{age_days:5},{age_days:null}];
const sorted=sortRows(rows,"age_days",true);
return sorted.map(r=>{const v=r&&r.age_days;return (typeof v==="number" && isFinite(v))?String(v):"(empty)";}).join("|");
})()`
	ageVal, err := rt.RunString(ageScript)
	if err != nil {
		t.Fatalf("age sort script failed: %v", err)
	}
	ageParts := strings.Split(ageVal.String(), "|")
	if len(ageParts) != 4 {
		t.Fatalf("unexpected age sort result length: %v", ageParts)
	}
	if ageParts[0] != "10" || ageParts[1] != "5" {
		t.Fatalf("descending age sort should keep numeric values first, got: %v", ageParts)
	}
	if ageParts[2] != "(empty)" || ageParts[3] != "(empty)" {
		t.Fatalf("numeric empties should be grouped at the end, got: %v", ageParts)
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
	idx := strings.Index(webTemplateHTML, marker)
	if idx < 0 {
		t.Fatalf("function %s not found", name)
	}
	rest := webTemplateHTML[idx:]
	end := len(rest)
	candidates := []string{"\n  function ", "\nfunction ", "\n  </script>", "\n</script>"}
	for _, c := range candidates {
		if next := strings.Index(rest, c); next >= 0 && next < end {
			end = next
		}
	}
	if end == len(rest) {
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
