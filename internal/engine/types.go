package engine

// Item は 1 件の TODO/FIXME を表す
type Item struct {
	Kind    string `json:"kind"` // TODO | FIXME | TODO|FIXME
	Author  string `json:"author"`
	Email   string `json:"email"`
	Date    string `json:"date"`   // author date (iso-strict-local)
	Commit  string `json:"commit"` // full SHA
	File    string `json:"file"`
	Line    int    `json:"line"`
	Comment string `json:"comment,omitempty"` // TODO/FIXME からの行
	Message string `json:"message,omitempty"` // commit subject (1行目)
}

// Options は実行オプション
type Options struct {
	Type         string // todo|fixme|both
	Mode         string // last|first
	AuthorRegex  string
	WithComment  bool
	WithMessage  bool
	TruncAll     int
	TruncComment int
	TruncMessage int
	IgnoreWS     bool
	Jobs         int
	RepoDir      string
	Progress     bool
}

// Result は出力
type Result struct {
	Items      []Item `json:"items"`
	HasComment bool   `json:"has_comment"`
	HasMessage bool   `json:"has_message"`
	Total      int    `json:"total"`
	ElapsedMS  int64  `json:"elapsed_ms"`
}
