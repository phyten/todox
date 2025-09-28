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

// Defaults は CLI / Web の双方で利用する共通既定値を返します。
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

// ApplyWebQueryToOptions は URL クエリ文字列を Options に反映します。
func ApplyWebQueryToOptions(def engine.Options, q url.Values) (engine.Options, error) {
	opts := def

	if raw, ok := lastValue(q, "type"); ok {
		opts.Type = strings.ToLower(raw)
	}
	if raw, ok := lastValue(q, "mode"); ok {
		opts.Mode = strings.ToLower(raw)
	}
	if raw, ok := lastValue(q, "author"); ok {
		opts.AuthorRegex = raw
	}
	if raw, ok := lastValue(q, "repo"); ok {
		opts.RepoDir = raw
	}
	if raw, ok := lastValue(q, "with_comment"); ok {
		v, err := ParseBool(raw, "with_comment")
		if err != nil {
			return opts, err
		}
		opts.WithComment = v
	}
	if raw, ok := lastValue(q, "with_message"); ok {
		v, err := ParseBool(raw, "with_message")
		if err != nil {
			return opts, err
		}
		opts.WithMessage = v
	}
	if raw, ok := lastValue(q, "ignore_ws"); ok {
		v, err := ParseBool(raw, "ignore_ws")
		if err != nil {
			return opts, err
		}
		opts.IgnoreWS = v
	}
	if raw, ok := lastValue(q, "jobs"); ok {
		v, err := ParseIntInRange(raw, "jobs", 1, 64)
		if err != nil {
			return opts, err
		}
		opts.Jobs = v
	}
	if raw, ok := lastValue(q, "truncate"); ok {
		v, err := ParseIntInRange(raw, "truncate", 0, math.MinInt)
		if err != nil {
			return opts, err
		}
		opts.TruncAll = v
	}
	if raw, ok := lastValue(q, "truncate_comment"); ok {
		v, err := ParseIntInRange(raw, "truncate_comment", 0, math.MinInt)
		if err != nil {
			return opts, err
		}
		opts.TruncComment = v
	}
	if raw, ok := lastValue(q, "truncate_message"); ok {
		v, err := ParseIntInRange(raw, "truncate_message", 0, math.MinInt)
		if err != nil {
			return opts, err
		}
		opts.TruncMessage = v
	}
	if raw, ok := lastValue(q, "progress"); ok {
		v, err := ParseBool(raw, "progress")
		if err != nil {
			return opts, err
		}
		opts.Progress = v
	}

	if err := NormalizeAndValidate(&opts); err != nil {
		return opts, err
	}
	return opts, nil
}

// NormalizeAndValidate は Options の小文字化や範囲チェックを行います。
func NormalizeAndValidate(o *engine.Options) error {
	o.Type = strings.TrimSpace(strings.ToLower(o.Type))
	switch o.Type {
	case "", "both":
		o.Type = "both"
	case "todo", "fixme":
	default:
		return fmt.Errorf("invalid --type: %s", o.Type)
	}

	o.Mode = strings.TrimSpace(strings.ToLower(o.Mode))
	switch o.Mode {
	case "", "last":
		o.Mode = "last"
	case "first":
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
	return nil
}

// ParseBool は真偽値のバリエーションを受理して bool を返します。
func ParseBool(raw, key string) (bool, error) {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid value for %s: %q", key, raw)
	}
}

// ParseIntInRange は整数の解析と範囲チェックを行います。
// max < min の場合は上限無しとして扱います。
func ParseIntInRange(raw, key string, min, max int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %q", key, raw)
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

// NormalizeOutput は出力形式の小文字化と検証を行います。
func NormalizeOutput(raw string) (string, error) {
	out := strings.TrimSpace(strings.ToLower(raw))
	if out == "" {
		out = "table"
	}
	switch out {
	case "table", "tsv", "json":
		return out, nil
	default:
		return "", fmt.Errorf("invalid --output: %s", raw)
	}
}

// SplitMulti は "foo,bar" のような複数指定を分割しトリムします。
func SplitMulti(vals []string) []string {
	var out []string
	for _, v := range vals {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func lastValue(q url.Values, key string) (string, bool) {
	vals := SplitMulti(q[key])
	if len(vals) == 0 {
		return "", false
	}
	return vals[len(vals)-1], true
}
