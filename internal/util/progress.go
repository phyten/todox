package util

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

func isTTY(f *os.File) bool {
	fi, _ := f.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ShouldShowProgress は進捗表示を有効化すべきかを判定します。
// force が true の場合は常に true を返し、no が true の場合は常に false を返します。
// どちらでもない場合は標準出力・標準エラーが TTY かどうかを基準に判定します。
func ShouldShowProgress(force, no bool) bool {
	if no {
		return false
	}
	if force {
		return true
	}
	return isTTY(os.Stdout) && isTTY(os.Stderr)
}

// Progress は処理件数と経過時間から進捗を計算し、標準エラーに表示するための構造体です。
type Progress struct {
	total   int
	start   time.Time
	enabled bool
	done    atomic.Int64
}

// NewProgress は総件数と有効状態を受け取り、新しい Progress を生成します。
func NewProgress(total int, enabled bool) *Progress {
	return &Progress{total: total, start: time.Now(), enabled: enabled}
}

// Advance は処理済み件数を 1 件進め、最新の件数を返します。
// 進捗表示が有効な場合は表示を更新します。
func (p *Progress) Advance() int {
	done := int(p.done.Add(1))
	p.Update(done)
	return done
}

// Update は処理済み件数を明示的に指定して進捗表示を更新します。
// マイナスや総件数超過の値が渡された場合でも適切な範囲に丸めます。
func (p *Progress) Update(done int) {
	if !p.enabled {
		return
	}
	if done < 0 {
		done = 0
	}
	if done > p.total {
		done = p.total
	}
	elapsed := time.Since(p.start)
	eta := "-"
	if done > 0 && elapsed > 0 {
		remaining := p.total - done
		if remaining < 0 {
			remaining = 0
		}
		if remaining > 0 {
			avgPerItem := elapsed / time.Duration(done)
			remain := avgPerItem * time.Duration(remaining)
			eta = fmt.Sprintf("%02d:%02d:%02d", int(remain.Hours()), int(remain.Minutes())%60, int(remain.Seconds())%60)
		}
	}
	// clear line and print
	fmt.Fprintf(os.Stderr, "\r\033[K[progress] %d/%d (%d%%) ETA %s",
		done, p.total, percent(done, p.total), eta)
}

// Done は進捗表示を終了し、描画に使用した行を消去します。
func (p *Progress) Done() {
	if !p.enabled {
		return
	}
	fmt.Fprint(os.Stderr, "\r\033[K")
}

func percent(a, b int) int {
	if b == 0 {
		return 100
	}
	p := int(float64(a) * 100 / float64(b))
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
