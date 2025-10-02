package main

import "testing"

func TestResolveFieldsDefaultUsesFlags(t *testing.T) {
	sel, err := ResolveFields("", true, false, true, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}
	headers := []string{"TYPE", "AUTHOR", "EMAIL", "DATE", "AGE", "COMMIT", "LOCATION", "COMMENT"}
	if len(sel.Fields) != len(headers) {
		t.Fatalf("field count mismatch: got=%d want=%d", len(sel.Fields), len(headers))
	}
	for i, f := range sel.Fields {
		if f.Header != headers[i] {
			t.Fatalf("header %d mismatch: got=%s want=%s", i, f.Header, headers[i])
		}
	}
	if !sel.ShowAge || !sel.ShowComment || sel.ShowMessage {
		t.Fatalf("show flags mismatch: %+v", sel)
	}
	if !sel.NeedComment || sel.NeedMessage {
		t.Fatalf("need flags mismatch: %+v", sel)
	}
}

func TestResolveFieldsOverridesFlags(t *testing.T) {
	sel, err := ResolveFields("type,author", true, true, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}
	if sel.ShowComment {
		t.Fatal("comment column should be disabled when fields override")
	}
	if !sel.NeedComment || !sel.NeedMessage {
		t.Fatalf("need flags should respect original requests: %+v", sel)
	}
	if len(sel.Fields) != 2 || sel.Fields[0].Key != "type" || sel.Fields[1].Key != "author" {
		t.Fatalf("fields mismatch: %+v", sel.Fields)
	}
}

func TestResolveFieldsEnablesMessageViaFields(t *testing.T) {
	sel, err := ResolveFields("type,message", false, false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}
	if !sel.ShowMessage || !sel.NeedMessage {
		t.Fatalf("message flags not set: %+v", sel)
	}
}

func TestResolveFieldsUnknownField(t *testing.T) {
	if _, err := ResolveFields("unknown", false, false, false, false); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestResolveFieldsIncludesURLColumn(t *testing.T) {
	sel, err := ResolveFields("", false, false, false, true)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}
	found := false
	for _, f := range sel.Fields {
		if f.Key == "url" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("URL column not included: %+v", sel.Fields)
	}
	if !sel.ShowURL || !sel.NeedURL {
		t.Fatalf("URL flags not set: %+v", sel)
	}
}

func TestResolveFieldsURLColumnRequestedViaFieldsOnly(t *testing.T) {
	sel, err := ResolveFields("url", false, false, false, false)
	if err != nil {
		t.Fatalf("ResolveFields failed: %v", err)
	}
	if len(sel.Fields) != 1 || sel.Fields[0].Key != "url" {
		t.Fatalf("unexpected fields: %+v", sel.Fields)
	}
	if !sel.ShowURL {
		t.Fatalf("URL column should be shown when requested explicitly: %+v", sel)
	}
	if !sel.NeedURL {
		t.Fatalf("NeedURL should be true when field selection includes url: %+v", sel)
	}
}
