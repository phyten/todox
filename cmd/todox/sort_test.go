package main

import (
	"testing"

	"github.com/example/todox/internal/engine"
)

func TestApplySortAge降順に並ぶ(t *testing.T) {
	items := []engine.Item{
		{File: "b.go", Line: 10, AgeDays: 3},
		{File: "a.go", Line: 5, AgeDays: 3},
		{File: "c.go", Line: 1, AgeDays: 7},
		{File: "a.go", Line: 2, AgeDays: 7},
	}

	if err := applySort(items, "-age"); err != nil {
		t.Fatalf("applySort returned error: %v", err)
	}

	want := []engine.Item{
		{File: "a.go", Line: 2, AgeDays: 7},
		{File: "c.go", Line: 1, AgeDays: 7},
		{File: "a.go", Line: 5, AgeDays: 3},
		{File: "b.go", Line: 10, AgeDays: 3},
	}

	for i := range want {
		if items[i].File != want[i].File || items[i].Line != want[i].Line || items[i].AgeDays != want[i].AgeDays {
			t.Fatalf("unexpected order at %d: got=%v want=%v", i, items[i], want[i])
		}
	}
}

func TestApplySortUnknownKeyはエラー(t *testing.T) {
	items := make([]engine.Item, 0)
	if err := applySort(items, "date"); err == nil {
		t.Fatal("unsupported key should return error")
	}
}
