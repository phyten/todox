package progress

import (
	"math"
	"sync"
	"time"
)

type Stage string

const (
	StageScan Stage = "scan"
	StageAttr Stage = "attr"
)

type Snapshot struct {
	Stage     Stage         `json:"stage"`
	Total     int           `json:"total"`
	Done      int           `json:"done"`
	Remaining int           `json:"remaining"`
	RateEMA   float64       `json:"rate_per_sec"`
	RateP50   float64       `json:"rate_p50"`
	RateP10   float64       `json:"rate_p10"`
	ETAP50    time.Duration `json:"eta_p50"`
	ETAP90    time.Duration `json:"eta_p90"`
	Warmup    bool          `json:"warmup"`
	StartedAt time.Time     `json:"started_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Elapsed   time.Duration `json:"elapsed"`
}

type Config struct {
	Alpha          float64
	WindowSize     int
	WarmupSamples  int
	WarmupDuration time.Duration
	NotifyInterval time.Duration
	SlowFallback   float64
}

type stageState struct {
	ema    float64
	window *window
}

type Estimator struct {
	mu         sync.Mutex
	cfg        Config
	start      time.Time
	lastUpdate time.Time
	lastNotify time.Time
	stage      Stage
	total      int
	done       int
	stageData  map[Stage]*stageState
}

func DefaultConfig() Config {
	return Config{
		Alpha:          0.2,
		WindowSize:     60,
		WarmupSamples:  20,
		WarmupDuration: 2 * time.Second,
		NotifyInterval: 250 * time.Millisecond,
		SlowFallback:   0.6,
	}
}

func NewEstimator(total int, cfg Config) *Estimator {
	base := DefaultConfig()
	if cfg.Alpha > 0 {
		base.Alpha = cfg.Alpha
	}
	if cfg.WindowSize > 0 {
		base.WindowSize = cfg.WindowSize
	}
	if cfg.WarmupSamples > 0 {
		base.WarmupSamples = cfg.WarmupSamples
	}
	if cfg.WarmupDuration > 0 {
		base.WarmupDuration = cfg.WarmupDuration
	}
	if cfg.NotifyInterval > 0 {
		base.NotifyInterval = cfg.NotifyInterval
	}
	if cfg.SlowFallback > 0 {
		base.SlowFallback = cfg.SlowFallback
	}
	now := time.Now()
	return &Estimator{
		cfg:        base,
		start:      now,
		lastUpdate: now,
		stage:      StageScan,
		total:      total,
		stageData:  make(map[Stage]*stageState),
	}
}

func (e *Estimator) SetTotal(total int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.total = total
}

func (e *Estimator) Stage(stage Stage) (Snapshot, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if stage == e.stage {
		return e.snapshotLocked(time.Now()), false
	}
	e.stage = stage
	now := time.Now()
	e.lastNotify = now
	snap := e.snapshotLocked(now)
	return snap, true
}

func (e *Estimator) Advance(delta int) (Snapshot, bool) {
	if delta <= 0 {
		return e.Snapshot(), false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	if now.Before(e.lastUpdate) {
		now = e.lastUpdate
	}
	dt := now.Sub(e.lastUpdate).Seconds()
	if dt <= 0 {
		dt = 1e-6
	}
	e.done += delta
	state := e.stateLocked(e.stage)
	instant := float64(delta) / dt
	if math.IsNaN(instant) || math.IsInf(instant, 0) || instant < 0 {
		instant = 0
	}
	if state.ema == 0 {
		state.ema = instant
	} else {
		state.ema = e.cfg.Alpha*instant + (1-e.cfg.Alpha)*state.ema
	}
	state.window.Add(instant)
	e.lastUpdate = now
	snap := e.snapshotLocked(now)
	notify := now.Sub(e.lastNotify) >= e.cfg.NotifyInterval || snap.Remaining == 0
	if notify {
		e.lastNotify = now
	}
	return snap, notify
}

func (e *Estimator) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshotLocked(time.Now())
}

func (e *Estimator) Complete() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	if e.done < e.total && e.total >= 0 {
		e.done = e.total
	}
	snap := e.snapshotLocked(now)
	e.lastNotify = now
	return snap
}

func (e *Estimator) stateLocked(stage Stage) *stageState {
	st, ok := e.stageData[stage]
	if !ok {
		st = &stageState{window: newWindow(e.cfg.WindowSize)}
		e.stageData[stage] = st
	}
	return st
}

func (e *Estimator) remainingLocked() int {
	if e.total < 0 {
		return -1
	}
	remaining := e.total - e.done
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}

func (e *Estimator) snapshotLocked(now time.Time) Snapshot {
	if now.IsZero() {
		now = time.Now()
	}
	state := e.stateLocked(e.stage)
	remain := e.remainingLocked()
	elapsed := now.Sub(e.start)
	warmReady := e.done >= e.cfg.WarmupSamples && elapsed >= e.cfg.WarmupDuration
	rateP50 := state.window.Quantile(0.50)
	rateP10 := state.window.Quantile(0.10)
	if rateP50 <= 0 {
		rateP50 = state.ema
	}
	if rateP10 <= 0 {
		rateP10 = rateP50 * e.cfg.SlowFallback
	}
	var eta50, eta90 time.Duration
	if warmReady && remain > 0 {
		eta50 = durationFrom(float64(remain), rateP50)
		eta90 = durationFrom(float64(remain), rateP10)
	}
	return Snapshot{
		Stage:     e.stage,
		Total:     e.total,
		Done:      e.done,
		Remaining: remain,
		RateEMA:   state.ema,
		RateP50:   rateP50,
		RateP10:   rateP10,
		ETAP50:    eta50,
		ETAP90:    eta90,
		Warmup:    !warmReady,
		StartedAt: e.start,
		UpdatedAt: now,
		Elapsed:   elapsed,
	}
}

func durationFrom(count float64, rate float64) time.Duration {
	if rate <= 0 {
		return 0
	}
	seconds := count / rate
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0
	}
	if seconds < 0 {
		seconds = 0
	}
	if seconds > float64((1<<63-1)/int(time.Second)) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(seconds * float64(time.Second))
}
