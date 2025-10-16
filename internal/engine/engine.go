package engine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/phyten/todox/internal/model"
	"github.com/phyten/todox/internal/progress"
	"github.com/phyten/todox/internal/textutil"
)

var reLine = regexp.MustCompile(`:(\d+):`) // first :<num>:
var defaultTags = []string{"TODO", "FIXME"}

type match struct {
	file string
	line int
	text string
}

func normalizeSpan(span model.Span) model.Span {
	out := span
	if out.StartLine <= 0 {
		switch {
		case out.EndLine > 0:
			out.StartLine = out.EndLine
		default:
			out.StartLine = 1
		}
	}
	if out.EndLine <= 0 {
		out.EndLine = out.StartLine
	}
	if out.StartCol <= 0 {
		out.StartCol = 1
	}
	if out.EndCol <= 0 || out.EndCol < out.StartCol {
		out.EndCol = out.StartCol
	}
	if out.ByteStart < 0 {
		out.ByteStart = 0
	}
	if out.ByteEnd < out.ByteStart {
		out.ByteEnd = out.ByteStart
	}
	return out
}

// Run は指定されたオプションに従ってリポジトリを走査し、TODO/FIXME の一覧とメタデータを返します。
//
// 成功時には発見した項目と補助情報を保持した Result を返し、
// 途中で発生したエラー情報は Result.Errors に集約されます。
func Run(opts Options) (*Result, error) {
	start := time.Now()
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	} else {
		opts.Now = opts.Now.UTC()
	}
	if opts.Jobs <= 0 {
		opts.Jobs = runtime.NumCPU()
	}
	tags := effectiveTags(opts.Tags)
	var searchTags []string
	switch strings.ToLower(opts.Type) {
	case "todo":
		filtered := filterTagsByType(tags, "TODO")
		if len(filtered) == 0 {
			searchTags = []string{"TODO"}
		} else {
			searchTags = filtered
		}
	case "fixme":
		filtered := filterTagsByType(tags, "FIXME")
		if len(filtered) == 0 {
			searchTags = []string{"FIXME"}
		} else {
			searchTags = filtered
		}
	case "", "both":
		searchTags = tags
	default:
		return nil, fmt.Errorf("invalid --type: %s", opts.Type)
	}

	rx := opts.PathRegexCompiled
	if len(rx) == 0 && len(opts.PathRegex) > 0 {
		compiled, compileErr := CompilePathRegex(opts.PathRegex)
		if compileErr != nil {
			return nil, fmt.Errorf("invalid --path-regex: %w", compileErr)
		}
		rx = compiled
	}
	opts.PathRegexCompiled = rx

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	modelMatches, detectErrs, err := collectMatches(ctx, opts, searchTags)
	if err != nil {
		return nil, err
	}
	if len(modelMatches) == 0 {
		return &Result{Items: nil, HasComment: opts.WithComment, HasMessage: opts.WithMessage, Total: 0, ElapsedMS: msSince(start), Errors: detectErrs, ErrorCount: len(detectErrs)}, nil
	}

	normalized := normalizedTags(tags)
	switch opts.Type {
	case "todo":
		todoTags := normalizedTagsForType(normalized, "TODO")
		modelMatches = filterModelMatchesByTags(modelMatches, todoTags, []string{"TODO"})
	case "fixme":
		fixmeTags := normalizedTagsForType(normalized, "FIXME")
		modelMatches = filterModelMatchesByTags(modelMatches, fixmeTags, []string{"FIXME"})
	}
	if len(modelMatches) == 0 {
		return &Result{Items: nil, HasComment: opts.WithComment, HasMessage: opts.WithMessage, Total: 0, ElapsedMS: msSince(start), Errors: detectErrs, ErrorCount: len(detectErrs)}, nil
	}

	out := make([]Item, len(modelMatches))

	var observers []progress.Observer
	if opts.ProgressObserver != nil {
		observers = append(observers, opts.ProgressObserver)
	}
	if opts.Progress {
		observers = append(observers, progress.NewAutoObserver(os.Stderr))
	}
	observer := progress.NewMultiObserver(observers...)
	estimator := progress.NewEstimator(len(modelMatches), progress.Config{})
	if snap, changed := estimator.Stage(progress.StageAttr); changed {
		observer.Publish(snap)
	}
	var errsMu sync.Mutex
	errs := append([]ItemError(nil), detectErrs...)

	// worker pool
	type job struct {
		idx int
		m   model.Match
	}
	jobs := make(chan job)
	var wg sync.WaitGroup

	var authorRe *regexp.Regexp
	if opts.AuthorRegex != "" {
		authorRe, err = regexp.Compile(opts.AuthorRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid --author regex: %w", err)
		}
	}

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			item, itemErrs := processOne(ctx, opts, j.m)
			if len(itemErrs) > 0 {
				errsMu.Lock()
				errs = append(errs, itemErrs...)
				errsMu.Unlock()
			}
			if authorRe != nil && item.Commit != "" {
				if !authorRe.MatchString(item.Author) && !authorRe.MatchString(item.Email) {
					item.Commit = ""
				}
			}
			out[j.idx] = item
			if snap, notify := estimator.Advance(1); notify {
				observer.Publish(snap)
			}
		}
	}

	nw := opts.Jobs
	if nw < 1 {
		nw = 1
	}
	wg.Add(nw)
	for i := 0; i < nw; i++ {
		go worker()
	}
	for i, m := range modelMatches {
		jobs <- job{idx: i, m: m}
	}
	close(jobs)
	wg.Wait()

	finalSnap := estimator.Complete()
	observer.Publish(finalSnap)
	observer.Done(finalSnap)

	// compact skipped
	final := out[:0]
	for _, it := range out {
		if it.Commit != "" || (it.Author == "(working tree)" && it.Commit == "") {
			final = append(final, it)
		}
	}

	// stable order by file:line
	sort.SliceStable(final, func(i, j int) bool {
		if final[i].File == final[j].File {
			return final[i].Line < final[j].Line
		}
		return final[i].File < final[j].File
	})

	sort.Slice(errs, func(i, j int) bool {
		if errs[i].File == errs[j].File {
			if errs[i].Line == errs[j].Line {
				return errs[i].Stage < errs[j].Stage
			}
			return errs[i].Line < errs[j].Line
		}
		return errs[i].File < errs[j].File
	})

	return &Result{
		Items:      final,
		HasComment: opts.WithComment,
		HasMessage: opts.WithMessage,
		Total:      len(final),
		ElapsedMS:  msSince(start),
		Errors:     errs,
		ErrorCount: len(errs),
	}, nil
}

func newItemError(file string, line int, stage string, err error) ItemError {
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		msg = "unknown error"
	}
	return ItemError{File: file, Line: line, Stage: stage, Message: msg}
}

func processOne(ctx context.Context, opts Options, m model.Match) (Item, []ItemError) {
	span := normalizeSpan(m.Span)
	line := span.StartLine
	it := Item{
		Kind:      m.Tag,
		Tag:       m.Tag,
		Lang:      m.Lang,
		MatchKind: string(m.Kind),
		Text:      m.Text,
		Span:      span,
		File:      m.File,
		Line:      line,
	}
	var sha string
	var errs []ItemError

	if strings.ToLower(opts.Mode) == "first" {
		firstSHA, err := firstCommitForLine(ctx, opts.RepoDir, m.File, line)
		if err != nil {
			errs = append(errs, newItemError(m.File, line, "git log -L", err))
		}
		if firstSHA != "" {
			sha = firstSHA
		} else {
			bl, err := blameSHA(ctx, opts.RepoDir, m.File, line, opts.IgnoreWS)
			if err != nil {
				errs = append(errs, newItemError(m.File, line, "git blame", err))
				return it, errs
			}
			sha = bl
		}
	} else {
		bl, err := blameSHA(ctx, opts.RepoDir, m.File, line, opts.IgnoreWS)
		if err != nil {
			errs = append(errs, newItemError(m.File, line, "git blame", err))
			return it, errs
		}
		sha = bl
	}

	if sha == "" || sha == strings.Repeat("0", 40) {
		it.Author = "(working tree)"
		it.Email = "-"
		it.Date = "(uncommitted)"
		it.Commit = ""
	} else {
		a, e, d, authorTime, s, err := commitMeta(ctx, opts.RepoDir, sha)
		if err != nil {
			errs = append(errs, newItemError(m.File, line, "git show", err))
		}
		it.Author, it.Email, it.Date, it.Commit = a, e, d, sha
		it.AgeDays = ageDays(opts.Now, authorTime)
		if opts.WithMessage {
			it.Message = truncateDisplayWidth(s, effectiveTrunc(opts.TruncMessage, opts.TruncAll))
		}
	}

	if opts.WithComment {
		text := strings.TrimSpace(m.Text)
		comment := extractComment(text, opts.Type, opts.Tags)
		if strings.TrimSpace(comment) == "" {
			comment = m.Tag
		}
		it.Comment = truncateDisplayWidth(comment, effectiveTrunc(opts.TruncComment, opts.TruncAll))
	}

	return it, errs
}

func buildBlameArgs(file string, line int, ignoreWS bool) []string {
	args := []string{"blame"}
	if ignoreWS {
		args = append(args, "-w")
	}
	lineSpec := fmt.Sprintf("%d,%d", line, line)
	return append(args, "--line-porcelain", "-L", lineSpec, "--", file)
}

func blameSHA(ctx context.Context, repo, file string, line int, ignoreWS bool) (string, error) {
	args := buildBlameArgs(file, line, ignoreWS)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// first token of 1st line is SHA
	first := bytes.SplitN(out, []byte("\n"), 2)[0]
	sha := strings.Fields(string(first))
	if len(sha) > 0 {
		return sha[0], nil
	}
	return "", nil
}

func firstCommitForLine(ctx context.Context, repo, file string, line int) (string, error) {
	spec := fmt.Sprintf("%d,%d:%s", line, line, file)
	cmd := exec.CommandContext(ctx, "git", "log", "--reverse", "-L", spec, "--format=%H")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	if sc.Scan() {
		return strings.TrimSpace(sc.Text()), nil
	}
	return "", nil
}

func commitMeta(ctx context.Context, repo, sha string) (author, email, date string, authorTime time.Time, subject string, err error) {
	cmd := exec.CommandContext(ctx, "git", "show", "-s", "--date=iso-strict-local", "--format=%an%x09%ae%x09%ad%x09%at%x09%s", sha)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "-", "-", "-", time.Time{}, "-", fmt.Errorf("git show: %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 5)
	if len(parts) != 5 {
		return "-", "-", "-", time.Time{}, "-", fmt.Errorf("git show unexpected output: %q", strings.TrimSpace(string(out)))
	}
	ts, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return "-", "-", "-", time.Time{}, "-", fmt.Errorf("git show timestamp parse: %w", err)
	}
	return parts[0], parts[1], parts[2], time.Unix(ts, 0).UTC(), parts[4], nil
}

func effectiveTags(tags []string) []string {
	if len(tags) == 0 {
		return append([]string(nil), defaultTags...)
	}
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, raw := range tags {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return append([]string(nil), defaultTags...)
	}
	return out
}

func filterTagsByType(tags []string, target string) []string {
	normalizedTarget := strings.ToUpper(strings.TrimSpace(target))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if strings.ToUpper(strings.TrimSpace(tag)) == normalizedTarget {
			out = append(out, tag)
		}
	}
	return out
}

func patternForTags(tags []string) string {
	effective := effectiveTags(tags)
	if len(effective) == 1 {
		return regexp.QuoteMeta(effective[0])
	}
	escaped := make([]string, 0, len(effective))
	for _, tag := range effective {
		escaped = append(escaped, regexp.QuoteMeta(tag))
	}
	return "(" + strings.Join(escaped, "|") + ")"
}

func normalizedTags(tags []string) []string {
	effective := effectiveTags(tags)
	out := make([]string, 0, len(effective))
	seen := make(map[string]struct{}, len(effective))
	for _, tag := range effective {
		upper := strings.ToUpper(tag)
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		out = append(out, upper)
	}
	return out
}

func normalizedTagsForType(normalized []string, target string) []string {
	want := strings.ToUpper(strings.TrimSpace(target))
	if want == "" {
		return nil
	}
	out := make([]string, 0, len(normalized))
	for _, tag := range normalized {
		if tag == want {
			out = append(out, tag)
		}
	}
	return out
}

func filterModelMatchesByTags(matches []model.Match, include []string, fallback []string) []model.Match {
	tags := include
	if len(tags) == 0 {
		tags = fallback
	}
	if len(tags) == 0 {
		return matches
	}
	wanted := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := strings.ToUpper(strings.TrimSpace(tag))
		if trimmed == "" {
			continue
		}
		wanted[trimmed] = struct{}{}
	}
	if len(wanted) == 0 {
		return matches
	}
	out := matches[:0]
	for _, m := range matches {
		upper := strings.ToUpper(strings.TrimSpace(m.Tag))
		if _, ok := wanted[upper]; ok {
			out = append(out, m)
		}
	}
	return out
}

func kindOf(text string, tags []string) string {
	normalized := normalizedTags(tags)
	upper := strings.ToUpper(text)
	found := make([]string, 0, len(normalized))
	seen := make(map[string]struct{}, len(normalized))
	for _, tag := range normalized {
		if strings.Contains(upper, tag) {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			found = append(found, tag)
		}
	}
	if len(found) == 0 {
		if strings.Contains(upper, "TODO") {
			found = append(found, "TODO")
		}
		if strings.Contains(upper, "FIXME") {
			found = append(found, "FIXME")
		}
	}
	if len(found) == 0 {
		return "UNKNOWN"
	}
	if len(found) == 1 {
		return found[0]
	}
	return strings.Join(found, "|")
}

func extractComment(text, typ string, tags []string) string {
	normalized := normalizedTags(tags)
	var search []string
	switch strings.ToLower(typ) {
	case "todo":
		search = normalizedTagsForType(normalized, "TODO")
	case "fixme":
		search = normalizedTagsForType(normalized, "FIXME")
	default:
		search = normalized
	}
	if len(search) == 0 {
		switch strings.ToLower(typ) {
		case "todo":
			search = []string{"TODO"}
		case "fixme":
			search = []string{"FIXME"}
		default:
			search = []string{"TODO", "FIXME"}
		}
	}
	upper := strings.ToUpper(text)
	pos := -1
	for _, tag := range search {
		idx := strings.Index(upper, tag)
		if idx >= 0 && (pos == -1 || idx < pos) {
			pos = idx
		}
	}
	if pos >= 0 && pos < len(text) {
		return text[pos:]
	}
	return text
}

func truncateDisplayWidth(s string, n int) string {
	if n <= 0 {
		return s
	}
	if textutil.VisibleWidth(s) <= n {
		return s
	}
	if out := textutil.TruncateByWidth(s, n, "…"); out != "" {
		return out
	}
	return textutil.TruncateByWidth(s, n, "")
}

func effectiveTrunc(specific, all int) int {
	if specific > 0 {
		return specific
	}
	return all
}

func ageDays(now, author time.Time) int {
	if author.IsZero() {
		return 0
	}
	diff := int(now.Sub(author).Hours() / 24)
	if diff < 0 {
		return 0
	}
	return diff
}

func msSince(t time.Time) int64 { return time.Since(t).Milliseconds() }
