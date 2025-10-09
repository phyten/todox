package config

import "strings"

func MergeEngine(base EngineSettings, layers ...EngineConfig) EngineSettings {
	out := base
	for _, layer := range layers {
		out.Type = ResolveString(out.Type, layer.Type)
		out.Mode = ResolveString(out.Mode, layer.Mode)
		out.Author = ResolveString(out.Author, layer.Author)
		out.Paths = ResolveStrings(out.Paths, layer.Paths)
		out.Excludes = ResolveStrings(out.Excludes, layer.Excludes)
		out.PathRegex = ResolveStrings(out.PathRegex, layer.PathRegex)
		out.ExcludeTypical = ResolveBool(out.ExcludeTypical, layer.ExcludeTypical)
		out.WithComment = ResolveBool(out.WithComment, layer.WithComment)
		out.WithMessage = ResolveBool(out.WithMessage, layer.WithMessage)
		out.TruncAll = ResolveInt(out.TruncAll, layer.TruncAll)
		out.TruncComment = ResolveInt(out.TruncComment, layer.TruncComment)
		out.TruncMessage = ResolveInt(out.TruncMessage, layer.TruncMessage)
		out.IgnoreWS = ResolveBool(out.IgnoreWS, layer.IgnoreWS)
		out.Jobs = ResolveInt(out.Jobs, layer.Jobs)
		out.Repo = ResolveAndTrim(out.Repo, layer.Repo)
		out.Output = ResolveAndTrim(out.Output, layer.Output)
		out.Color = ResolveAndTrim(out.Color, layer.Color)
	}
	if strings.TrimSpace(out.Output) == "" {
		out.Output = "table"
	}
	if strings.TrimSpace(out.Color) == "" {
		out.Color = "auto"
	}
	return out
}

func MergeUI(base UISettings, layers ...UIConfig) UISettings {
	out := base
	for _, layer := range layers {
		out.WithAge = ResolveBool(out.WithAge, layer.WithAge)
		out.WithCommitLink = ResolveBool(out.WithCommitLink, layer.WithCommitLink)
		out.WithPRLinks = ResolveBool(out.WithPRLinks, layer.WithPRLinks)
		out.PRState = ResolveString(out.PRState, layer.PRState)
		out.PRLimit = ResolveInt(out.PRLimit, layer.PRLimit)
		out.PRPrefer = ResolveString(out.PRPrefer, layer.PRPrefer)
		out.Fields = ResolveAndTrim(out.Fields, layer.Fields)
		out.Sort = ResolveAndTrim(out.Sort, layer.Sort)
	}
	out.PRState = strings.TrimSpace(out.PRState)
	out.PRPrefer = strings.TrimSpace(out.PRPrefer)
	return out
}
