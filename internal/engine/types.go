package engine

import "time"

// Item は 1 件の TODO/FIXME を表す
type Item struct {
	Kind    string `json:"kind"` // TODO | FIXME | TODO|FIXME
	Author  string `json:"author"`
	Email   string `json:"email"`
	Date    string `json:"date"`     // author date (iso-strict-local)
	AgeDays int    `json:"age_days"` // author date からの経過日数
	Commit  string `json:"commit"`   // full SHA
	File    string `json:"file"`
	Line    int    `json:"line"`
	Comment string `json:"comment,omitempty"` // TODO/FIXME からの行
	Message string `json:"message,omitempty"` // commit subject (1行目)
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
	Type           string // todo|fixme|both
	Mode           string // last|first
	AuthorRegex    string
	WithComment    bool
	WithMessage    bool
	TruncAll       int
	TruncComment   int
	TruncMessage   int
	IgnoreWS       bool
	Jobs           int
	RepoDir        string
	Progress       bool
	Now            time.Time
	Paths          []string
	Excludes       []string
	PathRegex      []string
	ExcludeTypical bool
}

// Result は出力
type Result struct {
	Items      []Item      `json:"items"`
	HasComment bool        `json:"has_comment"`
	HasMessage bool        `json:"has_message"`
	HasAge     bool        `json:"has_age"`
	Total      int         `json:"total"`
	ElapsedMS  int64       `json:"elapsed_ms"`
	Errors     []ItemError `json:"errors,omitempty"`
	ErrorCount int         `json:"error_count"`
}
