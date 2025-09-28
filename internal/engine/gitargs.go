package engine

import (
	"path/filepath"
	"regexp"
	"strings"
)

var typicalExcludePatterns = []string{
	":(glob,exclude)vendor/**",
	":(glob,exclude)node_modules/**",
	":(glob,exclude)dist/**",
	":(glob,exclude)build/**",
	":(glob,exclude)target/**",
	":(glob,exclude)*.min.*",
}

// buildGrepPathspecs builds the list to append after "--" for `git grep`.
func buildGrepPathspecs(includes, excludes []string, typical bool) []string {
	normalizedIncludes := make([]string, 0, len(includes))
	for _, raw := range includes {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, ":") {
			normalizedIncludes = append(normalizedIncludes, filepath.ToSlash(trimmed))
			continue
		}
		normalizedIncludes = append(normalizedIncludes, filepath.ToSlash(trimmed))
	}

	out := make([]string, 0, len(normalizedIncludes)+len(excludes)+len(typicalExcludePatterns)+1)
	if len(normalizedIncludes) == 0 {
		out = append(out, ".")
	} else {
		out = append(out, normalizedIncludes...)
	}

	if typical {
		out = append(out, typicalExcludePatterns...)
	}

	for _, raw := range excludes {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		trimmed = filepath.ToSlash(trimmed)
		if strings.HasPrefix(trimmed, ":!") || strings.HasPrefix(trimmed, ":(exclude)") || strings.HasPrefix(trimmed, ":(glob,exclude)") {
			out = append(out, trimmed)
			continue
		}
		out = append(out, ":(glob,exclude)"+trimmed)
	}
	return out
}

func compilePathRegex(patterns []string) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, raw := range patterns {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		rx, err := regexp.Compile(trimmed)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, rx)
	}
	return compiled, nil
}

func filterByPathRegex(matches []match, rx []*regexp.Regexp) []match {
	if len(rx) == 0 {
		return matches
	}
	out := matches[:0]
	for _, m := range matches {
		for _, r := range rx {
			if r.MatchString(m.file) {
				out = append(out, m)
				break
			}
		}
	}
	return out
}
