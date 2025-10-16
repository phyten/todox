package engine

import (
	"regexp"
	"time"

	"github.com/phyten/todox/internal/model"
	"github.com/phyten/todox/internal/progress"
)

// Item は 1 件の TODO/FIXME を表す
type Item struct {
	Kind      string           `json:"kind"`
	Tag       string           `json:"tag,omitempty"`
	Lang      string           `json:"lang,omitempty"`
	MatchKind string           `json:"match_kind,omitempty"`
	Text      string           `json:"text,omitempty"`
	Span      model.Span       `json:"span"`
	Author    string           `json:"author"`
	Email     string           `json:"email"`
	Date      string           `json:"date"`
	AgeDays   int              `json:"age_days"`
	Commit    string           `json:"commit"`
	File      string           `json:"file"`
	Line      int              `json:"line"`
	Comment   string           `json:"comment,omitempty"`
	Message   string           `json:"message,omitempty"`
	URL       string           `json:"url,omitempty"`
	PRs       []PullRequestRef `json:"prs,omitempty"`
}

// PullRequestRef はコミットに紐づく PR の参照情報を表す
type PullRequestRef struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	URL    string `json:"url"`
	Title  string `json:"title,omitempty"`
	Body   string `json:"body,omitempty"`
}

// ItemError は 1 行の取得に失敗した際の情報を表す
type ItemError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

// Options は実行オプション
type Options struct {
	Type              string // todo|fixme|both
	Mode              string // last|first
	DetectMode        string
	AuthorRegex       string
	WithComment       bool
	WithMessage       bool
	IncludeStrings    bool
	Tags              []string
	TruncAll          int
	TruncComment      int
	TruncMessage      int
	IgnoreWS          bool
	Jobs              int
	RepoDir           string
	Progress          bool
	Now               time.Time
	DetectLangs       []string
	Paths             []string
	Excludes          []string
	PathRegex         []string
	PathRegexCompiled []*regexp.Regexp
	MaxFileBytes      int
	ExcludeTypical    bool
	NoPrefilter       bool
	ProgressObserver  progress.Observer `json:"-"`
}

// Result は出力
type Result struct {
	Items      []Item      `json:"items"`
	HasComment bool        `json:"has_comment"`
	HasMessage bool        `json:"has_message"`
	HasAge     bool        `json:"has_age"`
	HasURL     bool        `json:"has_url"`
	HasPRs     bool        `json:"has_prs"`
	Total      int         `json:"total"`
	ElapsedMS  int64       `json:"elapsed_ms"`
	Errors     []ItemError `json:"errors,omitempty"`
	ErrorCount int         `json:"error_count"`
}
