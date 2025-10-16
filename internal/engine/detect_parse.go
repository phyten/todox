package engine

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/phyten/todox/internal/detect"
	"github.com/phyten/todox/internal/model"
)

type detectionMode int

const (
	detectionModeRegex detectionMode = iota
	detectionModeParse
	detectionModeAuto
)

type commentStyle struct {
	linePrefixes []string
	block        []blockPattern
	stringDelims []string
}

type blockPattern struct {
	start              string
	end                string
	kind               model.MatchKind
	allowIndentedStart bool
}

type parseJob struct {
	path string
}

type parseResult struct {
	matches []model.Match
	errs    []ItemError
}

type tagSpec struct {
	raw   string
	upper string
}

func collectMatches(ctx context.Context, opts Options, searchTags []string) ([]model.Match, []ItemError, error) {
	var mode detectionMode
	switch strings.ToLower(strings.TrimSpace(opts.DetectMode)) {
	case "", "auto":
		mode = detectionModeAuto
	case "regex":
		mode = detectionModeRegex
	case "parse":
		mode = detectionModeParse
	default:
		return nil, nil, fmt.Errorf("invalid detect mode: %s", opts.DetectMode)
	}
	switch mode {
	case detectionModeRegex:
		matches, errs, err := collectMatchesRegex(opts, searchTags)
		return matches, errs, err
	case detectionModeParse:
		return collectMatchesParse(ctx, opts, searchTags, false)
	case detectionModeAuto:
		return collectMatchesParse(ctx, opts, searchTags, true)
	default:
		return nil, nil, fmt.Errorf("unknown detect mode")
	}
}

func collectMatchesRegex(opts Options, tags []string) ([]model.Match, []ItemError, error) {
	pattern := patternForTags(tags)
	matches, err := gitGrepMatches(opts.RepoDir, pattern, opts.Paths, opts.Excludes, opts.ExcludeTypical)
	if err != nil {
		return nil, nil, err
	}
	matches = filterByPathRegex(matches, opts.PathRegexCompiled)
	// refine tags inside each match (multiple per line)
	expanded := expandLineMatches(matches, tags)
	return expanded, nil, nil
}

func collectMatchesParse(ctx context.Context, opts Options, tags []string, allowFallback bool) ([]model.Match, []ItemError, error) {
	pattern := patternForTags(tags)
	var candidateFiles []string
	var err error
	if opts.NoPrefilter {
		candidateFiles, err = gitListFiles(opts.RepoDir, opts.Paths, opts.Excludes, opts.ExcludeTypical)
	} else {
		candidateFiles, err = gitGrepFiles(opts.RepoDir, pattern, opts.Paths, opts.Excludes, opts.ExcludeTypical)
	}
	if err != nil {
		return nil, nil, err
	}
	candidateFiles = filterPathsByRegex(candidateFiles, opts.PathRegexCompiled)
	if len(candidateFiles) == 0 {
		return nil, nil, nil
	}
	tagsSpec := normalizeTags(tags)

	jobs := make(chan parseJob)
	results := make(chan parseResult)

	workers := opts.Jobs
	if workers < 1 {
		workers = 1
	}
	if workers > 64 {
		workers = 64
	}
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				matches, errs := parseFile(job.path, opts, tagsSpec, allowFallback)
				results <- parseResult{matches: matches, errs: errs}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, path := range candidateFiles {
			select {
			case <-ctx.Done():
				return
			case jobs <- parseJob{path: path}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var all []model.Match
	var errs []ItemError
	for res := range results {
		if len(res.matches) > 0 {
			all = append(all, res.matches...)
		}
		if len(res.errs) > 0 {
			errs = append(errs, res.errs...)
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].File == all[j].File {
			if all[i].Span.StartLine == all[j].Span.StartLine {
				return all[i].Span.StartCol < all[j].Span.StartCol
			}
			return all[i].Span.StartLine < all[j].Span.StartLine
		}
		return all[i].File < all[j].File
	})
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].File == errs[j].File {
			if errs[i].Line == errs[j].Line {
				return errs[i].Stage < errs[j].Stage
			}
			return errs[i].Line < errs[j].Line
		}
		return errs[i].File < errs[j].File
	})
	return all, errs, nil
}

func parseFile(relPath string, opts Options, tags []tagSpec, allowFallback bool) ([]model.Match, []ItemError) {
	full := filepath.Join(opts.RepoDir, relPath)
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, []ItemError{newItemError(relPath, 0, "read", err)}
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, nil
	}
	if !utf8.Valid(data) {
		matches := scanPlainText(relPath, data, tags)
		return matches, nil
	}
	if opts.MaxFileBytes > 0 && len(data) > opts.MaxFileBytes {
		matches := scanPlainText(relPath, data, tags)
		return matches, nil
	}
	info := detect.FromPathAndContent(relPath, data)
	if len(opts.DetectLangs) > 0 && !detect.MatchesLang(info, opts.DetectLangs) {
		if allowFallback {
			return scanPlainText(relPath, data, tags), nil
		}
		return nil, nil
	}
	style, ok := styleForLanguage(detect.NormalizeLangName(info.Name))
	if !ok {
		if allowFallback {
			return scanPlainText(relPath, data, tags), nil
		}
		return nil, nil
	}
	matches := scanWithStyle(relPath, data, tags, info.Name, style, opts.IncludeStrings)
	if allowFallback && len(matches) == 0 {
		fallback := scanPlainText(relPath, data, tags)
		if len(fallback) > 0 {
			return fallback, nil
		}
	}
	return matches, nil
}

func normalizeTags(tags []string) []tagSpec {
	eff := effectiveTags(tags)
	out := make([]tagSpec, 0, len(eff))
	for _, tag := range eff {
		upper := strings.ToUpper(tag)
		out = append(out, tagSpec{raw: tag, upper: upper})
	}
	return out
}

func scanWithStyle(path string, data []byte, tags []tagSpec, lang string, style commentStyle, includeStrings bool) []model.Match {
	if len(data) == 0 {
		return nil
	}
	lineOffsets := computeLineOffsets(data)
	var matches []model.Match

	type blockState struct {
		pattern     blockPattern
		startOffset int
		buffer      bytes.Buffer
	}

	var state *blockState
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// allow very long lines
	buf := make([]byte, 0, 1024*16)
	scanner.Buffer(buf, len(data)+1)
	offset := 0
	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		line := string(lineBytes)
		lineLen := len(lineBytes)
		lineEnd := offset + lineLen
		if state != nil {
			state.buffer.Write(lineBytes)
			state.buffer.WriteByte('\n')
			if idx := strings.Index(state.buffer.String(), state.pattern.end); idx >= 0 {
				content := state.buffer.String()
				endIdx := strings.Index(content, state.pattern.end)
				if endIdx >= 0 {
					inner := content[:endIdx]
					blockMatches := findMatchesInText(path, inner, tags, lang, state.pattern.kind, state.startOffset, lineOffsets)
					matches = append(matches, blockMatches...)
				}
				state = nil
			}
			if lineEnd < len(data) && data[lineEnd] == '\n' {
				offset = lineEnd + 1
			} else {
				offset = lineEnd
			}
			continue
		}

		// block start detection
		started := false
		for _, block := range style.block {
			if idx := indexBlockStart(line, block); idx >= 0 {
				state = &blockState{pattern: block, startOffset: offset + idx + len(block.start)}
				remainder := line[idx+len(block.start):] + "\n"
				state.buffer.WriteString(remainder)
				started = true
				break
			}
		}
		if started {
			if lineEnd < len(data) && data[lineEnd] == '\n' {
				offset = lineEnd + 1
			} else {
				offset = lineEnd
			}
			continue
		}

		// line comments
		for _, prefix := range style.linePrefixes {
			idx := strings.Index(line, prefix)
			if idx >= 0 {
				commentText := line[idx+len(prefix):]
				if mt := findMatchesInText(path, commentText, tags, lang, model.MatchKindComment, offset+idx+len(prefix), lineOffsets); len(mt) > 0 {
					matches = append(matches, mt...)
				}
				break
			}
		}

		if includeStrings {
			for _, delim := range style.stringDelims {
				pos := 0
				for pos < len(line) {
					idx := strings.Index(line[pos:], delim)
					if idx < 0 {
						break
					}
					start := pos + idx
					if len(delim) == 1 && isEscaped(line, start) {
						pos = start + len(delim)
						continue
					}
					contentStart := start + len(delim)
					end := findClosingDelimiter(line, contentStart, delim)
					if end < 0 {
						break
					}
					inner := line[contentStart:end]
					mt := findMatchesInText(path, inner, tags, lang, model.MatchKindString, offset+contentStart, lineOffsets)
					if len(mt) > 0 {
						matches = append(matches, mt...)
					}
					pos = end + len(delim)
				}
			}
		}

		if lineEnd < len(data) && data[lineEnd] == '\n' {
			offset = lineEnd + 1
		} else {
			offset = lineEnd
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, bufio.ErrTooLong) {
		return matches
	}
	if state != nil {
		content := state.buffer.String()
		blockMatches := findMatchesInText(path, content, tags, lang, state.pattern.kind, state.startOffset, lineOffsets)
		matches = append(matches, blockMatches...)
	}
	return matches
}

func indexBlockStart(line string, block blockPattern) int {
	if block.allowIndentedStart {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, block.start) {
			return len(line) - len(trimmed)
		}
		return -1
	}
	return strings.Index(line, block.start)
}

func findClosingDelimiter(line string, start int, delim string) int {
	if len(delim) == 0 {
		return -1
	}
	if len(delim) == 1 {
		target := delim[0]
		for i := start; i < len(line); i++ {
			if line[i] != target {
				continue
			}
			if isEscaped(line, i) {
				continue
			}
			return i
		}
		return -1
	}
	idx := strings.Index(line[start:], delim)
	if idx < 0 {
		return -1
	}
	return start + idx
}

func isEscaped(line string, pos int) bool {
	if pos == 0 {
		return false
	}
	count := 0
	for i := pos - 1; i >= 0; i-- {
		if line[i] != '\\' {
			break
		}
		count++
	}
	return count%2 == 1
}

func scanPlainText(path string, data []byte, tags []tagSpec) []model.Match {
	lineOffsets := computeLineOffsets(data)
	var matches []model.Match
	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 1024*8)
	scanner.Buffer(buf, len(data)+1)
	offset := 0
	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		text := string(lineBytes)
		mt := findMatchesInText(path, text, tags, "", model.MatchKindUnknown, offset, lineOffsets)
		if len(mt) > 0 {
			matches = append(matches, mt...)
		}
		offset += len(lineBytes) + 1
	}
	return matches
}

func findMatchesInText(path string, text string, tags []tagSpec, lang string, kind model.MatchKind, baseOffset int, lineOffsets []int) []model.Match {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	upper := strings.ToUpper(text)
	var hits []struct {
		idx int
		tag tagSpec
	}
	for _, tag := range tags {
		searchFrom := 0
		for {
			pos := strings.Index(upper[searchFrom:], tag.upper)
			if pos < 0 {
				break
			}
			abs := searchFrom + pos
			hits = append(hits, struct {
				idx int
				tag tagSpec
			}{idx: abs, tag: tag})
			searchFrom = abs + len(tag.upper)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].idx < hits[j].idx })
	matches := make([]model.Match, 0, len(hits))
	for _, hit := range hits {
		byteStart := baseOffset + hit.idx
		span := spanFromOffset(byteStart, len(hit.tag.raw), lineOffsets)
		matches = append(matches, model.Match{
			File: path,
			Lang: detect.NormalizeLangName(lang),
			Kind: kind,
			Tag:  hit.tag.upper,
			Text: strings.TrimSpace(text),
			Span: span,
		})
	}
	return matches
}

func spanFromOffset(start, length int, lineOffsets []int) model.Span {
	line, col := lineColFromOffset(start, lineOffsets)
	endLine, endCol := lineColFromOffset(start+length, lineOffsets)
	return model.Span{
		StartLine: line,
		StartCol:  col,
		EndLine:   endLine,
		EndCol:    endCol,
		ByteStart: start,
		ByteEnd:   start + length,
	}
}

func lineColFromOffset(offset int, lineOffsets []int) (line, col int) {
	idx := sort.Search(len(lineOffsets), func(i int) bool { return lineOffsets[i] > offset })
	if idx == 0 {
		return 1, offset + 1
	}
	lineStart := lineOffsets[idx-1]
	return idx, offset - lineStart + 1
}

func computeLineOffsets(data []byte) []int {
	offsets := make([]int, 0, bytes.Count(data, []byte{'\n'})+2)
	offsets = append(offsets, 0)
	for i, b := range data {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	if offsets[len(offsets)-1] != len(data) {
		offsets = append(offsets, len(data))
	}
	return offsets
}

func filterPathsByRegex(paths []string, rx []*regexp.Regexp) []string {
	if len(rx) == 0 {
		return paths
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if matchAny(rx, p) {
			out = append(out, p)
		}
	}
	return out
}

func matchAny(rx []*regexp.Regexp, text string) bool {
	if len(rx) == 0 {
		return true
	}
	for _, r := range rx {
		if r.MatchString(text) {
			return true
		}
	}
	return false
}

func gitGrepMatches(repo, pattern string, includes, excludes []string, typical bool) ([]match, error) {
	pathspecs := buildGrepPathspecs(includes, excludes, typical)
	args := []string{"-c", "core.quotePath=false", "grep", "-nI", "--no-color", "-i", "-E", pattern, "--"}
	args = append(args, pathspecs...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("git grep: %w", err)
	}
	var res []match
	sc := bufio.NewScanner(bytes.NewReader(out))
	buf := make([]byte, 0, 1024*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		loc := reLine.FindStringIndex(line)
		if loc == nil {
			continue
		}
		file := line[:loc[0]]
		lineStr := line[loc[0]+1 : loc[1]-1]
		text := line[loc[1]:]
		n, _ := strconv.Atoi(lineStr)
		file = filepath.ToSlash(file)
		res = append(res, match{file: file, line: n, text: text})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("git grep scan: %w", err)
	}
	return res, nil
}

func gitGrepFiles(repo, pattern string, includes, excludes []string, typical bool) ([]string, error) {
	pathspecs := buildGrepPathspecs(includes, excludes, typical)
	args := []string{"-c", "core.quotePath=false", "grep", "-Ilz", "-i", "-E", pattern, "--"}
	args = append(args, pathspecs...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("git grep -l: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	parts := bytes.Split(out, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		paths = append(paths, filepath.ToSlash(string(p)))
	}
	return paths, nil
}

func gitListFiles(repo string, includes, excludes []string, typical bool) ([]string, error) {
	args := []string{"ls-files", "-z"}
	args = append(args, buildGrepPathspecs(includes, excludes, typical)...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	parts := bytes.Split(out, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		paths = append(paths, filepath.ToSlash(string(p)))
	}
	return paths, nil
}

func expandLineMatches(matches []match, tags []string) []model.Match {
	specs := normalizeTags(tags)
	out := make([]model.Match, 0, len(matches))
	for _, m := range matches {
		text := m.text
		mt := findMatchesInText(m.file, text, specs, "", model.MatchKindUnknown, 0, []int{0})
		if len(mt) == 0 {
			// fallback to original line-level match
			clean := strings.TrimRight(text, "\r\n")
			tag := kindOf(clean, tags)
			if tag == "UNKNOWN" {
				tag = ""
			}
			width := utf8.RuneCountInString(clean)
			if width == 0 {
				width = len(clean)
			}
			if width < 1 {
				width = 1
			}
			out = append(out, model.Match{
				File: m.file,
				Lang: "",
				Kind: model.MatchKindUnknown,
				Tag:  tag,
				Text: strings.TrimSpace(text),
				Span: model.Span{StartLine: m.line, EndLine: m.line, StartCol: 1, EndCol: width, ByteStart: 0, ByteEnd: len(clean)},
			})
			continue
		}
		for i := range mt {
			mt[i].File = m.file
			mt[i].Span.StartLine = m.line
			mt[i].Span.EndLine = m.line
			if mt[i].Span.StartCol <= 0 {
				mt[i].Span.StartCol = 1
			}
			if mt[i].Span.EndCol <= 0 {
				mt[i].Span.EndCol = mt[i].Span.StartCol + len(mt[i].Tag)
			}
		}
		out = append(out, mt...)
	}
	return out
}
