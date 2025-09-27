package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

	author, email, date, subject, err := commitMeta(ctx, repo, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatalf("エラーが返される想定でした")
	}
	if author != "-" || email != "-" || date != "-" || subject != "-" {
		t.Fatalf("エラー時のプレースホルダーが想定外です: %q %q %q %q", author, email, date, subject)
	}
	if !strings.Contains(err.Error(), "git show") {
		t.Fatalf("エラーメッセージにコマンド名が含まれていません: %v", err)
	}
}

func TestKindOfテキスト種別を判定できる(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  string
	}{
		"TODOのみ":   {input: "// TODO: write tests", want: "TODO"},
		"FIXMEのみ":  {input: "# FIXME: fix bug", want: "FIXME"},
		"両方含む":     {input: "// TODO and FIXME", want: "TODO|FIXME"},
		"どちらも含まない": {input: "// nothing", want: "UNKNOWN"},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := kindOf(tt.input); got != tt.want {
				t.Fatalf("種別が一致しません: got=%s want=%s", got, tt.want)
			}
		})
	}
}

func TestExtractComment指定タイプで抽出できる(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		typ  string
		want string
	}{
		{name: "TODO指定", text: "prefix TODO: something", typ: "todo", want: "TODO: something"},
		{name: "FIXME指定", text: "prefix FIXME: fix", typ: "fixme", want: "FIXME: fix"},
		{name: "両方で先に出た方", text: "NOTE FIXME before TODO", typ: "", want: "FIXME before TODO"},
		{name: "対象がない場合は原文", text: "no markers", typ: "todo", want: "no markers"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extractComment(tt.text, tt.typ); got != tt.want {
				t.Fatalf("コメント抽出結果が一致しません: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestTruncateRunesマルチバイト文字も考慮する(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{name: "制限なし", input: "あいうえお", n: 0, want: "あいうえお"},
		{name: "十分長い", input: "hello", n: 10, want: "hello"},
		{name: "途中で切り詰め", input: "あいうえお", n: 3, want: "あい…"},
		{name: "最小値", input: "example", n: 1, want: "…"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := truncateRunes(tt.input, tt.n); got != tt.want {
				t.Fatalf("切り詰め結果が一致しません: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestEffectiveTrunc優先度(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		specific int
		all      int
		want     int
	}{
		{name: "個別指定を優先", specific: 5, all: 20, want: 5},
		{name: "個別指定が0なら全体", specific: 0, all: 15, want: 15},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := effectiveTrunc(tt.specific, tt.all); got != tt.want {
				t.Fatalf("切り詰め長が一致しません: got=%d want=%d", got, tt.want)
			}
		})
	}
}
