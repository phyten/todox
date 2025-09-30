package progress

import (
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"
)

type Observer interface {
	Publish(Snapshot)
	Done(Snapshot)
}

type NoopObserver struct{}

func (NoopObserver) Publish(Snapshot) {}
func (NoopObserver) Done(Snapshot)    {}

type ObserverFunc func(Snapshot)

func (f ObserverFunc) Publish(s Snapshot) { f(s) }
func (ObserverFunc) Done(Snapshot)        {}

type MultiObserver struct {
	observers []Observer
}

func NewMultiObserver(obs ...Observer) Observer {
	filtered := make([]Observer, 0, len(obs))
	for _, ob := range obs {
		if ob == nil {
			continue
		}
		filtered = append(filtered, ob)
	}
	if len(filtered) == 0 {
		return NoopObserver{}
	}
	return &MultiObserver{observers: filtered}
}

func (m *MultiObserver) Publish(s Snapshot) {
	for _, ob := range m.observers {
		ob.Publish(s)
	}
}

func (m *MultiObserver) Done(s Snapshot) {
	for _, ob := range m.observers {
		ob.Done(s)
	}
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

type ttyObserver struct {
	w  io.Writer
	mu sync.Mutex
}

type lineObserver struct {
	w  io.Writer
	mu sync.Mutex
}

func NewTTYObserver(w io.Writer) Observer {
	if w == nil {
		w = os.Stderr
	}
	return &ttyObserver{w: w}
}

func NewLineObserver(w io.Writer) Observer {
	if w == nil {
		w = os.Stderr
	}
	return &lineObserver{w: w}
}

func NewAutoObserver(w io.Writer) Observer {
	if w == nil {
		w = os.Stderr
	}
	if f, ok := w.(*os.File); ok && isTTY(f) {
		return NewTTYObserver(w)
	}
	return NewLineObserver(w)
}

func (o *ttyObserver) Publish(s Snapshot) {
	o.mu.Lock()
	defer o.mu.Unlock()
	line := renderTTY(s)
	_, _ = fmt.Fprintf(o.w, "\r\033[K%s", line)
}

func (o *ttyObserver) Done(Snapshot) {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, _ = fmt.Fprint(o.w, "\r\033[K")
}

func (o *lineObserver) Publish(s Snapshot) {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, _ = fmt.Fprintln(o.w, renderLine(s))
}

func (o *lineObserver) Done(Snapshot) {}

func renderTTY(s Snapshot) string {
	pct := percent(s.Done, s.Total)
	rate := "--/s"
	if !s.Warmup && s.RateEMA > 0 {
		rate = fmt.Sprintf("%.1f/s", s.RateEMA)
	}
	eta := "--:--"
	if !s.Warmup && s.ETAP50 > 0 {
		eta = formatETA(s.ETAP50)
	}
	p90 := ""
	if !s.Warmup && s.ETAP90 > 0 {
		p90 = fmt.Sprintf(" (P90 %s)", formatETA(s.ETAP90))
	}
	return fmt.Sprintf("[progress] %3d%% %d/%d %s ETA %s%s", pct, s.Done, s.Total, rate, eta, p90)
}

func renderLine(s Snapshot) string {
	eta50 := secondsOrNegOne(s.ETAP50)
	eta90 := secondsOrNegOne(s.ETAP90)
	return fmt.Sprintf("progress stage=%s total=%d done=%d rate=%.3f eta_p50=%g eta_p90=%g warmup=%t updated_at=%s", string(s.Stage), s.Total, s.Done, s.RateEMA, eta50, eta90, s.Warmup, s.UpdatedAt.Format(time.RFC3339Nano))
}

func formatETA(d time.Duration) string {
	totalSeconds := int(math.Round(d.Seconds()))
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 99 {
		hours = 99
	}
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func secondsOrNegOne(d time.Duration) float64 {
	if d <= 0 {
		return -1
	}
	return d.Seconds()
}

func percent(a, b int) int {
	if b <= 0 {
		if a <= 0 {
			return 0
		}
		return 100
	}
	if a <= 0 {
		return 0
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

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
