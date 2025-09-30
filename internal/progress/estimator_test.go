package progress

import (
	"sync"
	"testing"
	"time"
)

func TestEstimatorAdvanceIsSequential(t *testing.T) {
	const workers = 128
	est := NewEstimator(workers, Config{NotifyInterval: time.Nanosecond})

	var wg sync.WaitGroup
	wg.Add(workers)

	start := make(chan struct{})
	results := make(chan int, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			snap, _ := est.Advance(1)
			results <- snap.Done
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	seen := make([]bool, workers)
	count := 0
	for r := range results {
		if r <= 0 || r > workers {
			t.Fatalf("進捗値が範囲外です: got=%d", r)
		}
		if seen[r-1] {
			t.Fatalf("進捗値が重複しました: got=%d", r)
		}
		seen[r-1] = true
		count++
	}

	if count != workers {
		t.Fatalf("進捗値の数が期待と一致しません: want=%d got=%d", workers, count)
	}

	for i, ok := range seen {
		if !ok {
			t.Fatalf("進捗値が欠落しています: index=%d", i+1)
		}
	}
}

func TestPercentClampsTo100(t *testing.T) {
	if got := percent(5, 4); got != 100 {
		t.Fatalf("5/4 は 100%% として扱うべきです: got=%d", got)
	}
}

func TestEstimatorETAOrder(t *testing.T) {
	est := NewEstimator(100, Config{
		WarmupSamples:  5,
		WarmupDuration: time.Millisecond,
		NotifyInterval: time.Nanosecond,
	})

	var snap Snapshot
	for i := 0; i < 20; i++ {
		time.Sleep(2 * time.Millisecond)
		snap, _ = est.Advance(1)
		if snap.Warmup {
			continue
		}
		if snap.ETAP50 > 0 && snap.ETAP90 > 0 && snap.ETAP90 < snap.ETAP50 {
			t.Fatalf("ETA の順序が逆転しています: p50=%v p90=%v", snap.ETAP50, snap.ETAP90)
		}
	}

	if snap.Warmup {
		t.Fatalf("ウォームアップが終了しませんでした: done=%d elapsed=%v", snap.Done, snap.Elapsed)
	}
	if snap.ETAP50 <= 0 {
		t.Fatalf("P50 ETA が正であるべきです: got=%v", snap.ETAP50)
	}
	if snap.ETAP90 < snap.ETAP50 {
		t.Fatalf("P90 ETA は P50 以上であるべきです: p50=%v p90=%v", snap.ETAP50, snap.ETAP90)
	}
}
