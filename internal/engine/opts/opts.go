package opts

import (
	"fmt"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"github.com/phyten/todox/internal/engine"
)

const (
	maxJobs = 64
)

var (
	trueLiterals  = map[string]struct{}{"1": {}, "true": {}, "yes": {}, "on": {}}
	falseLiterals = map[string]struct{}{"0": {}, "false": {}, "no": {}, "off": {}}
)

// Defaults returns the shared baseline options for both CLI and Web inputs.
func Defaults(repoDir string) engine.Options {
	jobs := runtime.NumCPU()
	if jobs < 1 {
		jobs = 1
	}
	if jobs > maxJobs {
		jobs = maxJobs
	}
	return engine.Options{
		Type:         "both",
		Mode:         "last",
		AuthorRegex:  "",
		WithComment:  false,
		WithMessage:  false,
		TruncAll:     0,
		TruncComment: 0,
		TruncMessage: 0,
		IgnoreWS:     true,
		Jobs:         jobs,
		RepoDir:      repoDir,
		Progress:     false,
	}
}

// ApplyWebQueryToOptions copies recognised values from the query string into the
// provided options. Validation happens separately via NormalizeAndValidate.
func ApplyWebQueryToOptions(def engine.Options, q url.Values) (engine.Options, error) {
	out := def

	if raw, ok := lastLiteralValue(q["type"]); ok {
		out.Type = raw
	}
	if raw, ok := lastLiteralValue(q["mode"]); ok {
		out.Mode = raw
	}
	if raw, ok := lastRawValue(q["author"]); ok {
		out.AuthorRegex = raw
	}
	if raw, ok := lastLiteralValue(q["with_comment"]); ok {
		v, err := ParseBool(raw, "with_comment")
		if err != nil {
			return out, err
		}
		out.WithComment = v
	}
	if raw, ok := lastLiteralValue(q["with_message"]); ok {
		v, err := ParseBool(raw, "with_message")
		if err != nil {
			return out, err
		}
		out.WithMessage = v
	}
	if raw, ok := lastLiteralValue(q["truncate"]); ok {
		n, err := parseInt(raw, "truncate")
		if err != nil {
			return out, err
		}
		out.TruncAll = n
	}
	if raw, ok := lastLiteralValue(q["truncate_comment"]); ok {
		n, err := parseInt(raw, "truncate_comment")
		if err != nil {
			return out, err
		}
		out.TruncComment = n
	}
	if raw, ok := lastLiteralValue(q["truncate_message"]); ok {
		n, err := parseInt(raw, "truncate_message")
		if err != nil {
			return out, err
		}
		out.TruncMessage = n
	}
	if raw, ok := lastLiteralValue(q["jobs"]); ok {
		n, err := parseInt(raw, "jobs")
		if err != nil {
			return out, err
		}
		out.Jobs = n
	}
	if raw, ok := lastLiteralValue(q["ignore_ws"]); ok {
		v, err := ParseBool(raw, "ignore_ws")
		if err != nil {
			return out, err
		}
		out.IgnoreWS = v
	}
	if raw, ok := lastLiteralValue(q["progress"]); ok {
		v, err := ParseBool(raw, "progress")
		if err != nil {
			return out, err
		}
		out.Progress = v
	}
	if raw, ok := lastRawValue(q["repo"]); ok {
		out.RepoDir = raw
	}

	return out, nil
}

// NormalizeAndValidate ensures the options are canonical and within the allowed ranges.
func NormalizeAndValidate(o *engine.Options) error {
	o.Type = strings.ToLower(strings.TrimSpace(o.Type))
	switch o.Type {
	case "", "both":
		o.Type = "both"
	case "todo", "fixme":
	default:
		return fmt.Errorf("invalid --type: %s", o.Type)
	}

	o.Mode = strings.ToLower(strings.TrimSpace(o.Mode))
	switch o.Mode {
	case "", "last":
		o.Mode = "last"
	case "first":
	default:
		return fmt.Errorf("invalid --mode: %s", o.Mode)
	}

	if o.Jobs < 1 || o.Jobs > maxJobs {
		return fmt.Errorf("jobs must be between 1 and %d", maxJobs)
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

	if strings.TrimSpace(o.RepoDir) == "" {
		o.RepoDir = "."
	}

	return nil
}

// ParseBool converts a string literal into a boolean, accepting multiple synonyms.
func ParseBool(raw, key string) (bool, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := trueLiterals[v]; ok {
		return true, nil
	}
	if _, ok := falseLiterals[v]; ok {
		return false, nil
	}
	return false, fmt.Errorf("invalid value for %s: %q", key, raw)
}

// ParseIntInRange parses a string into an int and ensures it falls within [min, max].
// If max < min, the upper bound is ignored.
func ParseIntInRange(raw, key string, min, max int) (int, error) {
	n, err := parseInt(raw, key)
	if err != nil {
		return 0, err
	}
	if n < min {
		if max >= min {
			return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
		}
		return 0, fmt.Errorf("%s must be >= %d", key, min)
	}
	if max >= min && n > max {
		return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return n, nil
}

// NormalizeOutput validates and lower-cases the CLI/Web output format value.
func NormalizeOutput(value string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "table", "tsv", "json":
		return v, nil
	}
	return "", fmt.Errorf("invalid --output: %s", value)
}

// SplitMulti turns repeated query parameters (and comma-separated values) into a flat slice.
func SplitMulti(vals []string) []string {
	var out []string
	for _, raw := range vals {
		for _, piece := range strings.Split(raw, ",") {
			part := strings.TrimSpace(piece)
			if part == "" {
				continue
			}
			out = append(out, part)
		}
	}
	return out
}

func parseInt(raw, key string) (int, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, fmt.Errorf("invalid integer value for %s: %q", key, raw)
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %q", key, raw)
	}
	return n, nil
}

func lastLiteralValue(vals []string) (string, bool) {
	flat := SplitMulti(vals)
	if len(flat) == 0 {
		return "", false
	}
	return flat[len(flat)-1], true
}

func lastRawValue(vals []string) (string, bool) {
	for i := len(vals) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(vals[i])
		if trimmed == "" {
			continue
		}
		return trimmed, true
	}
	return "", false
}
