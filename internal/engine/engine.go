package engine

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/phyten/todox/internal/util"
)

var reLine = regexp.MustCompile(`:(\d+):`) // first :<num>:

type match struct {
	file string
	line int
	text string
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
	grepPat := "(TODO|FIXME)"
	switch strings.ToLower(opts.Type) {
	case "todo":
		grepPat = "TODO"
	case "fixme":
		grepPat = "FIXME"
	case "both":
	default:
		return nil, fmt.Errorf("invalid --type: %s", opts.Type)
	}

	matches, err := gitGrep(opts.RepoDir, grepPat)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return &Result{Items: nil, HasComment: opts.WithComment, HasMessage: opts.WithMessage, Total: 0, ElapsedMS: msSince(start)}, nil
	}

	// filter by TYPE precisely (for lines containing both)
	if opts.Type == "todo" || opts.Type == "fixme" {
		filter := matches[:0]
		for _, m := range matches {
			hasTODO := strings.Contains(m.text, "TODO")
			hasFIX := strings.Contains(m.text, "FIXME")
			if opts.Type == "todo" && hasTODO {
				filter = append(filter, m)
			}
			if opts.Type == "fixme" && hasFIX {
				filter = append(filter, m)
			}
		}
		matches = filter
	}

	out := make([]Item, len(matches))
	prog := util.NewProgress(len(matches), opts.Progress)
	var errsMu sync.Mutex
	var errs []ItemError

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// worker pool
	type job struct {
		idx int
		m   match
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
			// author filter (name or email)
			if authorRe != nil && item.Commit != "" {
				if !authorRe.MatchString(item.Author) && !authorRe.MatchString(item.Email) {
					// mark as skipped by empty commit
					item.Commit = ""
				}
			}
			out[j.idx] = item
			prog.Advance()
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
	for i, m := range matches {
		jobs <- job{idx: i, m: m}
	}
	close(jobs)
	wg.Wait()
	prog.Done()

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

func processOne(ctx context.Context, opts Options, m match) (Item, []ItemError) {
	it := Item{
		Kind: kindOf(m.text),
		File: m.file,
		Line: m.line,
	}
	var sha string
	var errs []ItemError

	if strings.ToLower(opts.Mode) == "first" {
		firstSHA, err := firstCommitForLine(ctx, opts.RepoDir, m.file, m.line)
		if err != nil {
			errs = append(errs, newItemError(m.file, m.line, "git log -L", err))
		}
		if firstSHA != "" {
			sha = firstSHA
		} else {
			bl, err := blameSHA(ctx, opts.RepoDir, m.file, m.line, opts.IgnoreWS)
			if err != nil {
				errs = append(errs, newItemError(m.file, m.line, "git blame", err))
				return it, errs
			}
			sha = bl
		}
	} else {
		bl, err := blameSHA(ctx, opts.RepoDir, m.file, m.line, opts.IgnoreWS)
		if err != nil {
			errs = append(errs, newItemError(m.file, m.line, "git blame", err))
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
			errs = append(errs, newItemError(m.file, m.line, "git show", err))
		}
		it.Author, it.Email, it.Date, it.Commit = a, e, d, sha
		it.AgeDays = ageDays(opts.Now, authorTime)
		if opts.WithMessage {
			it.Message = truncateRunes(s, effectiveTrunc(opts.TruncMessage, opts.TruncAll))
		}
	}

	if opts.WithComment {
		cr := extractComment(m.text, opts.Type)
		it.Comment = truncateRunes(cr, effectiveTrunc(opts.TruncComment, opts.TruncAll))
	}

	return it, errs
}

func gitGrep(repo, pattern string) ([]match, error) {
	cmd := exec.Command("git", "-c", "core.quotePath=false", "grep", "-nI", "--no-color", "-E", pattern, "--", ".")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		// exit code 1 means "no matches"
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
		// normalize path to repo-relative
		file = filepath.ToSlash(file)
		res = append(res, match{file: file, line: n, text: text})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("git grep scan: %w", err)
	}
	return res, nil
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

func kindOf(text string) string {
	hasTODO := strings.Contains(text, "TODO")
	hasFIX := strings.Contains(text, "FIXME")
	switch {
	case hasTODO && hasFIX:
		return "TODO|FIXME"
	case hasTODO:
		return "TODO"
	case hasFIX:
		return "FIXME"
	default:
		return "UNKNOWN"
	}
}

func extractComment(text, typ string) string {
	iT := strings.Index(text, "TODO")
	iF := strings.Index(text, "FIXME")
	pos := -1
	switch strings.ToLower(typ) {
	case "todo":
		pos = iT
	case "fixme":
		pos = iF
	default:
		switch {
		case iT >= 0 && iF >= 0:
			if iT < iF {
				pos = iT
			} else {
				pos = iF
			}
		case iT >= 0:
			pos = iT
		case iF >= 0:
			pos = iF
		}
	}
	if pos < 0 {
		return text
	}
	return text[pos:]
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	rs := []rune(s)
	if n <= 1 {
		return "…"
	}
	return string(rs[:n-1]) + "…"
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
