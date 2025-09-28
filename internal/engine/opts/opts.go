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

type Source int

const (
	FromWeb Source = iota
	FromCLI
)

func Defaults(repoDir string) engine.Options {
	jobs := runtime.NumCPU()
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

func ApplyWebQueryToOptions(def engine.Options, q url.Values) (engine.Options, error) {
	out := def

	if raw := strings.TrimSpace(q.Get("type")); raw != "" {
		out.Type = raw
	}
	if raw := strings.TrimSpace(q.Get("mode")); raw != "" {
		out.Mode = raw
	}
	if raw := strings.TrimSpace(q.Get("author")); raw != "" {
		out.AuthorRegex = raw
	}
	if raw := strings.TrimSpace(q.Get("with_comment")); raw != "" {
		v, err := ParseBool(raw, "with_comment")
		if err != nil {
			return def, err
		}
		out.WithComment = v
	}
	if raw := strings.TrimSpace(q.Get("with_message")); raw != "" {
		v, err := ParseBool(raw, "with_message")
		if err != nil {
			return def, err
		}
		out.WithMessage = v
	}
	if raw := strings.TrimSpace(q.Get("truncate")); raw != "" {
		v, err := ParseIntInRange(raw, "truncate", 0, math.MaxInt)
		if err != nil {
			return def, err
		}
		out.TruncAll = v
	}
	if raw := strings.TrimSpace(q.Get("truncate_comment")); raw != "" {
		v, err := ParseIntInRange(raw, "truncate_comment", 0, math.MaxInt)
		if err != nil {
			return def, err
		}
		out.TruncComment = v
	}
	if raw := strings.TrimSpace(q.Get("truncate_message")); raw != "" {
		v, err := ParseIntInRange(raw, "truncate_message", 0, math.MaxInt)
		if err != nil {
			return def, err
		}
		out.TruncMessage = v
	}
	if raw := strings.TrimSpace(q.Get("ignore_ws")); raw != "" {
		v, err := ParseBool(raw, "ignore_ws")
		if err != nil {
			return def, err
		}
		out.IgnoreWS = v
	}
	if raw := strings.TrimSpace(q.Get("progress")); raw != "" {
		v, err := ParseBool(raw, "progress")
		if err != nil {
			return def, err
		}
		out.Progress = v
	}
	if raw := strings.TrimSpace(q.Get("jobs")); raw != "" {
		v, err := ParseIntInRange(raw, "jobs", 1, 64)
		if err != nil {
			return def, err
		}
		out.Jobs = v
	}

	return out, nil
}

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

	if o.Jobs < 1 || o.Jobs > 64 {
		return fmt.Errorf("jobs must be between 1 and 64")
	}

	if o.RepoDir == "" {
		o.RepoDir = "."
	}

	return nil
}

func ParseBool(raw, key string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	case "":
		return false, fmt.Errorf("invalid value for %s: %q", key, raw)
	default:
		return false, fmt.Errorf("invalid value for %s: %q", key, raw)
	}
}

func ParseIntInRange(raw, key string, min, max int) (int, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return 0, fmt.Errorf("invalid integer value for %s: %q", key, raw)
	}
	n, err := strconv.Atoi(normalized)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %q", key, raw)
	}
	if n < min || (max != math.MaxInt && n > max) {
		if max != math.MaxInt {
			return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
		}
		return 0, fmt.Errorf("%s must be >= %d", key, min)
	}
	return n, nil
}

func SplitMulti(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		for _, token := range strings.Split(v, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			out = append(out, token)
		}
	}
	return out
}
