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
	os.Setenv("PATH", fakeBin+string(os.PathListSeparator)+oldPath)
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })

	t.Cleanup(func() { os.Unsetenv("ENGINE_FAKE_GIT_ARGS") })

	call := func(t *testing.T, ignoreWS bool) []string {
		t.Helper()
		argsFile := filepath.Join(argsDir, "args-"+map[bool]string{false: "no", true: "ws"}[ignoreWS]+".txt")
		os.Setenv("ENGINE_FAKE_GIT_ARGS", argsFile)

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

}
