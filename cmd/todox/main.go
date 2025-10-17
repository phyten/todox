package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/phyten/todox/internal/config"
	"github.com/phyten/todox/internal/engine"
	engineopts "github.com/phyten/todox/internal/engine/opts"
	"github.com/phyten/todox/internal/execx"
	"github.com/phyten/todox/internal/gitremote"
	ghclient "github.com/phyten/todox/internal/host/github"
	"github.com/phyten/todox/internal/link"
	"github.com/phyten/todox/internal/output"
	"github.com/phyten/todox/internal/progress"
	"github.com/phyten/todox/internal/termcolor"
	"github.com/phyten/todox/internal/textutil"
	"github.com/phyten/todox/internal/web"
)

var (
	debugProgressDrops     = envBool("TODOX_DEBUG_PROGRESS")
	warnDeprecatedLinkOnce sync.Once
)

func main() {
	log.SetFlags(0)
	envEA := strings.TrimSpace(os.Getenv("TODOX_EASTASIAN"))
	if envEA == "1" || strings.EqualFold(envEA, "true") {
		runewidth.EastAsianWidth = true
	} else {
		runewidth.EastAsianWidth = false
	}
	runewidth.DefaultCondition = runewidth.NewCondition()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			serveCmd(os.Args[2:])
			return
		case "pr":
			prCmd(os.Args[2:])
			return
		}
	}
	scanCmd(os.Args[1:])
}

type scanConfig struct {
	opts        engine.Options
	output      string
	withComment bool
	withMessage bool
	withAge     bool
	withCommit  bool
	withPRs     bool
	sortKey     string
	fields      string
	showHelp    bool
	helpLang    string
	colorMode   termcolor.ColorMode
	prState     string
	prLimit     int
	prPrefer    string
}

type usageError struct {
	err error
}

func (u *usageError) Error() string {
	if u == nil || u.err == nil {
		return ""
	}
	return u.err.Error()
}

func (u *usageError) Unwrap() error {
	if u == nil {
		return nil
	}
	return u.err
}

type multiFlag struct {
	values  []string
	changed bool
}

func (m *multiFlag) String() string {
	if m == nil {
		return ""
	}
	return strings.Join(m.values, ",")
}

func (m *multiFlag) Set(value string) error {
	// 空文字は明示的なクリアとして扱い、既存値を全て削除する
	if value == "" {
		m.values = m.values[:0]
		m.changed = true
		return nil
	}
	for _, piece := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(piece)
		if trimmed == "" {
			continue
		}
		m.values = append(m.values, trimmed)
	}
	m.changed = true
	return nil
}

func (m *multiFlag) Slice() []string {
	if m == nil {
		return nil
	}
	out := make([]string, len(m.values))
	copy(out, m.values)
	return out
}

func (m *multiFlag) WasSet() bool {
	if m == nil {
		return false
	}
	return m.changed
}

func warnDeprecatedWithLink() {
	if deprecatedWarningsSuppressed() {
		return
	}
	warnDeprecatedLinkOnce.Do(func() {
		fmt.Fprintln(os.Stderr, "todox: --with-link is deprecated; use --with-commit-link instead")
	})
}

func deprecatedWarningsSuppressed() bool {
	env := strings.TrimSpace(os.Getenv("TODOX_NO_DEPRECATION_WARNINGS"))
	switch strings.ToLower(env) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseScanArgs(args []string, envLang string) (scanConfig, error) {
	cfg := scanConfig{helpLang: strings.ToLower(envLang)}
	if cfg.helpLang == "" {
		cfg.helpLang = "en"
	}

	envCfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return cfg, &usageError{err: err}
	}

	repoGuess := guessRepoDir(args, envCfg)
	configPath, _, err := config.Find(repoGuess, os.Getenv("TODOX_CONFIG"), os.Getenv("XDG_CONFIG_HOME"), os.Getenv("HOME"))
	if err != nil {
		return cfg, err
	}
	fileCfg, err := config.Load(configPath)
	if err != nil {
		return cfg, err
	}

	baseOpts := engineopts.Defaults(repoGuess)
	baseEngine := config.EngineSettingsFromOptions(baseOpts)
	baseUI := config.DefaultUISettings()

	defaultsEngine := config.MergeEngine(baseEngine, fileCfg.Engine, envCfg.Engine)
	defaultsUI := config.MergeUI(baseUI, fileCfg.UI, envCfg.UI)
	defaultsUI, err = config.NormalizeUI(defaultsUI)
	if err != nil {
		return cfg, &usageError{err: err}
	}
	normalizedOutput, err := engineopts.NormalizeOutput(defaultsEngine.Output)
	if err != nil {
		return cfg, &usageError{err: err}
	}
	defaultsEngine.Output = normalizedOutput

	fs := flag.NewFlagSet("todox", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	typ := fs.String("type", defaultsEngine.Type, "todo|fixme|both")
	mode := fs.String("mode", defaultsEngine.Mode, "last|first")
	detect := fs.String("detect", defaultsEngine.Detect, "detection engine: auto|parse|regex")
	author := fs.String("author", defaultsEngine.Author, "filter by author name/email (regexp)")
	outputFmt := fs.String("output", defaultsEngine.Output, "table|tsv|json|csv|ndjson|md")
	colorMode := fs.String("color", defaultsEngine.Color, "color output for tables: auto|always|never")
	withComment := fs.Bool("with-comment", defaultsEngine.WithComment, "show line text (from TODO/FIXME)")
	withMessage := fs.Bool("with-message", defaultsEngine.WithMessage, "show commit subject (1st line)")
	includeStrings := fs.Bool("include-strings", defaultsEngine.IncludeStrings, "include string literals in detection")
	commentsOnly := fs.Bool("comments-only", !defaultsEngine.IncludeStrings, "alias of --no-strings")
	noStrings := fs.Bool("no-strings", !defaultsEngine.IncludeStrings, "alias of --comments-only")
	withAge := fs.Bool("with-age", defaultsUI.WithAge, "show AGE column in tabular outputs (table/tsv/csv/md)")
	withCommitLink := fs.Bool("with-commit-link", defaultsUI.WithCommitLink, "show URL column (GitHub blob link)")
	withLinkAlias := fs.Bool("with-link", false, "DEPRECATED: alias of --with-commit-link")
	withPRLinks := fs.Bool("with-pr-links", defaultsUI.WithPRLinks, "include pull request links (table/tsv/csv/md/json)")
	prState := fs.String("pr-state", defaultsUI.PRState, "filter PRs by state: all|open|closed|merged")
	prLimit := fs.Int("pr-limit", defaultsUI.PRLimit, "maximum PRs to include per item (1-20)")
	prPrefer := fs.String("pr-prefer", defaultsUI.PRPrefer, "state preference when ordering PRs: open|merged|closed|none")
	fields := fs.String("fields", defaultsUI.Fields, "comma-separated columns for tabular outputs (table/tsv/csv/md; overrides --with-*)")
	full := fs.Bool("full", false, "shortcut for --with-comment --with-message (with default truncate)")
	withSnippet := fs.Bool("with-snippet", false, "alias of --with-comment")
	truncAll := fs.Int("truncate", defaultsEngine.TruncAll, "truncate comment/message to N runes (0=unlimited)")
	truncComment := fs.Int("truncate-comment", defaultsEngine.TruncComment, "truncate comment only (0=unlimited)")
	truncMessage := fs.Int("truncate-message", defaultsEngine.TruncMessage, "truncate message only (0=unlimited)")
	noIgnoreWS := fs.Bool("no-ignore-ws", !defaultsEngine.IgnoreWS, "include whitespace-only changes in blame")
	noProgress := fs.Bool("no-progress", false, "disable progress/ETA")
	forceProg := fs.Bool("progress", false, "force progress even when piped")
	sortKey := fs.String("sort", defaultsUI.Sort, "sort order (e.g. author,-date; default: file,line)")
	lang := fs.String("lang", "", "help language (en|ja)")
	jobs := fs.Int("jobs", defaultsEngine.Jobs, "max parallel workers")
	repo := fs.String("repo", defaultsEngine.Repo, "repo root (default: current dir)")
	var paths multiFlag
	var excludes multiFlag
	var pathRegex multiFlag
	var detectLangs multiFlag
	var tagList multiFlag
	fs.Var(&paths, "path", "limit search to given pathspec(s). repeatable / CSV")
	fs.Var(&excludes, "exclude", "exclude pathspec/glob(s). repeatable / CSV")
	fs.Var(&pathRegex, "path-regex", "post-filter files by Go regexp (OR). repeatable / CSV")
	fs.Var(&detectLangs, "detect-langs", "limit parser-based detection to languages. repeatable / CSV")
	fs.Var(&detectLangs, "detect-lang", "alias of --detect-langs")
	fs.Var(&tagList, "tag", "add or replace detection tags. repeatable / CSV")
	fs.Var(&tagList, "tags", "alias of --tag")
	excludeTypical := fs.Bool("exclude-typical", defaultsEngine.ExcludeTypical, "apply typical excludes (vendor/**, node_modules/**, dist/**, build/**, target/**, *.min.*)")
	maxFileBytes := fs.Int("max-file-bytes", defaultsEngine.MaxFileBytes, "skip parser detection for files larger than N bytes (0=unlimited)")
	noPrefilter := fs.Bool("no-prefilter", defaultsEngine.NoPrefilter, "disable git grep prefilter before parsing")

	shortMap := map[string]string{
		"-t": "--type",
		"-m": "--mode",
		"-a": "--author",
		"-o": "--output",
	}

	normalized := make([]string, 0, len(args))
	helpLangSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			cfg.showHelp = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				cfg.helpLang = strings.ToLower(args[i+1])
				helpLangSet = true
				i++
			}
		case strings.HasPrefix(arg, "--help="):
			cfg.showHelp = true
			cfg.helpLang = strings.ToLower(strings.TrimPrefix(arg, "--help="))
			helpLangSet = true
		case arg == "--help-ja":
			cfg.showHelp = true
			cfg.helpLang = "ja"
			helpLangSet = true
		case arg == "--help-en":
			cfg.showHelp = true
			cfg.helpLang = "en"
			helpLangSet = true
		default:
			if long, ok := shortMap[arg]; ok {
				normalized = append(normalized, long)
				continue
			}
			matched := false
			for short, long := range shortMap {
				if strings.HasPrefix(arg, short+"=") {
					normalized = append(normalized, long+"="+arg[len(short)+1:])
					matched = true
					break
				}
				if strings.HasPrefix(arg, short) && len(arg) > len(short) {
					normalized = append(normalized, long, arg[len(short):])
					matched = true
					break
				}
			}
			if matched {
				continue
			}
			normalized = append(normalized, arg)
		}
	}

	parseBoolFlag := func(arg, name string) (value bool, matched bool, ok bool) {
		if arg == name {
			return true, true, true
		}
		prefix := name + "="
		if strings.HasPrefix(arg, prefix) {
			trimmed := strings.TrimSpace(arg[len(prefix):])
			parsed, parseErr := engineopts.ParseBool(trimmed, name)
			if parseErr != nil {
				return false, true, false
			}
			return parsed, true, true
		}
		return false, false, true
	}
	var finalInclude *bool
	lastIdx := -1
	setFinalInclude := func(idx int, include bool) {
		if idx > lastIdx {
			v := include
			finalInclude = &v
			lastIdx = idx
		}
	}
	for idx, arg := range normalized {
		if val, matched, ok := parseBoolFlag(arg, "--include-strings"); matched {
			if ok {
				setFinalInclude(idx, val)
			}
			continue
		}
		if val, matched, ok := parseBoolFlag(arg, "--comments-only"); matched {
			if ok {
				setFinalInclude(idx, !val)
			}
			continue
		}
		if val, matched, ok := parseBoolFlag(arg, "--no-strings"); matched {
			if ok {
				setFinalInclude(idx, !val)
			}
			continue
		}
	}

	if parseErr := fs.Parse(normalized); parseErr != nil {
		if errors.Is(parseErr, flag.ErrHelp) {
			cfg.showHelp = true
			return cfg, nil
		}
		return cfg, parseErr
	}

	flagWasSet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		flagWasSet[f.Name] = true
	})

	includeStringsChanged := false
	if finalInclude != nil {
		*includeStrings = *finalInclude
		includeStringsChanged = true
	} else {
		includeStringsChanged = flagWasSet["include-strings"]
		if flagWasSet["comments-only"] {
			if *commentsOnly {
				*includeStrings = false
			} else if !includeStringsChanged {
				*includeStrings = true
			}
			includeStringsChanged = true
		}
		if flagWasSet["no-strings"] {
			if *noStrings {
				*includeStrings = false
			} else if !includeStringsChanged {
				*includeStrings = true
			}
			includeStringsChanged = true
		}
	}

	if *full {
		if !*withComment {
			*withComment = true
		}
		if !*withMessage {
			*withMessage = true
		}
		if *truncAll == 0 && *truncComment == 0 && *truncMessage == 0 {
			*truncAll = 120
		}
	}

	if *withSnippet {
		*withComment = true
	}

	if *lang != "" && !helpLangSet {
		cfg.helpLang = strings.ToLower(*lang)
	}
	if cfg.helpLang == "" {
		cfg.helpLang = "en"
	}

	truncAllChanged := flagWasSet["truncate"]
	truncCommentChanged := flagWasSet["truncate-comment"]
	truncMessageChanged := flagWasSet["truncate-message"]
	if *full && *truncAll == 120 && !truncAllChanged {
		truncAllChanged = true
	}

	withCommentChanged := flagWasSet["with-comment"]
	withMessageChanged := flagWasSet["with-message"]
	if *withSnippet {
		*withComment = true
		withCommentChanged = true
	}
	if *full {
		withCommentChanged = true
		withMessageChanged = true
	}

	var flagEngine config.EngineConfig
	if flagWasSet["type"] {
		v := *typ
		flagEngine.Type = &v
	}
	if flagWasSet["mode"] {
		v := *mode
		flagEngine.Mode = &v
	}
	if flagWasSet["detect"] {
		v := *detect
		flagEngine.Detect = &v
	}
	if flagWasSet["author"] {
		v := *author
		flagEngine.Author = &v
	}
	if paths.WasSet() {
		vals := paths.Slice()
		flagEngine.Paths = &vals
	}
	if excludes.WasSet() {
		vals := excludes.Slice()
		flagEngine.Excludes = &vals
	}
	if pathRegex.WasSet() {
		vals := pathRegex.Slice()
		flagEngine.PathRegex = &vals
	}
	if detectLangs.WasSet() {
		vals := detectLangs.Slice()
		flagEngine.DetectLangs = &vals
	}
	if tagList.WasSet() {
		vals := tagList.Slice()
		flagEngine.Tags = &vals
	}
	if flagWasSet["exclude-typical"] {
		v := *excludeTypical
		flagEngine.ExcludeTypical = &v
	}
	if withCommentChanged {
		v := *withComment
		flagEngine.WithComment = &v
	}
	if withMessageChanged {
		v := *withMessage
		flagEngine.WithMessage = &v
	}
	if truncAllChanged {
		v := *truncAll
		flagEngine.TruncAll = &v
	}
	if truncCommentChanged {
		v := *truncComment
		flagEngine.TruncComment = &v
	}
	if truncMessageChanged {
		v := *truncMessage
		flagEngine.TruncMessage = &v
	}
	if flagWasSet["no-ignore-ws"] {
		v := !*noIgnoreWS
		flagEngine.IgnoreWS = &v
	}
	if includeStringsChanged {
		v := *includeStrings
		flagEngine.IncludeStrings = &v
	}
	if flagWasSet["max-file-bytes"] {
		v := *maxFileBytes
		flagEngine.MaxFileBytes = &v
	}
	if flagWasSet["jobs"] {
		v := *jobs
		flagEngine.Jobs = &v
	}
	if flagWasSet["repo"] {
		v := *repo
		flagEngine.Repo = &v
	}
	if flagWasSet["output"] {
		v := *outputFmt
		flagEngine.Output = &v
	}
	if flagWasSet["color"] {
		v := *colorMode
		flagEngine.Color = &v
	}
	if flagWasSet["no-prefilter"] {
		v := *noPrefilter
		flagEngine.NoPrefilter = &v
	}

	var flagUI config.UIConfig
	if flagWasSet["with-age"] {
		v := *withAge
		flagUI.WithAge = &v
	}
	if flagWasSet["with-commit-link"] {
		v := *withCommitLink
		flagUI.WithCommitLink = &v
	}
	if flagWasSet["with-link"] && !flagWasSet["with-commit-link"] {
		v := *withLinkAlias
		flagUI.WithCommitLink = &v
	}
	if flagWasSet["with-pr-links"] {
		v := *withPRLinks
		flagUI.WithPRLinks = &v
	}
	if flagWasSet["pr-state"] {
		v := *prState
		flagUI.PRState = &v
	}
	if flagWasSet["pr-limit"] {
		v := *prLimit
		flagUI.PRLimit = &v
	}
	if flagWasSet["pr-prefer"] {
		v := *prPrefer
		flagUI.PRPrefer = &v
	}
	if flagWasSet["fields"] {
		v := *fields
		flagUI.Fields = &v
	}
	if flagWasSet["sort"] {
		v := *sortKey
		flagUI.Sort = &v
	}

	finalEngine := config.MergeEngine(defaultsEngine, flagEngine)
	finalUI := config.MergeUI(defaultsUI, flagUI)
	finalUI, err = config.NormalizeUI(finalUI)
	if err != nil {
		return cfg, &usageError{err: err}
	}

	normalizedOutput, err = engineopts.NormalizeOutput(finalEngine.Output)
	if err != nil {
		return cfg, &usageError{err: err}
	}
	finalEngine.Output = normalizedOutput

	opts := engineopts.Defaults(finalEngine.Repo)
	finalEngine.ApplyToOptions(&opts)
	opts.Progress = progress.ShouldShowProgress(*forceProg, *noProgress)

	cfg.opts = opts
	cfg.output = finalEngine.Output
	cfg.withComment = finalEngine.WithComment
	cfg.withMessage = finalEngine.WithMessage
	cfg.withAge = finalUI.WithAge
	cfg.withCommit = finalUI.WithCommitLink
	cfg.withPRs = finalUI.WithPRLinks
	cfg.sortKey = finalUI.Sort
	cfg.fields = finalUI.Fields
	cfg.prState = finalUI.PRState
	cfg.prLimit = finalUI.PRLimit
	cfg.prPrefer = finalUI.PRPrefer

	if flagWasSet["with-link"] {
		warnDeprecatedWithLink()
	}

	parsedMode, err := termcolor.ParseMode(finalEngine.Color)
	if err != nil {
		return cfg, &usageError{err: err}
	}
	cfg.colorMode = parsedMode

	if err := engineopts.NormalizeAndValidate(&cfg.opts); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func guessRepoDir(args []string, envCfg config.Config) string {
	repo := "."
	if envCfg.Engine.Repo != nil {
		trimmed := strings.TrimSpace(*envCfg.Engine.Repo)
		if trimmed != "" {
			repo = trimmed
		}
	}
	if v, ok := findFlagValue(args, "--repo"); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			repo = trimmed
		}
	}
	return repo
}

func writeJSONResult(w io.Writer, res *engine.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(res)
}

func findFlagValue(args []string, name string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == name {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true
		}
		if strings.HasPrefix(arg, name+"=") {
			return arg[len(name)+1:], true
		}
	}
	return "", false
}

func scanCmd(args []string) {
	envLang := os.Getenv("GIT_TODO_AUTHORS_LANG")
	if envLang == "" {
		envLang = os.Getenv("GTA_LANG")
	}

	cfg, err := parseScanArgs(args, envLang)
	if err != nil {
		var uerr *usageError
		if errors.As(err, &uerr) {
			fmt.Fprintf(os.Stderr, "todox: %s\n\n", uerr.Error())
			printHelp(cfg.helpLang)
			os.Exit(2)
		}
		log.Fatal(err)
	}

	if cfg.showHelp {
		printHelp(cfg.helpLang)
		return
	}

	fieldSel, err := output.ResolveFields(cfg.fields, cfg.withComment, cfg.withMessage, cfg.withAge, cfg.withCommit, cfg.withPRs)
	if err != nil {
		log.Fatal(err)
	}

	sortSpec, err := ParseSortSpec(cfg.sortKey)
	if err != nil {
		log.Fatal(err)
	}

	cfg.opts.WithComment = fieldSel.NeedComment
	cfg.opts.WithMessage = fieldSel.NeedMessage

	if err = engineopts.NormalizeAndValidate(&cfg.opts); err != nil {
		log.Fatal(err)
	}

	runner := execx.DefaultRunner()

	var obs progress.Observer
	if cfg.opts.Progress {
		baseObs := progress.NewAutoObserver(os.Stderr)
		obs = baseObs
		if fieldSel.NeedPRs {
			cfg.opts.ProgressObserver = suppressDoneObserver{base: baseObs}
		} else {
			cfg.opts.ProgressObserver = baseObs
		}
		cfg.opts.Progress = false
	} else {
		obs = cfg.opts.ProgressObserver
	}

	res, err := engine.Run(cfg.opts)
	if err != nil {
		log.Fatal(err)
	}

	ApplySort(res.Items, sortSpec)
	res.HasComment = fieldSel.ShowComment
	res.HasMessage = fieldSel.ShowMessage
	res.HasAge = fieldSel.ShowAge

	ctx := context.Background()
	var remoteCache remoteInfoCache
	_ = applyLinkColumn(ctx, runner, cfg.opts.RepoDir, &remoteCache, res, fieldSel)
	prStart := time.Time{}
	if fieldSel.NeedPRs {
		prStart = time.Now()
	}
	_ = applyPRColumns(ctx, runner, cfg.opts.RepoDir, &remoteCache, res, fieldSel, prOptions{
		State:  cfg.prState,
		Limit:  cfg.prLimit,
		Prefer: cfg.prPrefer,
		Jobs:   cfg.opts.Jobs,
	}, obs)
	if !prStart.IsZero() {
		res.ElapsedMS += time.Since(prStart).Milliseconds()
	}

	switch strings.ToLower(cfg.output) {
	case "json":
		// NOTE: JSON は機械可読フォーマットのため常に非カラー。--color の指定は無視する。
		if err := writeJSONResult(os.Stdout, res); err != nil {
			log.Fatal(err)
		}
	case "csv":
		if err := output.WriteCSV(os.Stdout, res.Items, fieldSel); err != nil {
			log.Fatal(err)
		}
	case "tsv":
		// NOTE: TSV もターミナル以外で扱われることが多いため常に非カラー。--color の指定は無視する。
		printTSV(res, fieldSel)
	case "ndjson":
		if err := output.WriteNDJSON(os.Stdout, res.Items); err != nil {
			log.Fatal(err)
		}
	case "md":
		if err := output.WriteMarkdownTable(os.Stdout, res.Items, fieldSel); err != nil {
			log.Fatal(err)
		}
	default: // table
		envMap := toEnvMap(os.Environ())
		profile := termcolor.DetectProfile(envMap)
		mode := cfg.colorMode
		enabled := false
		switch mode {
		case termcolor.ModeAlways, termcolor.ModeNever:
			enabled = termcolor.Enabled(mode, os.Stdout)
		default:
			auto := termcolor.DetectMode(os.Stdout, envMap)
			enabled = termcolor.Enabled(auto, os.Stdout)
		}
		printTable(res, fieldSel, tableColorConfig{enabled: enabled, profile: profile})
	}

	if res.ErrorCount > 0 {
		reportErrors(res)
		os.Exit(2)
	}
}

func printHelp(lang string) {
	switch strings.ToLower(lang) {
	case "ja", "ja_jp", "ja-jp":
		fmt.Print(helpJapanese)
	default:
		fmt.Print(helpEnglish)
	}
}

const helpEnglish = `todox — Find who wrote TODO/FIXME lines in a Git repo.

Usage:
  todox [options]

Search & attribution:
  -t, --type {todo|fixme|both}   Search target (default: both)
  -m, --mode {last|first}        last: last modifier via blame (fast)
                                 first: first introducer via 'git log -L' (slow)
  -a, --author REGEX             Filter by author name or email (extended regex)
      --path LIST               Limit search to pathspec(s) (repeatable / CSV)
      --exclude LIST            Exclude pathspec/glob(s) (repeatable / CSV)
      --path-regex REGEXP       Post-filter file paths by Go regexp (OR across entries)
      --exclude-typical         Apply typical excludes: vendor/**, node_modules/**, dist/**, build/**, target/**, *.min.*
      --detect {auto|parse|regex}
                               Detection engine (default: auto)
      --detect-langs LIST       Limit parser-based detection to languages (repeatable / CSV)
      --tags LIST               Override detection tags (repeatable / CSV)
      --include-strings         Include string literals (default)
      --comments-only           Scan comments only (alias of --no-strings)
      --no-strings              Alias of --comments-only
                               When combining --include-strings/--comments-only/--no-strings,
                               the last flag wins.
      --max-file-bytes N        Skip parser detection above N bytes (0 = unlimited)
      --no-prefilter            Disable git grep prefilter prior to parsing

Output:
  -o, --output {table|tsv|json|csv|ndjson|md}  Output format (default: table)
      --color {auto|always|never} Colorize table output (default: auto)
      --fields LIST             Columns for tabular outputs (table/tsv/csv/md; comma-separated)
                               Available columns: type, tag, kind, lang, author, email,
                               date, age, commit, location, text, span, comment, message,
                               url/commit_url, pr/prs/pr_urls
                               type reports the normalized tag (TODO/FIXME); kind reports
                               where the match came from (comment/string/heredoc). Include
                               comment/message explicitly when overriding defaults.

Extra columns (hidden by default):
      --full                     Show both COMMENT and MESSAGE columns
      --with-comment             Show COMMENT (line text trimmed to start at TODO/FIXME)
      --with-message             Show MESSAGE (commit subject = 1st line)
      --with-snippet             Alias of --with-comment (backward compatible)
      --with-age                 Show AGE (days since author date) in tabular outputs
      --with-commit-link         Show URL column with GitHub blob links
      --with-link                Deprecated alias of --with-commit-link
      --with-pr-links            Include pull requests containing each commit
      --pr-state {all|open|closed|merged}
                                Filter PRs by state (default: all)
      --pr-limit N              Limit PRs per item (1-20, default: 3)
      --pr-prefer {open|merged|closed|none}
                                Prioritize states when ordering PRs (default: open)

Truncation (applies to COMMENT / MESSAGE only):
      --truncate N               Truncate both to N chars (0 = unlimited)
      --truncate-comment N       Truncate comment to N chars (0 = unlimited)
      --truncate-message N       Truncate message to N chars (0 = unlimited)
                                 Tip: --full alone defaults to 120 chars for both.

Sorting:
      --sort KEYS                Sort order (e.g. --sort -age,file,line)
                                 Keys: age, date, author, email, type, file, line, commit, location

Blame / progress:
      --no-ignore-ws             Do not pass -w to git blame (whitespace changes count)
      --no-progress              Do not show progress/ETA
      --progress                 Force progress even when piped

Help / language:
  -h, --help [en|ja]             Show help in English (default) or Japanese
      --help=ja                  Same as -h ja
      --help-ja                  Same as -h ja
      --lang {en|ja}             Language for help (e.g. --lang ja -h)
Environment:
      GTA_LANG=ja                Default help language (also: GIT_TODO_AUTHORS_LANG)
      TODOX_NO_DEPRECATION_WARNINGS=1
                                   Suppress deprecated alias warnings (useful in CI)
      TODOX_GH_JOBS=N            Limit PR fetching workers (1-32, default min(jobs,32))
      GH_TOKEN / GITHUB_TOKEN    Authenticate GitHub REST calls when gh CLI is unavailable
      NO_COLOR=1                 Disable colors even in auto mode
      CLICOLOR=0                 Disable colors when auto-detected
      CLICOLOR_FORCE!=0          Force colors even when piped (any value other than "0")
      FORCE_COLOR!=0             Same as CLICOLOR_FORCE
      TERM=dumb                  Disable colors regardless of auto detection
      auto mode checks stdout only; stderr TTY is ignored

Examples:
  1) Show last author for all TODO/FIXME:
       todox

  2) Show first introducer (who wrote the TODO at first):
       todox -m first

  3) Filter by author (name or email, regex):
       todox -a 'Alice|alice@example.com'

  4) Only TODO (not FIXME):
       todox -t todo

  5) Show the TODO line and the commit message, both trimmed:
       todox --full                 # defaults to 120 chars each
       todox --full --truncate 80   # 80 chars for both

  6) Different truncate per field (comment 60, message unlimited):
       todox --full --truncate-comment 60 --truncate-message 0

GitHub helpers:
  todox pr find --commit <sha>    List pull requests containing the commit
  todox pr open --commit <sha>    Open the first matching pull request in a browser
  todox pr create --commit <sha>  Create a pull request via gh CLI (see todox pr create --help)

  7) Machine-friendly TSV:
       todox --full -o tsv > todo_full.tsv

  8) Progress control:
       todox --no-progress
       todox --progress | head   # force progress even when piped

  9) Include whitespace-only changes in blame:
       todox --no-ignore-ws
`

const helpJapanese = `todox — リポジトリ内の TODO / FIXME の「誰が書いたか」を特定するツール。

使い方:
  todox [options]

検索と属性付け:
  -t, --type {todo|fixme|both}   検索対象（既定: both）
  -m, --mode {last|first}        last : その行を最後に変更した人（git blame で高速）
                                 first: その TODO/FIXME を最初に入れた人（git log -L で低速）
  -a, --author REGEX             作者名またはメールを正規表現でフィルタ
      --path LIST               検索対象の pathspec を指定（繰り返し/カンマ区切り）
      --exclude LIST            除外する pathspec/glob（繰り返し/カンマ区切り）
      --path-regex REGEXP       ファイルパスを Go の正規表現で後段フィルタ（OR 条件）
      --exclude-typical         典型的な除外セットを適用（vendor/**, node_modules/**, dist/**, build/**, target/**, *.min.*）
      --detect {auto|parse|regex}
                                検出エンジン（既定: auto）
      --detect-langs LIST       構文解析対象の言語を限定（繰り返し/カンマ区切り）
      --tags LIST               検出タグを上書き（繰り返し/カンマ区切り）
      --include-strings         文字列リテラルも対象（既定で有効）
      --comments-only           コメントのみを対象（--no-strings の別名）
      --no-strings              --comments-only の別名
                               これらを併用した場合は最後に指定したフラグが優先されます。
      --max-file-bytes N        N バイト超のファイルは構文解析をスキップ（0=無制限）
      --no-prefilter            git grep による事前フィルタを無効化

出力:
  -o, --output {table|tsv|json|csv|ndjson|md}  出力形式（既定: table）
      --color {auto|always|never} 表形式に色付け（既定: auto）
      --fields LIST             表形式（table/tsv/csv/md）の列を指定（カンマ区切り。--with-* より優先）
                               指定可能な列: type, tag, kind, lang, author, email, date,
                               age, commit, location, text, span, comment, message,
                               url/commit_url, pr/prs/pr_urls
                               type は正規化タグ（TODO/FIXME など）、kind は検出元
                               （comment/string/heredoc 等）を表します。既定列を
                               上書きする場合は comment や message も明示的に
                               含めてください。

追加カラム（既定は非表示）:
      --full                     COMMENT と MESSAGE を両方表示
      --with-comment             COMMENT（行テキスト。TODO/FIXME から表示）
      --with-message             MESSAGE（コミットメッセージの1行目）
      --with-snippet             --with-comment の別名（後方互換）
      --with-age                 AGE（日数）列を表形式に追加
      --with-commit-link         URL 列を追加（コミット行リンク）
      --with-link                --with-commit-link の非推奨エイリアス
      --with-pr-links            コミットを含む PR 情報を追加
      --pr-state {all|open|closed|merged}
                                PR の状態でフィルタ（既定: all）
      --pr-limit N              各項目の PR 件数上限（1〜20、既定:3）
      --pr-prefer {open|merged|closed|none}
                                PR 表示時の状態優先順位（既定: open）

トランケート（COMMENT/MESSAGE のみ対象）:
      --truncate N               両方を N 文字で切り詰め（0=無制限）
      --truncate-comment N       コメントのみ N 文字で切り詰め（0=無制限）
      --truncate-message N       メッセージのみ N 文字で切り詰め（0=無制限）
                                 ※ --full だけ指定した場合は既定で 120 文字

並び替え:
      --sort KEYS                並び順（例: --sort -age,file,line）
                                 利用可能キー: age, date, author, email, type, file, line, commit, location

Blame / 進捗:
      --no-ignore-ws             git blame の -w を無効化（空白変更も追跡）
      --no-progress              進捗/ETA を表示しない
      --progress                 パイプ時でも進捗表示を強制

ヘルプ / 言語:
  -h, --help [en|ja]             ヘルプ表示（既定: 英語、ja を付けると日本語）
      --help=ja                  -h ja と同等
      --help-ja                  -h ja と同等
      --lang {en|ja}             言語指定（例: --lang ja -h）
環境変数:
      GTA_LANG=ja                既定のヘルプ言語（GIT_TODO_AUTHORS_LANG でも可）
      TODOX_NO_DEPRECATION_WARNINGS=1
                                   非推奨エイリアスの警告を抑止（CI 向け）
      TODOX_GH_JOBS=N            PR 取得ワーカー数の上限（1〜32。既定は min(jobs,32)）
      GH_TOKEN / GITHUB_TOKEN    gh CLI が使えない環境でも REST 認証で PR を取得
      NO_COLOR=1                 auto でも色を無効化
      CLICOLOR=0                 auto 判定時の色を無効化
      CLICOLOR_FORCE!=0          パイプ越しでも色を強制（"0" 以外を指定）
      FORCE_COLOR!=0             CLICOLOR_FORCE と同じ
      TERM=dumb                  dumb 端末では常に非カラー
      auto モードは stdout の TTY のみを判定（stderr は対象外）

Examples:
  1) TODO/FIXME の「最後に触った人」を一覧:
       todox

  2) TODO/FIXME を「最初に入れた人」を特定:
       todox -m first

  3) 作者で絞り込み（名前/メールを正規表現で）:
       todox -a 'Alice|alice@example.com'

  4) TODO のみ対象:
       todox -t todo

  5) コメント行とコミットメッセージを一緒に（どちらもトリム）:
       todox --full                 # 既定で各120文字
       todox --full --truncate 80   # 各80文字に変更

  6) 片方だけトランケート指定（コメント60 / メッセージは無制限）:
       todox --full --truncate-comment 60 --truncate-message 0

GitHub 連携コマンド:
  todox pr find --commit <sha>    指定コミットを含む PR を一覧表示
  todox pr open --commit <sha>    最初に見つかった PR をブラウザで開く
  todox pr create --commit <sha>  gh CLI 経由で PR を作成（詳細は --help）

  7) 機械処理向け TSV 出力:
       todox --full -o tsv > todo_full.tsv

  8) 進捗制御:
       todox --no-progress
       todox --progress | head   # パイプでも進捗を表示

  9) 空白変更も blame 対象にする:
       todox --no-ignore-ws
`

type scanInputs struct {
	Options  engine.Options
	FieldSel output.FieldSelection
	SortSpec SortSpec
	PRState  string
	PRLimit  int
	PRPrefer string
}

func prepareScanInputs(repoDir string, q url.Values) (scanInputs, error) {
	envCfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return scanInputs{}, err
	}

	configPath, _, err := config.Find(repoDir, os.Getenv("TODOX_CONFIG"), os.Getenv("XDG_CONFIG_HOME"), os.Getenv("HOME"))
	if err != nil {
		return scanInputs{}, err
	}
	fileCfg, err := config.Load(configPath)
	if err != nil {
		return scanInputs{}, err
	}

	baseOpts := engineopts.Defaults(repoDir)
	baseEngine := config.EngineSettingsFromOptions(baseOpts)
	mergedEngine := config.MergeEngine(baseEngine, fileCfg.Engine, envCfg.Engine)
	mergedEngine.ApplyToOptions(&baseOpts)

	baseUI := config.DefaultUISettings()
	mergedUI := config.MergeUI(baseUI, fileCfg.UI, envCfg.UI)
	mergedUI, err = config.NormalizeUI(mergedUI)
	if err != nil {
		return scanInputs{}, err
	}

	options, err := engineopts.ApplyWebQueryToOptions(baseOpts, q)
	if err != nil {
		return scanInputs{}, err
	}

	withAge := mergedUI.WithAge
	if vals := engineopts.SplitMulti(q["with_age"]); len(vals) > 0 {
		raw := vals[len(vals)-1]
		v, parseErr := engineopts.ParseBool(raw, "with_age")
		if parseErr != nil {
			return scanInputs{}, parseErr
		}
		withAge = v
	}

	withCommit := mergedUI.WithCommitLink
	if vals := engineopts.SplitMulti(q["with_commit_link"]); len(vals) > 0 {
		raw := vals[len(vals)-1]
		v, parseErr := engineopts.ParseBool(raw, "with_commit_link")
		if parseErr != nil {
			return scanInputs{}, parseErr
		}
		withCommit = v
	} else if vals := engineopts.SplitMulti(q["with_link"]); len(vals) > 0 {
		raw := vals[len(vals)-1]
		v, parseErr := engineopts.ParseBool(raw, "with_link")
		if parseErr != nil {
			return scanInputs{}, parseErr
		}
		withCommit = v
	}

	withPRs := mergedUI.WithPRLinks
	if vals := engineopts.SplitMulti(q["with_pr_links"]); len(vals) > 0 {
		raw := vals[len(vals)-1]
		v, parseErr := engineopts.ParseBool(raw, "with_pr_links")
		if parseErr != nil {
			return scanInputs{}, parseErr
		}
		withPRs = v
	}

	prState := mergedUI.PRState
	if vals := engineopts.SplitMulti(q["pr_state"]); len(vals) > 0 {
		state, stateErr := config.CanonicalizePRState(vals[len(vals)-1])
		if stateErr != nil {
			return scanInputs{}, stateErr
		}
		prState = state
	}

	prLimit := mergedUI.PRLimit
	if vals := engineopts.SplitMulti(q["pr_limit"]); len(vals) > 0 {
		raw := vals[len(vals)-1]
		limit, parseErr := engineopts.ParseIntInRange(raw, "pr_limit", 1, 20)
		if parseErr != nil {
			return scanInputs{}, parseErr
		}
		prLimit = limit
	}

	prPrefer := mergedUI.PRPrefer
	if vals := engineopts.SplitMulti(q["pr_prefer"]); len(vals) > 0 {
		prefer, preferErr := config.CanonicalizePRPrefer(vals[len(vals)-1])
		if preferErr != nil {
			return scanInputs{}, preferErr
		}
		prPrefer = prefer
	}

	fieldsParam := mergedUI.Fields
	if joined := strings.Join(engineopts.SplitMulti(q["fields"]), ","); strings.TrimSpace(joined) != "" {
		fieldsParam = joined
	}
	sortParam := mergedUI.Sort
	if rawSort := q["sort"]; len(rawSort) > 0 {
		sortParam = strings.TrimSpace(rawSort[len(rawSort)-1])
	}

	fieldSel, err := output.ResolveFields(fieldsParam, options.WithComment, options.WithMessage, withAge, withCommit, withPRs)
	if err != nil {
		return scanInputs{}, err
	}

	sortSpec, err := ParseSortSpec(sortParam)
	if err != nil {
		return scanInputs{}, err
	}

	options.WithComment = fieldSel.NeedComment
	options.WithMessage = fieldSel.NeedMessage

	if err := engineopts.NormalizeAndValidate(&options); err != nil {
		return scanInputs{}, err
	}

	return scanInputs{
		Options:  options,
		FieldSel: fieldSel,
		SortSpec: sortSpec,
		PRState:  prState,
		PRLimit:  prLimit,
		PRPrefer: prPrefer,
	}, nil
}

type streamObserver struct {
	ch     chan progress.Snapshot
	once   sync.Once
	mu     sync.Mutex
	closed bool
}

func newStreamObserver(buffer int) (*streamObserver, <-chan progress.Snapshot) {
	ch := make(chan progress.Snapshot, buffer)
	return &streamObserver{ch: ch}, ch
}

func (o *streamObserver) Publish(s progress.Snapshot) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	select {
	case o.ch <- s:
	default:
		if debugProgressDrops {
			log.Printf("debug: dropping progress snapshot (stage=%s done=%d total=%d)", s.Stage, s.Done, s.Total)
		}
	}
}

func (o *streamObserver) Done(s progress.Snapshot) {
	o.Publish(s)
	o.Close()
}

func (o *streamObserver) Close() {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return
	}
	o.closed = true
	o.once.Do(func() {
		close(o.ch)
	})
	o.mu.Unlock()
}

type suppressCloseObserver struct {
	base *streamObserver
}

func (o suppressCloseObserver) Publish(s progress.Snapshot) {
	if o.base == nil {
		return
	}
	o.base.Publish(s)
}

func (o suppressCloseObserver) Done(s progress.Snapshot) {
	if o.base == nil {
		return
	}
	o.base.Publish(s)
}

type suppressDoneObserver struct {
	base progress.Observer
}

func (o suppressDoneObserver) Publish(s progress.Snapshot) {
	if o.base == nil {
		return
	}
	o.base.Publish(s)
}

func (suppressDoneObserver) Done(progress.Snapshot) {}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func durationSeconds(d time.Duration) any {
	if d <= 0 {
		return nil
	}
	return d.Seconds()
}

func progressPayload(s progress.Snapshot) map[string]any {
	payload := map[string]any{
		"stage":        string(s.Stage),
		"total":        s.Total,
		"done":         s.Done,
		"remaining":    s.Remaining,
		"rate_per_sec": s.RateEMA,
		"rate_p50":     s.RateP50,
		"rate_p10":     s.RateP10,
		"warmup":       s.Warmup,
		"elapsed_sec":  s.Elapsed.Seconds(),
		"started_at":   s.StartedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":   s.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if v := durationSeconds(s.ETAP50); v != nil {
		payload["eta_sec_p50"] = v
	}
	if v := durationSeconds(s.ETAP90); v != nil {
		payload["eta_sec_p90"] = v
	}
	return payload
}

func apiScanHandler(repoDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inputs, err := prepareScanInputs(repoDir, r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		inputs.Options.Progress = false
		inputs.Options.ProgressObserver = nil

		runner := execx.DefaultRunner()

		res, err := engine.Run(inputs.Options)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ApplySort(res.Items, inputs.SortSpec)
		res.HasComment = inputs.FieldSel.ShowComment
		res.HasMessage = inputs.FieldSel.ShowMessage
		res.HasAge = inputs.FieldSel.ShowAge

		ctx := r.Context()
		var remoteCache remoteInfoCache
		_ = applyLinkColumn(ctx, runner, inputs.Options.RepoDir, &remoteCache, res, inputs.FieldSel)
		prStart := time.Time{}
		if inputs.FieldSel.NeedPRs {
			prStart = time.Now()
		}
		_ = applyPRColumns(ctx, runner, inputs.Options.RepoDir, &remoteCache, res, inputs.FieldSel, prOptions{
			State:  inputs.PRState,
			Limit:  inputs.PRLimit,
			Prefer: inputs.PRPrefer,
			Jobs:   inputs.Options.Jobs,
		}, nil)
		if !prStart.IsZero() {
			res.ElapsedMS += time.Since(prStart).Milliseconds()
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	}
}

func apiScanStreamHandler(repoDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		inputs, err := prepareScanInputs(repoDir, r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher.Flush()
		_, _ = fmt.Fprint(w, "retry: 3000\n\n")
		flusher.Flush()

		obsCore, snapCh := newStreamObserver(64)
		inputs.Options.Progress = false
		inputs.Options.ProgressObserver = suppressCloseObserver{base: obsCore}

		runner := execx.DefaultRunner()

		resCh := make(chan *engine.Result, 1)
		errCh := make(chan error, 1)

		type prStageResult struct {
			elapsed time.Duration
			err     error
			hadPRs  bool
		}

		const pingInterval = 30 * time.Second
		pingTicker := time.NewTicker(pingInterval)
		defer pingTicker.Stop()

		go func() {
			res, runErr := engine.Run(inputs.Options)
			if runErr != nil {
				errCh <- runErr
				return
			}
			resCh <- res
		}()

		ctx := r.Context()
		var remoteCache remoteInfoCache
		var currentRes *engine.Result
		var prDoneCh <-chan prStageResult

		// progress イベントの間引き（100ms）
		const minProgressInterval = 100 * time.Millisecond
		var lastProgressSent time.Time
		var pendingSnap *progress.Snapshot
		var throttleTimer *time.Timer
		var throttleCh <-chan time.Time

		stopThrottle := func() {
			if throttleTimer == nil {
				return
			}
			if !throttleTimer.Stop() {
				select {
				case <-throttleTimer.C:
				default:
				}
			}
			throttleTimer = nil
			throttleCh = nil
		}

		flushPending := func() bool {
			if pendingSnap == nil {
				return false
			}
			if err := writeSSE(w, flusher, "progress", progressPayload(*pendingSnap)); err != nil {
				obsCore.Close()
				return true
			}
			lastProgressSent = time.Now()
			pendingSnap = nil
			stopThrottle()
			return false
		}

		for snapCh != nil || resCh != nil || errCh != nil || prDoneCh != nil || throttleCh != nil {
			select {
			case <-ctx.Done():
				obsCore.Close()
				_ = flushPending()
				return
			case <-pingTicker.C:
				if err := writeSSE(w, flusher, "ping", map[string]string{"ts": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
					obsCore.Close()
					_ = flushPending()
					return
				}
			case snap, ok := <-snapCh:
				if !ok {
					if flushPending() {
						return
					}
					snapCh = nil
					continue
				}
				now := time.Now()
				if lastProgressSent.IsZero() || now.Sub(lastProgressSent) >= minProgressInterval {
					if err := writeSSE(w, flusher, "progress", progressPayload(snap)); err != nil {
						obsCore.Close()
						_ = flushPending()
						return
					}
					lastProgressSent = now
					pendingSnap = nil
					stopThrottle()
				} else {
					ps := snap
					pendingSnap = &ps
					wait := minProgressInterval - now.Sub(lastProgressSent)
					if throttleTimer == nil {
						throttleTimer = time.NewTimer(wait)
						throttleCh = throttleTimer.C
					}
				}
			case <-throttleCh:
				if flushPending() {
					return
				}
			case res := <-resCh:
				if flushPending() {
					return
				}
				ApplySort(res.Items, inputs.SortSpec)
				res.HasComment = inputs.FieldSel.ShowComment
				res.HasMessage = inputs.FieldSel.ShowMessage
				res.HasAge = inputs.FieldSel.ShowAge
				_ = applyLinkColumn(ctx, runner, inputs.Options.RepoDir, &remoteCache, res, inputs.FieldSel)

				currentRes = res
				if !inputs.FieldSel.NeedPRs {
					if err := writeSSE(w, flusher, "result", currentRes); err != nil {
						obsCore.Close()
						return
					}
					obsCore.Close()
					resCh = nil
					errCh = nil
					stopThrottle()
					continue
				}

				prChan := make(chan prStageResult, 1)
				go func(res *engine.Result) {
					start := time.Now()
					err := applyPRColumns(ctx, runner, inputs.Options.RepoDir, &remoteCache, res, inputs.FieldSel, prOptions{
						State:  inputs.PRState,
						Limit:  inputs.PRLimit,
						Prefer: inputs.PRPrefer,
						Jobs:   inputs.Options.Jobs,
					}, obsCore)
					elapsed := time.Since(start)
					res.ElapsedMS += elapsed.Milliseconds()
					prChan <- prStageResult{elapsed: elapsed, err: err, hadPRs: true}
					close(prChan)
				}(res)
				prDoneCh = prChan
				resCh = nil
			case prStage, ok := <-prDoneCh:
				if !ok {
					prDoneCh = nil
					continue
				}
				if flushPending() {
					return
				}
				if prStage.err != nil {
					_ = writeSSE(w, flusher, "error", map[string]string{"message": prStage.err.Error()})
					_ = writeSSE(w, flusher, "server_error", map[string]string{"message": prStage.err.Error()})
					obsCore.Close()
					stopThrottle()
					return
				}
				if err := writeSSE(w, flusher, "result", currentRes); err != nil {
					obsCore.Close()
					stopThrottle()
					return
				}
				obsCore.Close()
				prDoneCh = nil
				errCh = nil
				stopThrottle()
			case runErr := <-errCh:
				_ = flushPending()
				obsCore.Close()
				_ = writeSSE(w, flusher, "error", map[string]string{"message": runErr.Error()})
				_ = writeSSE(w, flusher, "server_error", map[string]string{"message": runErr.Error()})
				stopThrottle()
				return
			}
		}
	}
}

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var port = fs.Int("p", 8080, "port")
	var repo = fs.String("repo", ".", "repo root")
	_ = fs.Parse(args)

	mux := http.NewServeMux()
	web.Register(mux)
	mux.HandleFunc("/api/scan", apiScanHandler(*repo))
	mux.HandleFunc("/api/scan/stream", apiScanStreamHandler(*repo))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("todox serve listening on %s (repo=%s)", addr, mustAbs(*repo))
	log.Fatal(http.ListenAndServe(addr, mux))
}

type prOptions struct {
	State  string
	Limit  int
	Prefer string
	Jobs   int
}

type remoteInfoCache struct {
	once sync.Once
	info gitremote.Info
	err  error
}

func (c *remoteInfoCache) Get(ctx context.Context, runner execx.Runner, repoDir string) (gitremote.Info, error) {
	if c == nil {
		return gitremote.Detect(ctx, runner, repoDir)
	}
	c.once.Do(func() {
		c.info, c.err = gitremote.Detect(ctx, runner, repoDir)
	})
	return c.info, c.err
}

func applyLinkColumn(ctx context.Context, runner execx.Runner, repoDir string, cache *remoteInfoCache, res *engine.Result, sel output.FieldSelection) error {
	if res == nil {
		return nil
	}
	res.HasURL = sel.NeedURL
	if !sel.NeedURL {
		return nil
	}
	info, err := cache.Get(ctx, runner, repoDir)
	if err != nil {
		for idx := range res.Items {
			res.Items[idx].URL = ""
		}
		msg := "failed to determine git remote: " + err.Error()
		already := false
		for _, e := range res.Errors {
			if e.Stage == "link" && e.Message == msg {
				already = true
				break
			}
		}
		if !already {
			res.Errors = append(res.Errors, engine.ItemError{
				Stage:   "link",
				Message: msg,
			})
		}
		res.ErrorCount = len(res.Errors)
		return nil
	}
	for idx := range res.Items {
		it := &res.Items[idx]
		it.URL = link.Blob(info, it.Commit, it.File, it.Line)
	}
	return nil
}

func applyPRColumns(ctx context.Context, runner execx.Runner, repoDir string, cache *remoteInfoCache, res *engine.Result, sel output.FieldSelection, opts prOptions, obs progress.Observer) error {
	if res == nil {
		return nil
	}
	defer func() {
		res.ErrorCount = len(res.Errors)
	}()
	res.HasPRs = sel.NeedPRs
	if !sel.NeedPRs || len(res.Items) == 0 {
		return nil
	}

	commitToIndexes := make(map[string][]int)
	commits := make([]string, 0, len(res.Items))
	for idx := range res.Items {
		sha := strings.TrimSpace(res.Items[idx].Commit)
		if sha == "" {
			continue
		}
		if _, ok := commitToIndexes[sha]; !ok {
			commits = append(commits, sha)
		}
		commitToIndexes[sha] = append(commitToIndexes[sha], idx)
	}

	// (1) PR 進捗推定器を先に初期化しておき、初期スナップショットを Publish する
	var prEstimator *progress.Estimator
	if obs != nil {
		prEstimator = progress.NewEstimator(len(commits), progress.Config{})
		if snap, changed := prEstimator.Stage(progress.StagePR); changed {
			obs.Publish(snap)
		}
	}
	// (2) コミット 0 件なら Complete→Publish→Done を送って終了
	if len(commits) == 0 {
		if prEstimator != nil {
			finalSnap := prEstimator.Complete()
			obs.Publish(finalSnap)
			obs.Done(finalSnap)
		}
		return nil
	}

	info, err := cache.Get(ctx, runner, repoDir)
	if err != nil {
		msg := "failed to determine git remote: " + err.Error()
		recordPRStageError(res, msg)
		// (3) リモート解決が失敗した場合も Complete→Publish→Done を送って終端させる
		if prEstimator != nil {
			finalSnap := prEstimator.Complete()
			obs.Publish(finalSnap)
			obs.Done(finalSnap)
		}
		return nil
	}

	client := ghclient.NewClient(info, repoDir, runner)
	workerCount := prWorkerCount(len(commits), opts.Jobs)
	type prFetchResult struct {
		commit string
		prs    []ghclient.PRInfo
		err    error
	}
	jobs := make(chan string)
	results := make(chan prFetchResult, workerCount)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for commit := range jobs {
				prs, fetchErr := client.FindPullRequestsByCommit(ctx, commit)
				select {
				case results <- prFetchResult{commit: commit, prs: prs, err: fetchErr}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(results)
		for _, commit := range commits {
			select {
			case jobs <- commit:
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				return
			}
		}
		close(jobs)
		wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			// (4) キャンセル時も Publish→Done を送って Stage=pr が閉じるようにする
			if prEstimator != nil {
				snap, _ := prEstimator.Stage(progress.StagePR)
				obs.Publish(snap)
				obs.Done(snap)
			}
			res.ErrorCount = len(res.Errors)
			return nil
		case result, ok := <-results:
			if !ok {
				// (5) 終了時は Complete→Publish→Done を明示的に送る
				if prEstimator != nil {
					finalSnap := prEstimator.Complete()
					obs.Publish(finalSnap)
					obs.Done(finalSnap)
				}
				res.ErrorCount = len(res.Errors)
				return nil
			}
			if result.err != nil {
				msg := fmt.Sprintf("failed to fetch pull requests for commit %s: %v", short(result.commit), result.err)
				recordPRStageError(res, msg)
				continue
			}
			filtered, filterErr := filterPRsByState(result.prs, opts.State, "--pr-state")
			if filterErr != nil {
				recordPRStageError(res, filterErr.Error())
				continue
			}
			sortPRsByPreference(filtered, opts.Prefer)
			limited := limitPRs(filtered, opts.Limit)
			refs := make([]engine.PullRequestRef, 0, len(limited))
			for _, pr := range limited {
				refs = append(refs, engine.PullRequestRef{
					Number: pr.Number,
					State:  strings.ToLower(strings.TrimSpace(pr.State)),
					URL:    pr.URL,
					Title:  pr.Title,
					Body:   pr.Body,
				})
			}
			for _, idx := range commitToIndexes[result.commit] {
				res.Items[idx].PRs = append([]engine.PullRequestRef(nil), refs...)
			}
			if prEstimator != nil {
				if snap, notify := prEstimator.Advance(1); notify {
					obs.Publish(snap)
				}
			}
		}
	}
}

func prWorkerCount(commitCount, jobs int) int {
	max := jobs
	if max < 1 {
		max = 1
	}
	if env := strings.TrimSpace(os.Getenv("TODOX_GH_JOBS")); env != "" {
		if parsed, err := strconv.Atoi(env); err == nil && parsed > 0 {
			max = parsed
		}
	}
	const hardCap = 32
	if max > hardCap {
		max = hardCap
	}
	if commitCount > 0 && max > commitCount {
		max = commitCount
	}
	if max < 1 {
		max = 1
	}
	return max
}

func sortPRsByPreference(prs []ghclient.PRInfo, prefer string) {
	if len(prs) <= 1 {
		return
	}
	if strings.EqualFold(strings.TrimSpace(prefer), "none") {
		return
	}
	priority := map[string]int{"open": 1, "merged": 2, "closed": 3}
	switch prefer {
	case "merged":
		priority = map[string]int{"merged": 1, "open": 2, "closed": 3}
	case "closed":
		priority = map[string]int{"closed": 1, "open": 2, "merged": 3}
	}
	sort.SliceStable(prs, func(i, j int) bool {
		stateI := priority[strings.ToLower(strings.TrimSpace(prs[i].State))]
		stateJ := priority[strings.ToLower(strings.TrimSpace(prs[j].State))]
		if stateI != stateJ {
			return stateI < stateJ
		}
		return prs[i].Number < prs[j].Number
	})
}

func limitPRs(prs []ghclient.PRInfo, max int) []ghclient.PRInfo {
	if max <= 0 || len(prs) <= max {
		return prs
	}
	return prs[:max]
}

func recordPRStageError(res *engine.Result, msg string) {
	if res == nil || msg == "" {
		return
	}
	for _, e := range res.Errors {
		if e.Stage == "pr" && e.Message == msg {
			return
		}
	}
	res.Errors = append(res.Errors, engine.ItemError{Stage: "pr", Message: msg})
	res.ErrorCount = len(res.Errors)
}

func printTSV(res *engine.Result, sel output.FieldSelection) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0) // tabs only
	write := func(text string) {
		mustFprintln(w, text)
	}
	headers := output.Headers(sel.Fields)
	write(strings.Join(headers, "\t"))
	for _, it := range res.Items {
		row := output.RowValues(it, sel.Fields)
		for i := range row {
			row[i] = sanitizeField(row[i])
		}
		write(strings.Join(row, "\t"))
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

type tableColorConfig struct {
	enabled  bool
	profile  termcolor.Profile
	scheme   termcolor.Scheme
	ageScale float64 // AGE グラデーションの正規化係数（p95 を基準、下限 120 日、データ無し時は 120）
}

func printTable(res *engine.Result, sel output.FieldSelection, colors tableColorConfig) {
	colCount := len(sel.Fields)
	if colors.enabled && colors.scheme == termcolor.SchemeUnknown {
		env := toEnvMap(os.Environ())
		colors.scheme = termcolor.DetectScheme(env)
	}
	widths := make([]int, colCount)
	for i, f := range sel.Fields {
		widths[i] = textutil.VisibleWidth(f.Header)
	}
	if sel.ShowAge && colors.enabled {
		// AGE 列の色分布を決めるために p95 を基準としたスケールを算出する。
		colors.ageScale = computeAgeScale(res.Items)
	}
	rows := make([][]tableCell, len(res.Items))
	for rowIdx, it := range res.Items {
		row := make([]tableCell, colCount)
		for colIdx, f := range sel.Fields {
			val := sanitizeField(output.FormatFieldValue(it, f.Key))
			style := tableCellStyle(f.Key, it, colors)
			row[colIdx] = tableCell{text: val, style: style}
			if w := textutil.VisibleWidth(val); w > widths[colIdx] {
				widths[colIdx] = w
			}
		}
		rows[rowIdx] = row
	}
	headers := make([]tableCell, colCount)
	for i, f := range sel.Fields {
		headers[i] = tableCell{text: f.Header, style: termcolor.HeaderStyle()}
	}
	render := func(cells []tableCell) string {
		var b strings.Builder
		for i, cell := range cells {
			if i > 0 {
				b.WriteString("  ")
			}
			width := widths[i]
			truncated := textutil.TruncateByWidth(cell.text, width, "…")
			// 表示幅の計算と切り詰めは ANSI コードを除去したテキストに対して行い、
			// パディング後にスタイルを適用して桁揃えとリセットを保証する。
			if isRightAligned(sel.Fields[i].Key) {
				aligned := textutil.PadLeft(truncated, width)
				b.WriteString(termcolor.Apply(cell.style, aligned, colors.enabled))
			} else {
				aligned := textutil.PadRight(truncated, width)
				b.WriteString(termcolor.Apply(cell.style, aligned, colors.enabled))
			}
		}
		return b.String()
	}
	mustFprintln(os.Stdout, render(headers))
	for _, row := range rows {
		mustFprintln(os.Stdout, render(row))
	}
}

// toEnvMap converts a "KEY=VALUE" environment list into a map.
// DetectScheme relies on this form, and keeping it local avoids
// depending on helper functions that may not exist in older trees.
func toEnvMap(values []string) map[string]string {
	m := make(map[string]string, len(values))
	for _, entry := range values {
		if entry == "" {
			continue
		}
		if i := strings.IndexByte(entry, '='); i >= 0 {
			m[entry[:i]] = entry[i+1:]
			continue
		}
		m[entry] = ""
	}
	return m
}

type tableCell struct {
	text  string
	style termcolor.Style // このセルに適用する SGR スタイル（ゼロ値なら非カラー）
}

func tableCellStyle(key string, item engine.Item, colors tableColorConfig) termcolor.Style {
	if !colors.enabled {
		return termcolor.Style{}
	}
	switch key {
	case "type":
		return termcolor.TypeStyle(item.Kind, colors.scheme, colors.profile)
	case "age":
		return ageCellStyle(item.AgeDays, colors)
	default:
		return termcolor.Style{}
	}
}

func ageCellStyle(age int, colors tableColorConfig) termcolor.Style {
	scale := colors.ageScale
	if scale <= 0 {
		scale = 120
	}
	if colors.profile == termcolor.ProfileTrueColor || colors.profile == termcolor.ProfileANSI256 {
		return termcolor.AgeStyle(age, colors.profile, scale)
	}
	return termcolor.AgeStyle(age, termcolor.ProfileBasic8, scale)
}

// computeAgeScale は AGE の 95 パーセンタイル（最低 120 日、データが空なら 120）を返し、
// その値をグラデーションの上限として正規化に利用する。負の AGE は 0 に丸める。
func computeAgeScale(items []engine.Item) float64 {
	if len(items) == 0 {
		return 120
	}
	ages := make([]int, 0, len(items))
	for _, it := range items {
		age := it.AgeDays
		if age < 0 {
			age = 0
		}
		ages = append(ages, age)
	}
	sort.Ints(ages)
	idx := int(math.Ceil(0.95*float64(len(ages)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(ages) {
		idx = len(ages) - 1
	}
	p95 := ages[idx]
	if p95 < 0 {
		p95 = 0
	}
	scale := math.Max(120, float64(p95))
	if scale <= 0 {
		return 120
	}
	return scale
}

func isRightAligned(key string) bool {
	switch key {
	case "age", "date", "commit":
		return true
	default:
		return false
	}
}

func envBool(key string) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return false
	}
	switch strings.ToLower(val) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func mustFprintln(w io.Writer, text string) {
	if _, err := fmt.Fprintln(w, text); err != nil {
		log.Fatal(err)
	}
}

func short(s string) string {
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

func sanitizeField(s string) string {
	const newlineMark = "⏎"
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", newlineMark)
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

func reportErrors(res *engine.Result) {
	const maxDetails = 5
	fmt.Fprintf(os.Stderr, "todox: %d error(s) while invoking git commands\n", res.ErrorCount)
	for i, e := range res.Errors {
		if i >= maxDetails {
			remaining := res.ErrorCount - maxDetails
			if remaining > 0 {
				fmt.Fprintf(os.Stderr, "  ... (%d more)\n", remaining)
			}
			break
		}
		loc := fmt.Sprintf("%s:%d", e.File, e.Line)
		if e.File == "" && e.Line == 0 {
			loc = "(unknown location)"
		}
		stage := e.Stage
		if stage == "" {
			stage = "git"
		}
		fmt.Fprintf(os.Stderr, "  - %s [%s] %s\n", loc, stage, e.Message)
	}
}

func mustAbs(p string) string {
	a, _ := filepath.Abs(p)
	return a
}
