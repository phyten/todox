package util

import "testing"

func TestPercentは100を上限とする(t *testing.T) {
	if got := percent(5, 4); got != 100 {
		t.Fatalf("5/4 は 100%% として扱うべきです: got=%d", got)
	}
}
