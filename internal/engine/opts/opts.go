package opts

import (
	"fmt"
	"math"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"github.com/example/todox/internal/engine"
)

const maxInt = math.MaxInt

// Defaults returns the baseline engine options shared between CLI and Web frontends.
func Defaults(repoDir string) engine.Options {
	jobs := runtime.NumCPU()
	if jobs < 1 {
		jobs = 1
	}
	if jobs > 64 {
		jobs = 64
	}
	return engine.Options{
		Type:         "both",
		Mode:         "last",
		AuthorRegex:  "",
		WithComment:  false,
		WithMessage:  false,
		TruncAll:     120,
		TruncComment: 0,
		TruncMessage: 0,
		IgnoreWS:     true,
		Jobs:         jobs,
		RepoDir:      repoDir,
		Progress:     false,
	}
}

// ApplyWebQueryToOptions reads URL query parameters and applies them to the given defaults.
func ApplyWebQueryToOptions(def engine.Options, q url.Values) (engine.Options, error) {
	opts := def

	if raw, ok := firstNonEmpty(q, "type"); ok {
		opts.Type = raw
	}
	if raw, ok := firstNonEmpty(q, "mode"); ok {
		opts.Mode = raw
	}
	if raw, ok := firstNonEmpty(q, "author"); ok {
		opts.AuthorRegex = raw
	}
	if raw, ok := firstNonEmpty(q, "with_comment"); ok {
		v, err := ParseBool(raw, "with_comment")
		if err != nil {
			return def, err
		}
		opts.WithComment = v
	}
	if raw, ok := firstNonEmpty(q, "with_message"); ok {
		v, err := ParseBool(raw, "with_message")
		if err != nil {
			return def, err
		}
		opts.WithMessage = v
	}
	if raw, ok := firstNonEmpty(q, "truncate"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return def, fmt.Errorf("invalid integer value for truncate: %q", raw)
		}
		opts.TruncAll = n
	}
	if raw, ok := firstNonEmpty(q, "truncate_comment"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return def, fmt.Errorf("invalid integer value for truncate_comment: %q", raw)
		}
		opts.TruncComment = n
	}
	if raw, ok := firstNonEmpty(q, "truncate_message"); ok {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return def, fmt.Errorf("invalid integer value for truncate_message: %q", raw)
		}
		opts.TruncMessage = n
	}
	if raw, ok := firstNonEmpty(q, "jobs"); ok {
		n, err := ParseIntInRange(raw, "jobs", 1, 64)
		if err != nil {
			return def, err
		}
		opts.Jobs = n
	}
	if raw, ok := firstNonEmpty(q, "ignore_ws"); ok {
		v, err := ParseBool(raw, "ignore_ws")
		if err != nil {
			return def, err
		}
		opts.IgnoreWS = v
	}
	return opts, nil
}

// NormalizeAndValidate harmonises option fields and performs final validation.
func NormalizeAndValidate(o *engine.Options) error {
	o.Type = strings.ToLower(strings.TrimSpace(o.Type))
	if o.Type == "" {
		o.Type = "both"
	}
	switch o.Type {
	case "todo", "fixme", "both":
	default:
		return fmt.Errorf("invalid --type: %s", o.Type)
	}

	o.Mode = strings.ToLower(strings.TrimSpace(o.Mode))
	if o.Mode == "" {
		o.Mode = "last"
	}
	switch o.Mode {
	case "last", "first":
	default:
		return fmt.Errorf("invalid --mode: %s", o.Mode)
	}

	if o.TruncAll < 0 {
		return fmt.Errorf("truncate must be >= 0")
	}
	if o.TruncComment < 0 {
		return fmt.Errorf("truncate_comment must be >= 0")
	}
	if o.TruncMessage < 0 {
		return fmt.Errorf("truncate_message must be >= 0")
	}

	if o.WithComment && o.WithMessage && o.TruncAll == 0 && o.TruncComment == 0 && o.TruncMessage == 0 {
		o.TruncAll = 120
	}

	if o.Jobs < 1 || o.Jobs > 64 {
		return fmt.Errorf("jobs must be between 1 and 64")
	}

	if o.RepoDir == "" {
		o.RepoDir = "."
	}

	return nil
}

// NormalizeOutput validates the requested output format.
func NormalizeOutput(raw string) (string, error) {
	out := strings.ToLower(strings.TrimSpace(raw))
	if out == "" {
		return "table", nil
	}
	switch out {
	case "table", "tsv", "json":
		return out, nil
	default:
		return "", fmt.Errorf("invalid output: %s", raw)
	}
}

// ParseBool interprets a boolean flag supporting several synonyms.
func ParseBool(raw, key string) (bool, error) {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	case "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid value for %s: %q", key, raw)
	}
}

// ParseIntInRange parses an integer and validates it is within the provided bounds.
func ParseIntInRange(raw, key string, min, max int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %q", key, raw)
	}
	if n < min || n > max {
		if max == maxInt {
			return 0, fmt.Errorf("%s must be >= %d", key, min)
		}
		return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return n, nil
}

// SplitMulti splits comma-separated values, trimming whitespace and removing blanks.
func SplitMulti(vals []string) []string {
	if len(vals) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(vals))
	for _, raw := range vals {
		parts := strings.Split(raw, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

func firstNonEmpty(q url.Values, key string) (string, bool) {
	vals, ok := q[key]
	if !ok || len(vals) == 0 {
		return "", false
	}
	raw := strings.TrimSpace(vals[0])
	if raw == "" {
		return "", false
	}
	return raw, true
}
