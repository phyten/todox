package util

import (
	"fmt"
	"os"
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
}

func NewProgress(total int, enabled bool) *Progress {
	return &Progress{total: total, start: time.Now(), enabled: enabled}
}

func (p *Progress) Update(done int) {
	if !p.enabled {
		return
	}
	elapsed := time.Since(p.start)
	eta := "-"
	if done > 0 {
		remain := time.Duration(float64(elapsed) * float64(p.total-done) / float64(done))
		eta = fmt.Sprintf("%02d:%02d:%02d", int(remain.Hours()), int(remain.Minutes())%60, int(remain.Seconds())%60)
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
	return int(float64(a) * 100 / float64(b))
}
