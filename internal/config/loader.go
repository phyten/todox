package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	engineopts "github.com/phyten/todox/internal/engine/opts"
)

var engineKeyMap = map[string]string{
	"type":             "type",
	"mode":             "mode",
	"detect":           "detect",
	"detect_mode":      "detect",
	"author":           "author",
	"path":             "path",
	"paths":            "path",
	"exclude":          "exclude",
	"excludes":         "exclude",
	"path_regex":       "path_regex",
	"path_regexes":     "path_regex",
	"detect_langs":     "detect_langs",
	"detect_languages": "detect_langs",
	"tags":             "tags",
	"exclude_typical":  "exclude_typical",
	"with_comment":     "with_comment",
	"with_message":     "with_message",
	"include_strings":  "include_strings",
	"no_strings":       "no_strings",
	"comments_only":    "comments_only",
	"truncate":         "truncate",
	"truncate_comment": "truncate_comment",
	"truncate_message": "truncate_message",
	"ignore_ws":        "ignore_ws",
	"max_file_bytes":   "max_file_bytes",
	"max_bytes":        "max_file_bytes",
	"jobs":             "jobs",
	"repo":             "repo",
	"output":           "output",
	"color":            "color",
	"no_prefilter":     "no_prefilter",
}

var uiKeyMap = map[string]string{
	"with_age":         "with_age",
	"with_commit_link": "with_commit_link",
	"with_pr_links":    "with_pr_links",
	"pr_state":         "pr_state",
	"pr_limit":         "pr_limit",
	"pr_prefer":        "pr_prefer",
	"fields":           "fields",
	"sort":             "sort",
}

func Load(path string) (Config, error) {
	var cfg Config
	path = strings.TrimSpace(path)
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	var raw map[string]any
	switch ext {
	case ".yaml", ".yml":
		if decodeErr := yaml.Unmarshal(data, &raw); decodeErr != nil {
			return cfg, fmt.Errorf("parse %s: %w", path, decodeErr)
		}
	case ".toml":
		if decodeErr := toml.Unmarshal(data, &raw); decodeErr != nil {
			return cfg, fmt.Errorf("parse %s: %w", path, decodeErr)
		}
	case ".json":
		if decodeErr := json.Unmarshal(data, &raw); decodeErr != nil {
			return cfg, fmt.Errorf("parse %s: %w", path, decodeErr)
		}
	default:
		return cfg, fmt.Errorf("unsupported config extension: %s", ext)
	}
	if raw == nil {
		return cfg, nil
	}
	decoded, err := decodeConfigMap(raw)
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return decoded, nil
}

func decodeConfigMap(raw map[string]any) (Config, error) {
	var cfg Config
	engineSection := make(map[string]any)
	uiSection := make(map[string]any)

	if block, ok := raw["engine"]; ok {
		sub, err := toStringKeyMap(block)
		if err != nil {
			return cfg, fmt.Errorf("engine: %w", err)
		}
		if err := fillSection(engineSection, sub, engineKeyMap, "engine"); err != nil {
			return cfg, err
		}
	}
	if block, ok := raw["ui"]; ok {
		sub, err := toStringKeyMap(block)
		if err != nil {
			return cfg, fmt.Errorf("ui: %w", err)
		}
		if err := fillSection(uiSection, sub, uiKeyMap, "ui"); err != nil {
			return cfg, err
		}
	}

	for key, value := range raw {
		norm := normalizeKey(key)
		switch norm {
		case "engine", "ui":
			continue
		default:
			if canonical, ok := engineKeyMap[norm]; ok {
				engineSection[canonical] = value
				continue
			}
			if canonical, ok := uiKeyMap[norm]; ok {
				uiSection[canonical] = value
				continue
			}
			return cfg, fmt.Errorf("unknown config key: %s", key)
		}
	}

	if err := assignEngine(engineSection, &cfg.Engine); err != nil {
		return cfg, fmt.Errorf("engine: %w", err)
	}
	if err := assignUI(uiSection, &cfg.UI); err != nil {
		return cfg, fmt.Errorf("ui: %w", err)
	}
	return cfg, nil
}

func fillSection(dst, src map[string]any, allowed map[string]string, section string) error {
	for key, value := range src {
		canonical, ok := allowed[normalizeKey(key)]
		if !ok {
			return fmt.Errorf("unknown %s key: %s", section, key)
		}
		dst[canonical] = value
	}
	return nil
}

func assignEngine(section map[string]any, dst *EngineConfig) error {
	if raw, ok := section["include_strings"]; ok {
		b, err := expectBool(raw, "include_strings")
		if err != nil {
			return err
		}
		val := b
		dst.IncludeStrings = &val
	}
	if raw, ok := section["comments_only"]; ok {
		b, err := expectBool(raw, "comments_only")
		if err != nil {
			return err
		}
		val := b
		dst.CommentsOnly = &val
	}
	if raw, ok := section["no_strings"]; ok {
		b, err := expectBool(raw, "no_strings")
		if err != nil {
			return err
		}
		flipped := !b
		dst.IncludeStrings = &flipped
	}

	for key, value := range section {
		switch key {
		case "include_strings", "comments_only", "no_strings":
			continue
		case "type":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.Type = &str
		case "mode":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.Mode = &str
		case "detect":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.Detect = &str
		case "author":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.Author = &str
		case "path":
			list, err := expectStringList(value, key)
			if err != nil {
				return err
			}
			dst.Paths = &list
		case "exclude":
			list, err := expectStringList(value, key)
			if err != nil {
				return err
			}
			dst.Excludes = &list
		case "path_regex":
			list, err := expectStringList(value, key)
			if err != nil {
				return err
			}
			dst.PathRegex = &list
		case "detect_langs":
			list, err := expectStringList(value, key)
			if err != nil {
				return err
			}
			dst.DetectLangs = &list
		case "tags":
			list, err := expectStringList(value, key)
			if err != nil {
				return err
			}
			dst.Tags = &list
		case "exclude_typical":
			b, err := expectBool(value, key)
			if err != nil {
				return err
			}
			dst.ExcludeTypical = &b
		case "with_comment":
			b, err := expectBool(value, key)
			if err != nil {
				return err
			}
			dst.WithComment = &b
		case "with_message":
			b, err := expectBool(value, key)
			if err != nil {
				return err
			}
			dst.WithMessage = &b
		case "truncate":
			n, err := expectInt(value, key)
			if err != nil {
				return err
			}
			dst.TruncAll = &n
		case "truncate_comment":
			n, err := expectInt(value, key)
			if err != nil {
				return err
			}
			dst.TruncComment = &n
		case "truncate_message":
			n, err := expectInt(value, key)
			if err != nil {
				return err
			}
			dst.TruncMessage = &n
		case "ignore_ws":
			b, err := expectBool(value, key)
			if err != nil {
				return err
			}
			dst.IgnoreWS = &b
		case "max_file_bytes":
			n, err := expectInt(value, key)
			if err != nil {
				return err
			}
			dst.MaxFileBytes = &n
		case "jobs":
			n, err := expectInt(value, key)
			if err != nil {
				return err
			}
			dst.Jobs = &n
		case "repo":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.Repo = &str
		case "output":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			trimmed := strings.TrimSpace(str)
			dst.Output = &trimmed
		case "color":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			trimmed := strings.TrimSpace(str)
			dst.Color = &trimmed
		case "no_prefilter":
			b, err := expectBool(value, key)
			if err != nil {
				return err
			}
			dst.NoPrefilter = &b
		default:
			return fmt.Errorf("unknown key: %s", key)
		}
	}
	return nil
}

func assignUI(section map[string]any, dst *UIConfig) error {
	for key, value := range section {
		switch key {
		case "with_age":
			b, err := expectBool(value, key)
			if err != nil {
				return err
			}
			dst.WithAge = &b
		case "with_commit_link":
			b, err := expectBool(value, key)
			if err != nil {
				return err
			}
			dst.WithCommitLink = &b
		case "with_pr_links":
			b, err := expectBool(value, key)
			if err != nil {
				return err
			}
			dst.WithPRLinks = &b
		case "pr_state":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.PRState = &str
		case "pr_limit":
			n, err := expectInt(value, key)
			if err != nil {
				return err
			}
			dst.PRLimit = &n
		case "pr_prefer":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.PRPrefer = &str
		case "fields":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.Fields = &str
		case "sort":
			str, err := expectString(value, key)
			if err != nil {
				return err
			}
			dst.Sort = &str
		default:
			return fmt.Errorf("unknown key: %s", key)
		}
	}
	return nil
}

func expectString(value any, field string) (string, error) {
	if value == nil {
		return "", fmt.Errorf("%s cannot be null", field)
	}
	if s, ok := value.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("expected string for %s, got %T", field, value)
}

func expectBool(value any, field string) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return engineopts.ParseBool(v, field)
	default:
		return false, fmt.Errorf("expected bool for %s, got %T", field, value)
	}
}

func expectInt(value any, field string) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("expected integer for %s, got %v", field, value)
		}
		return int(v), nil
	case json.Number:
		n, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, fmt.Errorf("invalid integer value for %s: %v", field, value)
		}
		return n, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, fmt.Errorf("invalid integer value for %s: %q", field, v)
		}
		n, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, fmt.Errorf("invalid integer value for %s: %q", field, v)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("expected integer for %s, got %T", field, value)
	}
}

func expectStringList(value any, field string) ([]string, error) {
	switch v := value.(type) {
	case string:
		parts := engineopts.SplitMulti([]string{v})
		return normalizeList(parts), nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			str, err := expectString(item, field)
			if err != nil {
				return nil, err
			}
			out = append(out, str)
		}
		return normalizeList(out), nil
	case []string:
		return normalizeList(v), nil
	default:
		return nil, fmt.Errorf("expected string or list for %s, got %T", field, value)
	}
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func toStringKeyMap(v any) (map[string]any, error) {
	switch typed := v.(type) {
	case map[string]any:
		return typed, nil
	case map[any]any:
		out := make(map[string]any, len(typed))
		for k, value := range typed {
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key: %v", k)
			}
			out[key] = value
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected map, got %T", v)
	}
}

func normalizeKey(key string) string {
	norm := strings.ToLower(strings.TrimSpace(key))
	norm = strings.ReplaceAll(norm, "-", "_")
	return norm
}
