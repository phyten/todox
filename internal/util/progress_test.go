package util

import (
	"sync"
	"testing"
)

func TestPercentは100を上限とする(t *testing.T) {
	if got := percent(5, 4); got != 100 {
		t.Fatalf("5/4 は 100%% として扱うべきです: got=%d", got)
	}
}

func TestProgressAdvanceは並列でも連番になる(t *testing.T) {
	const workers = 128

	prog := NewProgress(workers, false)

	var wg sync.WaitGroup
	wg.Add(workers)

	start := make(chan struct{})
	results := make(chan int, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			results <- prog.Advance()
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
