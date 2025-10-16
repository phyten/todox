package model

// MatchKind 表示検出対象の種別（コメント／文字列など）。
type MatchKind string

const (
	MatchKindUnknown MatchKind = "unknown"
	MatchKindComment MatchKind = "comment"
	MatchKindString  MatchKind = "string"
	MatchKindHeredoc MatchKind = "heredoc"
)

// Span は 1 件の検出範囲を行・桁・バイトオフセットで表します。
type Span struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	ByteStart int
	ByteEnd   int
}

// Match は構文解析またはフォールバック検出による 1 件の TODO/FIXME を表します。
type Match struct {
	File string
	Lang string
	Kind MatchKind
	Tag  string
	Text string
	Span Span
}
