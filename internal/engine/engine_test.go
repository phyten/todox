package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBlameSHAコマンド引数(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	fakeBin := t.TempDir()
	argsDir := t.TempDir()

	setEnv := func(t *testing.T, key, value string) {
		t.Helper()
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("環境変数%sの設定に失敗しました: %v", key, err)
		}
	}

	unsetEnv := func(t *testing.T, key string) {
		t.Helper()
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("環境変数%sの解除に失敗しました: %v", key, err)
		}
	}

	scriptPath := filepath.Join(fakeBin, "git")
	script := "#!/bin/sh\n" +
		"if [ -z \"$ENGINE_FAKE_GIT_ARGS\" ]; then\n" +
		"  echo 'ENGINE_FAKE_GIT_ARGS not set' >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"printf '%s\\n' \"$@\" > \"$ENGINE_FAKE_GIT_ARGS\"\n" +
		"printf 'deadbeefdeadbeefdeadbeefdeadbeefdeadbeef 1 1 1\\n'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("フェイクgitの作成に失敗しました: %v", err)
	}

	oldPath := os.Getenv("PATH")
	setEnv(t, "PATH", fakeBin+string(os.PathListSeparator)+oldPath)
	t.Cleanup(func() { setEnv(t, "PATH", oldPath) })

	t.Cleanup(func() { unsetEnv(t, "ENGINE_FAKE_GIT_ARGS") })

	call := func(t *testing.T, ignoreWS bool) []string {
		t.Helper()
		argsFile := filepath.Join(argsDir, "args-"+map[bool]string{false: "no", true: "ws"}[ignoreWS]+".txt")
		setEnv(t, "ENGINE_FAKE_GIT_ARGS", argsFile)

		sha, err := blameSHA(ctx, repo, "dummy.txt", 12, ignoreWS)
		if err != nil {
			t.Fatalf("blameSHAの実行に失敗しました: %v", err)
		}
		const wantSHA = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
		if sha != wantSHA {
			t.Fatalf("SHAが想定外です: got=%s want=%s", sha, wantSHA)
		}

		data, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("引数記録ファイルの読み込みに失敗しました: %v", err)
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			t.Fatalf("引数が記録されていません")
		}
		return strings.Split(content, "\n")
	}

	t.Run("空白を無視しない場合", func(t *testing.T) {
		got := call(t, false)
		want := []string{"blame", "--line-porcelain", "-L", "12,12", "--", "dummy.txt"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("引数が期待と異なります: got=%v want=%v", got, want)
		}
	})

	t.Run("空白を無視する場合", func(t *testing.T) {
		got := call(t, true)
		want := []string{"blame", "-w", "--line-porcelain", "-L", "12,12", "--", "dummy.txt"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("引数が期待と異なります: got=%v want=%v", got, want)
		}
	})
}

func TestCommitMetaエラー時はプレースホルダーとエラーを返す(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()

	author, email, date, ts, subject, err := commitMeta(ctx, repo, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatalf("エラーが返される想定でした")
	}
	if author != "-" || email != "-" || date != "-" || subject != "-" {
		t.Fatalf("エラー時のプレースホルダーが想定外です: %q %q %q %q", author, email, date, subject)
	}
	if !ts.IsZero() {
		t.Fatalf("エラー時のタイムスタンプはゼロ値の想定です: %v", ts)
	}
	if !strings.Contains(err.Error(), "git show") {
		t.Fatalf("エラーメッセージにコマンド名が含まれていません: %v", err)
	}
}

func TestKindOf判定パターン(t *testing.T) {
	cases := map[string]string{
		"これは TODO のテスト":        "TODO",
		"FIXME のみを含む":          "FIXME",
		"両方 TODO と FIXME を含む":  "TODO|FIXME",
		"どちらも含まない":             "UNKNOWN",
		"小文字todoは対象外 FIXMEは検出": "FIXME",
		"TODO と FIXME が同じ行にある": "TODO|FIXME",
	}
	for input, want := range cases {
		if got := kindOf(input); got != want {
			t.Fatalf("kindOf(%q) の結果が想定外です: got=%q want=%q", input, got, want)
		}
	}
}

func TestExtractCommentタイプごとに抽出(t *testing.T) {
	const text = "prefix TODO something FIXME more"
	cases := []struct {
		name string
		typ  string
		want string
	}{
		{name: "TODO優先", typ: "todo", want: "TODO something FIXME more"},
		{name: "FIXME優先", typ: "fixme", want: "FIXME more"},
		{name: "両方デフォルトTODO", typ: "both", want: "TODO something FIXME more"},
		{name: "不明タイプはTODO", typ: "", want: "TODO something FIXME more"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractComment(text, tc.typ); got != tc.want {
				t.Fatalf("extractComment(%q, %q) = %q, want %q", text, tc.typ, got, tc.want)
			}
		})
	}

	const none = "コメント対象なし"
	if got := extractComment(none, "todo"); got != none {
		t.Fatalf("TODOが存在しない場合は元の文字列を返すべきです: got=%q want=%q", got, none)
	}
}

func TestTruncateRunes多バイト文字と省略記号(t *testing.T) {
	input := "あいうえお"
	if got := truncateRunes(input, 0); got != input {
		t.Fatalf("0指定の場合は元の文字列を返すべきです: got=%q want=%q", got, input)
	}
	if got := truncateRunes(input, 3); got != "あい…" {
		t.Fatalf("多バイト文字の切り詰めが期待と異なります: got=%q want=%q", got, "あい…")
	}
	if got := truncateRunes("abc", 1); got != "…" {
		t.Fatalf("1文字指定時は省略記号のみの想定です: got=%q", got)
	}
}

func TestEffectiveTrunc優先順位(t *testing.T) {
	cases := []struct {
		specific int
		all      int
		want     int
	}{
		{specific: 80, all: 20, want: 80},
		{specific: 0, all: 50, want: 50},
		{specific: 0, all: 0, want: 0},
	}
	for _, tc := range cases {
		if got := effectiveTrunc(tc.specific, tc.all); got != tc.want {
			t.Fatalf("effectiveTrunc(%d, %d) = %d, want %d", tc.specific, tc.all, got, tc.want)
		}
	}
}

func TestAgeDays計算(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	tenDaysAgo := now.AddDate(0, 0, -10)
	if got := ageDays(now, tenDaysAgo); got != 10 {
		t.Fatalf("10日前との差分が期待と異なります: got=%d", got)
	}

	future := now.Add(12 * time.Hour)
	if got := ageDays(now, future); got != 0 {
		t.Fatalf("未来日時は0日にクリップされる想定です: got=%d", got)
	}

	if got := ageDays(now, time.Time{}); got != 0 {
		t.Fatalf("ゼロ値の日時は0を返す想定です: got=%d", got)
	}
}

func TestNewItemErrorメッセージ整形(t *testing.T) {
	err := errors.New("  failure happened  ")
	item := newItemError("file.go", 10, "stage", err)
	if item.Message != "failure happened" {
		t.Fatalf("前後の空白を除去すべきです: got=%q", item.Message)
	}

	emptyErr := errors.New("")
	fallback := newItemError("file.go", 20, "stage", emptyErr)
	if fallback.Message != "unknown error" {
		t.Fatalf("空メッセージの場合は既定文言を利用すべきです: got=%q", fallback.Message)
	}
}

func TestGitGrepHandlesLongLines(t *testing.T) {
	repoDir := t.TempDir()

	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.name", "tester")
	runGit(t, repoDir, "config", "user.email", "tester@example.com")

	const testLongLineSize = 210_000
	longTail := strings.Repeat("A", testLongLineSize)
	content := "// TODO " + longTail + "\n"
	if err := os.WriteFile(filepath.Join(repoDir, "long.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	runGit(t, repoDir, "add", "long.go")
	runGit(t, repoDir, "commit", "-m", "add long line")

	matches, err := gitGrep(repoDir, "TODO")
	if err != nil {
		t.Fatalf("gitGrep returned error: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	got := matches[0]
	if got.file != "long.go" {
		t.Fatalf("unexpected file: %s", got.file)
	}

	if got.line != 1 {
		t.Fatalf("unexpected line: %d", got.line)
	}

	const todoPrefix = "// TODO "
	if !strings.HasPrefix(got.text, todoPrefix) {
		prefixLen := len(todoPrefix)
		var gotPrefix string
		if len(got.text) >= prefixLen {
			gotPrefix = got.text[:prefixLen]
		} else {
			gotPrefix = got.text
		}
		t.Fatalf("match text missing prefix: %q", gotPrefix)
	}

	if len(got.text) != len(strings.TrimSuffix(content, "\n")) {
		t.Fatalf("unexpected text length: got %d want %d", len(got.text), len(content)-1)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
}
