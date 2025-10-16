package opts

import (
	"fmt"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"github.com/phyten/todox/internal/detect"
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
		Type:           "both",
		Mode:           "last",
		DetectMode:     "auto",
		AuthorRegex:    "",
		WithComment:    false,
		WithMessage:    false,
		IncludeStrings: true,
		Tags:           []string{"TODO", "FIXME"},
		TruncAll:       0,
		TruncComment:   0,
		TruncMessage:   0,
		IgnoreWS:       true,
		Jobs:           jobs,
		RepoDir:        repoDir,
		Progress:       false,
		DetectLangs:    nil,
		MaxFileBytes:   0,
		NoPrefilter:    false,
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
	if raw, ok := lastLiteralValue(q["detect"]); ok {
		out.DetectMode = raw
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
	if raw, ok := lastLiteralValue(q["include_strings"]); ok {
		v, err := ParseBool(raw, "include_strings")
		if err != nil {
			return out, err
		}
		out.IncludeStrings = v
	}
	if raw, ok := lastLiteralValue(q["comments_only"]); ok {
		v, err := ParseBool(raw, "comments_only")
		if err != nil {
			return out, err
		}
		if v {
			out.IncludeStrings = false
		} else {
			out.IncludeStrings = true
		}
	}
	if raw, ok := lastLiteralValue(q["no_strings"]); ok {
		v, err := ParseBool(raw, "no_strings")
		if err != nil {
			return out, err
		}
		if v {
			out.IncludeStrings = false
		} else {
			out.IncludeStrings = true
		}
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
		n, err := ParseIntInRange(raw, "jobs", 1, maxJobs)
		if err != nil {
			return out, err
		}
		out.Jobs = n
	}
	if raw, ok := lastLiteralValue(q["max_file_bytes"]); ok {
		n, err := parseInt(raw, "max_file_bytes")
		if err != nil {
			return out, err
		}
		out.MaxFileBytes = n
	}
	if raw, ok := lastLiteralValue(q["ignore_ws"]); ok {
		v, err := ParseBool(raw, "ignore_ws")
		if err != nil {
			return out, err
		}
		out.IgnoreWS = v
	}
	if raw, ok := lastLiteralValue(q["no_prefilter"]); ok {
		v, err := ParseBool(raw, "no_prefilter")
		if err != nil {
			return out, err
		}
		out.NoPrefilter = v
	}
	if raw, ok := lastLiteralValue(q["progress"]); ok {
		v, err := ParseBool(raw, "progress")
		if err != nil {
			return out, err
		}
		out.Progress = v
	}
	if raw := q["path"]; len(raw) > 0 {
		out.Paths = SplitMulti(raw)
	}
	if raw := q["exclude"]; len(raw) > 0 {
		out.Excludes = SplitMulti(raw)
	}
	if raw := q["path_regex"]; len(raw) > 0 {
		out.PathRegex = SplitMulti(raw)
	}
	if raw := q["detect_langs"]; len(raw) > 0 {
		out.DetectLangs = SplitMulti(raw)
	}
	if raw := q["tags"]; len(raw) > 0 {
		out.Tags = SplitMulti(raw)
	}
	if raw, ok := lastLiteralValue(q["exclude_typical"]); ok {
		v, err := ParseBool(raw, "exclude_typical")
		if err != nil {
			return out, err
		}
		out.ExcludeTypical = v
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

	o.DetectMode = strings.ToLower(strings.TrimSpace(o.DetectMode))
	switch o.DetectMode {
	case "", "auto":
		o.DetectMode = "auto"
	case "parse", "regex":
	default:
		return fmt.Errorf("invalid --detect: %s", o.DetectMode)
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

	if o.MaxFileBytes < 0 {
		return fmt.Errorf("max_file_bytes must be >= 0")
	}

	o.Paths = trimSlice(o.Paths)
	o.Excludes = trimSlice(o.Excludes)
	o.PathRegex = trimSlice(o.PathRegex)
	o.DetectLangs = trimSlice(o.DetectLangs)
	if len(o.DetectLangs) > 0 {
		o.DetectLangs = detect.CanonicalDetectLangs(o.DetectLangs)
	}
	o.Tags = trimSlice(o.Tags)

	compiled, err := engine.CompilePathRegex(o.PathRegex)
	if err != nil {
		return fmt.Errorf("invalid --path-regex: %w", err)
	}
	o.PathRegexCompiled = compiled

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

func trimSlice(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:0]
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
