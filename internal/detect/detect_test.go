package detect

import "testing"

func TestNormalizeLangNameAliases(t *testing.T) {
	cases := map[string]string{
		"JS":   "javascript",
		"Ts":   "typescript",
		"c++":  "cpp",
		"Py":   "python",
		"bash": "shell",
	}
	for input, want := range cases {
		if got := NormalizeLangName(input); got != want {
			t.Fatalf("NormalizeLangName(%q)=%q want %q", input, got, want)
		}
	}
}

func TestCanonicalDetectLangsDedupes(t *testing.T) {
	in := []string{" js ", "TS", "js", "PY"}
	got := CanonicalDetectLangs(in)
	want := []string{"javascript", "typescript", "python"}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value mismatch at %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestFromPathAndContentMatlabHeuristic(t *testing.T) {
	data := []byte("% comment\nfunction y = square(x)\ny = x.^2;\nend\n")
	info := FromPathAndContent("foo.m", data)
	if info.Name != "" {
		t.Fatalf("expected matlab-like .m files to fall back, got %q", info.Name)
	}
}

func TestFromPathAndContentObjectiveCPreferred(t *testing.T) {
	data := []byte("#import <Foundation/Foundation.h>\n@interface Foo : NSObject\n@end\n")
	info := FromPathAndContent("bar.m", data)
	if info.Name != "objective-c" {
		t.Fatalf("expected objective-c heuristics to remain, got %q", info.Name)
	}
}
