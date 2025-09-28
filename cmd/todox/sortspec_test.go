package main

import "testing"

func TestParseSortSpecNormalizesKeys(t *testing.T) {
	spec, err := ParseSortSpec("author,-date,location,age_days")
	if err != nil {
		t.Fatalf("ParseSortSpec failed: %v", err)
	}
	want := []SortKey{
		{Name: "author", Desc: false},
		{Name: "age", Desc: false},
		{Name: "file", Desc: false},
		{Name: "line", Desc: false},
		{Name: "age", Desc: false},
	}
	if len(spec.Keys) != len(want) {
		t.Fatalf("unexpected key count: got=%v want=%v", spec.Keys, want)
	}
	for i, got := range spec.Keys {
		if got != want[i] {
			t.Fatalf("key %d mismatch: got=%+v want=%+v", i, got, want[i])
		}
	}
}

func TestParseSortSpecUnknownKey(t *testing.T) {
	if _, err := ParseSortSpec("unknown"); err == nil {
		t.Fatal("expected error for unknown sort key")
	}
}

func TestParseSortSpecEmptyEntry(t *testing.T) {
	if _, err := ParseSortSpec("author,,file"); err == nil {
		t.Fatal("expected error for empty sort key")
	}
}
