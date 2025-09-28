package main

import (
	"testing"

	"github.com/example/todox/internal/engine"
)

func TestParseSortSpec複合キーを展開する(t *testing.T) {
	spec, err := ParseSortSpec("author,-date,location")
	if err != nil {
		t.Fatalf("ParseSortSpec に失敗しました: %v", err)
	}
	want := []SortKey{{Name: "author", Desc: false}, {Name: "age", Desc: false}, {Name: "file", Desc: false}, {Name: "line", Desc: false}}
	if len(spec.Keys) != len(want) {
		t.Fatalf("キー数が一致しません: got=%d want=%d", len(spec.Keys), len(want))
	}
	for i, k := range spec.Keys {
		if k != want[i] {
			t.Fatalf("キーが一致しません: index=%d got=%+v want=%+v", i, k, want[i])
		}
	}
}

func TestApplySortはlocationを考慮する(t *testing.T) {
	items := []engine.Item{{File: "b.go", Line: 20}, {File: "a.go", Line: 30}, {File: "a.go", Line: 10}}
	spec, err := ParseSortSpec("location")
	if err != nil {
		t.Fatalf("ParseSortSpec に失敗しました: %v", err)
	}
	ApplySort(items, spec)
	if items[0].File != "a.go" || items[0].Line != 10 {
		t.Fatalf("location ソートが期待通りではありません: %+v", items)
	}
	if items[1].File != "a.go" || items[1].Line != 30 {
		t.Fatalf("2番目の項目が期待通りではありません: %+v", items)
	}
}

func TestResolveFields既定値(t *testing.T) {
	sel, err := ResolveFields("", true, false, true)
	if err != nil {
		t.Fatalf("ResolveFields に失敗しました: %v", err)
	}
	if !sel.HasComment || sel.HasMessage || !sel.HasAge {
		t.Fatalf("既定のフラグ解釈が期待通りではありません: %+v", sel)
	}
	gotHeaders := make([]string, len(sel.Fields))
	for i, f := range sel.Fields {
		gotHeaders[i] = f.Header
	}
	want := []string{"TYPE", "AUTHOR", "EMAIL", "DATE", "AGE", "COMMIT", "LOCATION", "COMMENT"}
	if len(gotHeaders) != len(want) {
		t.Fatalf("ヘッダー数が一致しません: got=%v want=%v", gotHeaders, want)
	}
	for i := range want {
		if gotHeaders[i] != want[i] {
			t.Fatalf("ヘッダー順が一致しません: index=%d got=%s want=%s", i, gotHeaders[i], want[i])
		}
	}
}

func TestResolveFields任意指定(t *testing.T) {
	sel, err := ResolveFields("type,author,date,age,location", false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields に失敗しました: %v", err)
	}
	if !sel.HasAge {
		t.Fatal("age を指定した場合は HasAge が真になるはずです")
	}
	if sel.HasComment || sel.HasMessage {
		t.Fatalf("comment/message を含まない場合は false のはずです: %+v", sel)
	}
	if len(sel.Fields) != 5 {
		t.Fatalf("列数が一致しません: %d", len(sel.Fields))
	}
	if sel.Fields[3].Name != "age" {
		t.Fatalf("age 列の位置が期待と異なります: %+v", sel.Fields)
	}
}

func TestResolveFields未知値はエラー(t *testing.T) {
	if _, err := ResolveFields("type,unknown", false, false, false); err == nil {
		t.Fatal("未知のフィールドでエラーを期待しました")
	}
}
