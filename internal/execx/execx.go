package execx

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// Runner は外部コマンドを実行するための最小インターフェースです。
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) (stdout []byte, stderr []byte, err error)
}

// CommandRunner は exec.CommandContext を利用したデフォルト実装です。
type CommandRunner struct{}

// Run は指定された作業ディレクトリでコマンドを実行し、標準出力・標準エラーを収集します。
func (CommandRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// IsNotFound はコマンドが見つからない場合のエラーを判定します。
func IsNotFound(err error) bool {
	var execErr *exec.Error
	return errors.As(err, &execErr)
}

// DefaultRunner は CommandRunner を返します。
func DefaultRunner() Runner {
	return CommandRunner{}
}
