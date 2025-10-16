package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func strPtr(s string) *string { return &s }

func intPtr(n int) *int { return &n }

func stringsPtr(values ...string) *[]string {
	copied := append([]string(nil), values...)
	return &copied
}

func TestMergeEnginePrecedence(t *testing.T) {
	base := EngineSettings{Type: "both", Detect: "auto", IgnoreWS: true, Jobs: 2, Paths: []string{"base"}, IncludeStrings: true}

	fileCfg := EngineConfig{Type: strPtr("todo"), Detect: strPtr("parse"), IgnoreWS: boolPtr(false), Paths: stringsPtr("file")}
	envCfg := EngineConfig{Type: strPtr("fixme"), Paths: stringsPtr("env"), CommentsOnly: boolPtr(true)}
	flagCfg := EngineConfig{Type: strPtr("both"), Paths: stringsPtr("flag"), Jobs: intPtr(8), IncludeStrings: boolPtr(true), Detect: strPtr("regex")}

	merged := MergeEngine(base, fileCfg, envCfg, flagCfg)

	if merged.Type != "both" {
		t.Fatalf("expected Type both, got %q", merged.Type)
	}
	if merged.Detect != "regex" {
		t.Fatalf("expected Detect regex, got %q", merged.Detect)
	}
	if !reflect.DeepEqual(merged.Paths, []string{"flag"}) {
		t.Fatalf("unexpected paths: %v", merged.Paths)
	}
	if merged.IgnoreWS {
		t.Fatal("expected IgnoreWS to be false")
	}
	if merged.Jobs != 8 {
		t.Fatalf("expected Jobs 8, got %d", merged.Jobs)
	}
	if !merged.IncludeStrings {
		t.Fatal("expected IncludeStrings true after flag override")
	}
}

func TestMergeUIPrecedence(t *testing.T) {
	base := UISettings{WithAge: false, PRState: "all", PRLimit: 3, PRPrefer: "open"}

	fileCfg := UIConfig{WithAge: boolPtr(true), PRState: strPtr("closed")}
	envCfg := UIConfig{PRState: strPtr("merged"), PRLimit: intPtr(5)}
	flagCfg := UIConfig{WithPRLinks: boolPtr(true), PRState: strPtr("open")}

	merged := MergeUI(base, fileCfg, envCfg, flagCfg)
	if !merged.WithPRLinks {
		t.Fatal("expected WithPRLinks true")
	}
	if merged.WithAge != true {
		t.Fatal("expected WithAge true from file layer")
	}
	if merged.PRState != "open" {
		t.Fatalf("expected PRState open, got %q", merged.PRState)
	}
	if merged.PRLimit != 5 {
		t.Fatalf("expected PRLimit 5, got %d", merged.PRLimit)
	}
}

func TestFromEnv(t *testing.T) {
	env := map[string]string{
		"TODOX_TYPE":             "todo",
		"TODOX_DETECT":           "regex",
		"TODOX_AUTHOR":           "Alice",
		"TODOX_WITH_COMMENT":     "1",
		"TODOX_WITH_MESSAGE":     "true",
		"TODOX_PATH":             "src,cmd",
		"TODOX_PATH_REGEX":       ".*\\.go$",
		"TODOX_EXCLUDE":          "vendor,dist",
		"TODOX_EXCLUDE_TYPICAL":  "yes",
		"TODOX_DETECT_LANGS":     "go,py",
		"TODOX_TAGS":             "TODO,FIXME",
		"TODOX_INCLUDE_STRINGS":  "1",
		"TODOX_NO_STRINGS":       "true",
		"TODOX_COMMENTS_ONLY":    "1",
		"TODOX_TRUNCATE":         "5000",
		"TODOX_TRUNCATE_COMMENT": "80",
		"TODOX_TRUNCATE_MESSAGE": "72",
		"TODOX_IGNORE_WS":        "0",
		"TODOX_MAX_FILE_BYTES":   "8192",
		"TODOX_JOBS":             "128",
		"TODOX_PR_STATE":         "open",
		"TODOX_PR_LIMIT":         "4",
		"TODOX_PR_PREFER":        "merged",
		"TODOX_WITH_AGE":         "true",
		"TODOX_WITH_PR_LINKS":    "yes",
		"TODOX_FIELDS":           "type,author",
		"TODOX_SORT":             "-age",
		"TODOX_NO_PREFILTER":     "1",
	}
	cfg, err := FromEnv(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}
	if cfg.Engine.Type == nil || *cfg.Engine.Type != "todo" {
		t.Fatalf("expected Type todo, got %+v", cfg.Engine.Type)
	}
	if cfg.Engine.Author == nil || *cfg.Engine.Author != "Alice" {
		t.Fatalf("expected Author Alice, got %+v", cfg.Engine.Author)
	}
	if cfg.Engine.Detect == nil || *cfg.Engine.Detect != "regex" {
		t.Fatalf("expected Detect regex, got %+v", cfg.Engine.Detect)
	}
	if cfg.Engine.WithComment == nil || !*cfg.Engine.WithComment {
		t.Fatal("expected WithComment true")
	}
	if cfg.Engine.WithMessage == nil || !*cfg.Engine.WithMessage {
		t.Fatal("expected WithMessage true")
	}
	if cfg.Engine.IncludeStrings == nil || *cfg.Engine.IncludeStrings {
		t.Fatal("expected IncludeStrings false")
	}
	if cfg.Engine.Paths == nil || !reflect.DeepEqual(*cfg.Engine.Paths, []string{"src", "cmd"}) {
		t.Fatalf("unexpected paths: %v", cfg.Engine.Paths)
	}
	if cfg.Engine.PathRegex == nil || !reflect.DeepEqual(*cfg.Engine.PathRegex, []string{".*\\.go$"}) {
		t.Fatalf("unexpected path_regex: %v", cfg.Engine.PathRegex)
	}
	if cfg.Engine.Excludes == nil || !reflect.DeepEqual(*cfg.Engine.Excludes, []string{"vendor", "dist"}) {
		t.Fatalf("unexpected excludes: %v", cfg.Engine.Excludes)
	}
	if cfg.Engine.DetectLangs == nil || !reflect.DeepEqual(*cfg.Engine.DetectLangs, []string{"go", "py"}) {
		t.Fatalf("unexpected detect_langs: %v", cfg.Engine.DetectLangs)
	}
	if cfg.Engine.Tags == nil || !reflect.DeepEqual(*cfg.Engine.Tags, []string{"TODO", "FIXME"}) {
		t.Fatalf("unexpected tags: %v", cfg.Engine.Tags)
	}
	if cfg.Engine.ExcludeTypical == nil || !*cfg.Engine.ExcludeTypical {
		t.Fatal("expected ExcludeTypical true")
	}
	if cfg.Engine.TruncAll == nil || *cfg.Engine.TruncAll != 5000 {
		t.Fatalf("unexpected truncate: %+v", cfg.Engine.TruncAll)
	}
	if cfg.Engine.TruncComment == nil || *cfg.Engine.TruncComment != 80 {
		t.Fatalf("unexpected truncate_comment: %+v", cfg.Engine.TruncComment)
	}
	if cfg.Engine.TruncMessage == nil || *cfg.Engine.TruncMessage != 72 {
		t.Fatalf("unexpected truncate_message: %+v", cfg.Engine.TruncMessage)
	}
	if cfg.Engine.IgnoreWS == nil || *cfg.Engine.IgnoreWS {
		t.Fatal("expected IgnoreWS false")
	}
	if cfg.Engine.MaxFileBytes == nil || *cfg.Engine.MaxFileBytes != 8192 {
		t.Fatalf("unexpected max_file_bytes: %+v", cfg.Engine.MaxFileBytes)
	}
	if cfg.Engine.Jobs == nil || *cfg.Engine.Jobs != 128 {
		t.Fatalf("expected Jobs 128, got %+v", cfg.Engine.Jobs)
	}
	if cfg.Engine.NoPrefilter == nil || !*cfg.Engine.NoPrefilter {
		t.Fatal("expected NoPrefilter true")
	}
	if cfg.UI.PRState == nil || *cfg.UI.PRState != "open" {
		t.Fatalf("expected PRState open, got %+v", cfg.UI.PRState)
	}
	if cfg.UI.PRLimit == nil || *cfg.UI.PRLimit != 4 {
		t.Fatalf("expected PRLimit 4, got %+v", cfg.UI.PRLimit)
	}
	if cfg.UI.PRPrefer == nil || *cfg.UI.PRPrefer != "merged" {
		t.Fatalf("expected PRPrefer merged, got %+v", cfg.UI.PRPrefer)
	}
	if cfg.UI.WithAge == nil || !*cfg.UI.WithAge {
		t.Fatal("expected WithAge true")
	}
	if cfg.UI.WithPRLinks == nil || !*cfg.UI.WithPRLinks {
		t.Fatal("expected WithPRLinks true")
	}
	if cfg.UI.Fields == nil || *cfg.UI.Fields != "type,author" {
		t.Fatalf("unexpected fields: %+v", cfg.UI.Fields)
	}
	if cfg.UI.Sort == nil || *cfg.UI.Sort != "-age" {
		t.Fatalf("unexpected sort: %+v", cfg.UI.Sort)
	}
}

func TestAssignEngineNoStrings(t *testing.T) {
	section := map[string]any{
		"no_strings": true,
	}
	var cfg EngineConfig
	if err := assignEngine(section, &cfg); err != nil {
		t.Fatalf("assignEngine returned error: %v", err)
	}
	if cfg.IncludeStrings == nil || *cfg.IncludeStrings {
		t.Fatal("expected IncludeStrings to be false when no_strings is true")
	}
}

func TestLoadConfigFormats(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		".yaml": "type: fixme\ndetect: parse\npath:\n  - src\nwith_comment: true\ninclude_strings: false\nmax_file_bytes: 2048\nno_prefilter: true\ntags:\n  - FIXME\nui:\n  pr_state: merged\n  with_age: true\n",
		".toml": "type = \"todo\"\ndetect = \"regex\"\ndetect_langs = [\"go\"]\npath = [\"cmd\"]\nwith_message = true\n[ui]\npr_limit = 6\nwith_pr_links = true\n",
		".json": "{\n  \"engine\": {\"type\": \"todo\", \"exclude\": [\"vendor\"], \"tags\": [\"TODO\", \"FIXME\"]},\n  \"pr_prefer\": \"closed\"\n}\n",
	}

	for ext, content := range cases {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(dir, "config"+ext)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if cfg.Engine.Type == nil {
				t.Fatal("expected engine type to be set")
			}
			switch ext {
			case ".yaml":
				if *cfg.Engine.Type != "fixme" {
					t.Fatalf("yaml type mismatch: %q", *cfg.Engine.Type)
				}
				if cfg.Engine.Detect == nil || *cfg.Engine.Detect != "parse" {
					t.Fatalf("yaml detect mismatch: %q", ptrString(cfg.Engine.Detect))
				}
				if cfg.Engine.WithComment == nil || !*cfg.Engine.WithComment {
					t.Fatal("yaml with_comment should be true")
				}
				if cfg.Engine.IncludeStrings == nil || *cfg.Engine.IncludeStrings {
					t.Fatal("yaml include_strings should be false")
				}
				if cfg.Engine.MaxFileBytes == nil || *cfg.Engine.MaxFileBytes != 2048 {
					t.Fatalf("yaml max_file_bytes mismatch: %d", ptrInt(cfg.Engine.MaxFileBytes))
				}
				if cfg.Engine.NoPrefilter == nil || !*cfg.Engine.NoPrefilter {
					t.Fatal("yaml no_prefilter should be true")
				}
				if cfg.UI.PRState == nil || *cfg.UI.PRState != "merged" {
					t.Fatalf("yaml pr_state mismatch: %q", ptrString(cfg.UI.PRState))
				}
				if cfg.UI.WithAge == nil || !*cfg.UI.WithAge {
					t.Fatal("yaml with_age should be true")
				}
			case ".toml":
				if cfg.Engine.WithMessage == nil || !*cfg.Engine.WithMessage {
					t.Fatal("toml with_message should be true")
				}
				if cfg.Engine.Detect == nil || *cfg.Engine.Detect != "regex" {
					t.Fatalf("toml detect mismatch: %q", ptrString(cfg.Engine.Detect))
				}
				if cfg.Engine.DetectLangs == nil || !reflect.DeepEqual(*cfg.Engine.DetectLangs, []string{"go"}) {
					t.Fatalf("toml detect_langs mismatch: %v", cfg.Engine.DetectLangs)
				}
				if cfg.UI.PRLimit == nil || *cfg.UI.PRLimit != 6 {
					t.Fatalf("toml pr_limit mismatch: %d", ptrInt(cfg.UI.PRLimit))
				}
				if cfg.UI.WithPRLinks == nil || !*cfg.UI.WithPRLinks {
					t.Fatal("toml with_pr_links should be true")
				}
			case ".json":
				if cfg.Engine.Excludes == nil || !reflect.DeepEqual(*cfg.Engine.Excludes, []string{"vendor"}) {
					t.Fatalf("json exclude mismatch: %v", cfg.Engine.Excludes)
				}
				if cfg.Engine.Tags == nil || !reflect.DeepEqual(*cfg.Engine.Tags, []string{"TODO", "FIXME"}) {
					t.Fatalf("json tags mismatch: %v", cfg.Engine.Tags)
				}
				if cfg.UI.PRPrefer == nil || *cfg.UI.PRPrefer != "closed" {
					t.Fatalf("json pr_prefer mismatch: %q", ptrString(cfg.UI.PRPrefer))
				}
			}
		})
	}
}

func TestLoadUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("unknown: value\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestFindOrder(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	if mkErr := os.MkdirAll(filepath.Join(repoRoot, "sub", "dir"), 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	repoConfig := filepath.Join(repoRoot, ".todox.yaml")
	if writeErr := os.WriteFile(repoConfig, []byte("type: todo\n"), 0o644); writeErr != nil {
		t.Fatalf("write repo config: %v", writeErr)
	}
	path, where, err := Find(filepath.Join(repoRoot, "sub", "dir"), "", "", "")
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if path != repoConfig || where != "cwd-up" {
		t.Fatalf("unexpected result: path=%s where=%s", path, where)
	}

	explicitDir := t.TempDir()
	explicit := filepath.Join(explicitDir, "custom.toml")
	if writeErr := os.WriteFile(explicit, []byte("type='fixme'\n"), 0o644); writeErr != nil {
		t.Fatalf("write explicit: %v", writeErr)
	}
	path, where, err = Find(repoRoot, explicit, "", "")
	if err != nil {
		t.Fatalf("Find explicit failed: %v", err)
	}
	if path != explicit || where != "explicit" {
		t.Fatalf("expected explicit config, got path=%s where=%s", path, where)
	}

	xdgHome := t.TempDir()
	if mkErr := os.MkdirAll(filepath.Join(xdgHome, "todox"), 0o755); mkErr != nil {
		t.Fatalf("mkdir xdg: %v", mkErr)
	}
	xdgPath := filepath.Join(xdgHome, "todox", "config.json")
	if writeErr := os.WriteFile(xdgPath, []byte("{}"), 0o644); writeErr != nil {
		t.Fatalf("write xdg: %v", writeErr)
	}
	path, where, err = Find(t.TempDir(), "", xdgHome, "")
	if err != nil {
		t.Fatalf("Find xdg failed: %v", err)
	}
	if path != xdgPath || where != "xdg" {
		t.Fatalf("expected xdg config, got path=%s where=%s", path, where)
	}

	homeDir := t.TempDir()
	homePath := filepath.Join(homeDir, ".todox.toml")
	if writeErr := os.WriteFile(homePath, []byte("type='both'\n"), 0o644); writeErr != nil {
		t.Fatalf("write home: %v", writeErr)
	}
	path, where, err = Find(t.TempDir(), "", "", homeDir)
	if err != nil {
		t.Fatalf("Find home failed: %v", err)
	}
	if path != homePath || where != "home" {
		t.Fatalf("expected home config, got path=%s where=%s", path, where)
	}
}

func TestNormalizeUI(t *testing.T) {
	values := UISettings{PRState: "OPEN", PRLimit: 4, PRPrefer: "MERGED", Fields: " type,author ", Sort: " -age "}
	normalized, err := NormalizeUI(values)
	if err != nil {
		t.Fatalf("NormalizeUI error: %v", err)
	}
	if normalized.PRState != "open" {
		t.Fatalf("expected pr_state open, got %q", normalized.PRState)
	}
	if normalized.PRPrefer != "merged" {
		t.Fatalf("expected pr_prefer merged, got %q", normalized.PRPrefer)
	}
	if normalized.Sort != "-age" {
		t.Fatalf("expected sort -age, got %q", normalized.Sort)
	}
	if normalized.Fields != "type,author" {
		t.Fatalf("expected fields trimmed, got %q", normalized.Fields)
	}

	if _, err := NormalizeUI(UISettings{PRState: "open", PRLimit: 0}); err == nil {
		t.Fatal("expected error for invalid pr_limit")
	}
}

func ptrString(v *string) string {
	if v == nil {
		return "<nil>"
	}
	return *v
}

func ptrInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
