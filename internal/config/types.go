package config

import (
	"strings"

	"github.com/phyten/todox/internal/engine"
)

type EngineConfig struct {
	Type           *string   `yaml:"type" toml:"type" json:"type"`
	Mode           *string   `yaml:"mode" toml:"mode" json:"mode"`
	Detect         *string   `yaml:"detect" toml:"detect" json:"detect"`
	Author         *string   `yaml:"author" toml:"author" json:"author"`
	Paths          *[]string `yaml:"path" toml:"path" json:"path"`
	Excludes       *[]string `yaml:"exclude" toml:"exclude" json:"exclude"`
	PathRegex      *[]string `yaml:"path_regex" toml:"path_regex" json:"path_regex"`
	ExcludeTypical *bool     `yaml:"exclude_typical" toml:"exclude_typical" json:"exclude_typical"`
	WithComment    *bool     `yaml:"with_comment" toml:"with_comment" json:"with_comment"`
	WithMessage    *bool     `yaml:"with_message" toml:"with_message" json:"with_message"`
	IncludeStrings *bool     `yaml:"include_strings" toml:"include_strings" json:"include_strings"`
	CommentsOnly   *bool     `yaml:"comments_only" toml:"comments_only" json:"comments_only"`
	DetectLangs    *[]string `yaml:"detect_langs" toml:"detect_langs" json:"detect_langs"`
	Tags           *[]string `yaml:"tags" toml:"tags" json:"tags"`
	TruncAll       *int      `yaml:"truncate" toml:"truncate" json:"truncate"`
	TruncComment   *int      `yaml:"truncate_comment" toml:"truncate_comment" json:"truncate_comment"`
	TruncMessage   *int      `yaml:"truncate_message" toml:"truncate_message" json:"truncate_message"`
	IgnoreWS       *bool     `yaml:"ignore_ws" toml:"ignore_ws" json:"ignore_ws"`
	Jobs           *int      `yaml:"jobs" toml:"jobs" json:"jobs"`
	Repo           *string   `yaml:"repo" toml:"repo" json:"repo"`
	Output         *string   `yaml:"output" toml:"output" json:"output"`
	Color          *string   `yaml:"color" toml:"color" json:"color"`
	MaxFileBytes   *int      `yaml:"max_file_bytes" toml:"max_file_bytes" json:"max_file_bytes"`
	NoPrefilter    *bool     `yaml:"no_prefilter" toml:"no_prefilter" json:"no_prefilter"`
}

type UIConfig struct {
	WithAge        *bool   `yaml:"with_age" toml:"with_age" json:"with_age"`
	WithCommitLink *bool   `yaml:"with_commit_link" toml:"with_commit_link" json:"with_commit_link"`
	WithPRLinks    *bool   `yaml:"with_pr_links" toml:"with_pr_links" json:"with_pr_links"`
	PRState        *string `yaml:"pr_state" toml:"pr_state" json:"pr_state"`
	PRLimit        *int    `yaml:"pr_limit" toml:"pr_limit" json:"pr_limit"`
	PRPrefer       *string `yaml:"pr_prefer" toml:"pr_prefer" json:"pr_prefer"`
	Fields         *string `yaml:"fields" toml:"fields" json:"fields"`
	Sort           *string `yaml:"sort" toml:"sort" json:"sort"`
}

type Config struct {
	Engine EngineConfig `yaml:"engine" toml:"engine" json:"engine"`
	UI     UIConfig     `yaml:"ui" toml:"ui" json:"ui"`
}

type EngineSettings struct {
	Type           string
	Mode           string
	Detect         string
	Author         string
	Paths          []string
	Excludes       []string
	PathRegex      []string
	ExcludeTypical bool
	WithComment    bool
	WithMessage    bool
	IncludeStrings bool
	DetectLangs    []string
	Tags           []string
	TruncAll       int
	TruncComment   int
	TruncMessage   int
	IgnoreWS       bool
	Jobs           int
	Repo           string
	Output         string
	Color          string
	MaxFileBytes   int
	NoPrefilter    bool
}

type UISettings struct {
	WithAge        bool
	WithCommitLink bool
	WithPRLinks    bool
	PRState        string
	PRLimit        int
	PRPrefer       string
	Fields         string
	Sort           string
}

func EngineSettingsFromOptions(opts engine.Options) EngineSettings {
	return EngineSettings{
		Type:           opts.Type,
		Mode:           opts.Mode,
		Detect:         opts.DetectMode,
		Author:         opts.AuthorRegex,
		Paths:          cloneStrings(opts.Paths),
		Excludes:       cloneStrings(opts.Excludes),
		PathRegex:      cloneStrings(opts.PathRegex),
		ExcludeTypical: opts.ExcludeTypical,
		WithComment:    opts.WithComment,
		WithMessage:    opts.WithMessage,
		IncludeStrings: opts.IncludeStrings,
		DetectLangs:    cloneStrings(opts.DetectLangs),
		Tags:           cloneStrings(opts.Tags),
		TruncAll:       opts.TruncAll,
		TruncComment:   opts.TruncComment,
		TruncMessage:   opts.TruncMessage,
		IgnoreWS:       opts.IgnoreWS,
		Jobs:           opts.Jobs,
		Repo:           opts.RepoDir,
		Output:         "table",
		Color:          "auto",
		MaxFileBytes:   opts.MaxFileBytes,
		NoPrefilter:    opts.NoPrefilter,
	}
}

func (s EngineSettings) ApplyToOptions(opts *engine.Options) {
	if opts == nil {
		return
	}
	opts.Type = s.Type
	opts.Mode = s.Mode
	opts.DetectMode = s.Detect
	opts.AuthorRegex = s.Author
	opts.Paths = cloneStrings(s.Paths)
	opts.Excludes = cloneStrings(s.Excludes)
	opts.PathRegex = cloneStrings(s.PathRegex)
	opts.ExcludeTypical = s.ExcludeTypical
	opts.WithComment = s.WithComment
	opts.WithMessage = s.WithMessage
	opts.IncludeStrings = s.IncludeStrings
	opts.DetectLangs = cloneStrings(s.DetectLangs)
	opts.Tags = cloneStrings(s.Tags)
	opts.TruncAll = s.TruncAll
	opts.TruncComment = s.TruncComment
	opts.TruncMessage = s.TruncMessage
	opts.IgnoreWS = s.IgnoreWS
	opts.Jobs = s.Jobs
	if trimmed := strings.TrimSpace(s.Repo); trimmed != "" {
		opts.RepoDir = trimmed
	}
	opts.MaxFileBytes = s.MaxFileBytes
	opts.NoPrefilter = s.NoPrefilter
}

func DefaultUISettings() UISettings {
	return UISettings{
		WithAge:        false,
		WithCommitLink: false,
		WithPRLinks:    false,
		PRState:        "all",
		PRLimit:        3,
		PRPrefer:       "open",
		Fields:         "",
		Sort:           "",
	}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
