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

func ShouldShowProgress(force, no bool) bool {
	if no {
		return false
	}
	if force {
		return true
	}
	return isTTY(os.Stdout) && isTTY(os.Stderr)
}

type Progress struct {
	total   int
	start   time.Time
	enabled bool
	done    atomic.Int64
}

func NewProgress(total int, enabled bool) *Progress {
	return &Progress{total: total, start: time.Now(), enabled: enabled}
}

func (p *Progress) Advance() int {
	done := int(p.done.Add(1))
	p.Update(done)
	return done
}

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
			if avgPerItem <= 0 {
				avgPerItem = time.Nanosecond
			}
			remain := avgPerItem * time.Duration(remaining)
			eta = fmt.Sprintf("%02d:%02d:%02d", int(remain.Hours()), int(remain.Minutes())%60, int(remain.Seconds())%60)
		}
	}
	// clear line and print
	fmt.Fprintf(os.Stderr, "\r\033[K[progress] %d/%d (%d%%) ETA %s",
		done, p.total, percent(done, p.total), eta)
}

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
