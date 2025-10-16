package engine

import "github.com/phyten/todox/internal/model"

var (
	styleC = commentStyle{
		linePrefixes: []string{"//"},
		block:        []blockPattern{{start: "/*", end: "*/", kind: model.MatchKindComment}},
		stringDelims: []string{"\""},
	}
	styleGo = commentStyle{
		linePrefixes: []string{"//"},
		block:        []blockPattern{{start: "/*", end: "*/", kind: model.MatchKindComment}, {start: "`", end: "`", kind: model.MatchKindString}},
		stringDelims: []string{"\""},
	}
	styleJS = commentStyle{
		linePrefixes: []string{"//"},
		block:        []blockPattern{{start: "/*", end: "*/", kind: model.MatchKindComment}, {start: "`", end: "`", kind: model.MatchKindString}},
		stringDelims: []string{"\"", "'"},
	}
	styleHash = commentStyle{
		linePrefixes: []string{"#"},
		stringDelims: []string{"\"", "'"},
	}
	styleRuby = commentStyle{
		linePrefixes: []string{"#"},
		block:        []blockPattern{{start: "=begin", end: "=end", kind: model.MatchKindComment, allowIndentedStart: true}},
		stringDelims: []string{"\"", "'"},
	}
	stylePython = commentStyle{
		linePrefixes: []string{"#"},
		block:        []blockPattern{{start: "\"\"\"", end: "\"\"\"", kind: model.MatchKindString}, {start: "'''", end: "'''", kind: model.MatchKindString}},
		stringDelims: []string{"\"", "'"},
	}
	styleHTML = commentStyle{
		block: []blockPattern{{start: "<!--", end: "-->", kind: model.MatchKindComment}},
	}
	styleSQL = commentStyle{
		linePrefixes: []string{"--"},
		block:        []blockPattern{{start: "/*", end: "*/", kind: model.MatchKindComment}},
		stringDelims: []string{"'"},
	}
	styleCSS = commentStyle{
		block: []blockPattern{{start: "/*", end: "*/", kind: model.MatchKindComment}},
	}
	styleIni = commentStyle{
		linePrefixes: []string{";", "#"},
	}
	styleHCL = commentStyle{
		linePrefixes: []string{"//", "#"},
		block:        []blockPattern{{start: "/*", end: "*/", kind: model.MatchKindComment}},
	}
	styleLisp = commentStyle{
		linePrefixes: []string{";"},
	}
	styleHaskell = commentStyle{
		linePrefixes: []string{"--"},
		block:        []blockPattern{{start: "{-", end: "-}", kind: model.MatchKindComment}},
	}
	stylePowershell = commentStyle{
		linePrefixes: []string{"#"},
		block:        []blockPattern{{start: "<#", end: "#>", kind: model.MatchKindComment}},
	}
	styleJinja = commentStyle{
		block: []blockPattern{{start: "{#", end: "#}", kind: model.MatchKindComment}},
	}
	styleTwig       = styleJinja
	styleHandlebars = commentStyle{
		block: []blockPattern{{start: "{{!--", end: "--}}", kind: model.MatchKindComment}, {start: "{{!", end: "}}", kind: model.MatchKindComment}},
	}
	styleBatch = commentStyle{
		linePrefixes: []string{"REM ", "rem ", "::"},
	}
	stylePug = commentStyle{
		linePrefixes: []string{"//", "//-"},
	}
	styleBash = commentStyle{
		linePrefixes: []string{"#"},
		stringDelims: []string{"\"", "'", "`"},
	}
)

var languageStyleMap = map[string]commentStyle{
	"c":               styleC,
	"cpp":             styleC,
	"objective-c":     styleC,
	"objective-cpp":   styleC,
	"go":              styleGo,
	"java":            styleC,
	"csharp":          styleC,
	"scala":           styleC,
	"kotlin":          styleC,
	"swift":           styleC,
	"groovy":          styleC,
	"dart":            styleC,
	"rust":            styleC,
	"typescript":      styleJS,
	"typescriptreact": styleJS,
	"javascript":      styleJS,
	"javascriptreact": styleJS,
	"coffeescript":    styleJS,
	"php":             styleJS,
	"proto":           styleC,
	"thrift":          styleC,
	"hcl":             styleHCL,
	"terraform":       styleHCL,
	"cue":             styleHash,
	"starlark":        stylePython,
	"python":          stylePython,
	"ruby":            styleRuby,
	"perl":            styleHash,
	"shell":           styleBash,
	"fish":            styleHash,
	"powershell":      stylePowershell,
	"batch":           styleBatch,
	"yaml":            styleHash,
	"toml":            styleHash,
	"ini":             styleIni,
	"properties":      styleIni,
	"dotenv":          styleHash,
	"json":            styleHash,
	"markdown":        styleHash,
	"rst":             styleHash,
	"asciidoc":        styleHash,
	"latex":           styleHash,
	"html":            styleHTML,
	"vue":             styleHTML,
	"svelte":          styleHTML,
	"xml":             styleHTML,
	"css":             styleCSS,
	"scss":            styleCSS,
	"sass":            styleCSS,
	"less":            styleCSS,
	"stylus":          styleCSS,
	"sql":             styleSQL,
	"make":            styleHash,
	"ninja":           styleHash,
	"dockerfile":      styleHash,
	"jinja":           styleJinja,
	"twig":            styleTwig,
	"handlebars":      styleHandlebars,
	"django":          styleJinja,
	"liquid":          styleJinja,
	"pug":             stylePug,
	"haml":            styleHash,
	"erb":             styleRuby,
	"ejs":             styleJS,
	"aspnet":          styleHTML,
	"common-lisp":     styleLisp,
	"scheme":          styleLisp,
	"racket":          styleLisp,
	"haskell":         styleHaskell,
	"erlang":          styleHash,
	"elixir":          styleHash,
	"elm":             styleJS,
	"ocaml":           styleHaskell,
	"pascal":          styleHash,
	"ada":             styleHash,
	"verilog":         styleC,
	"systemverilog":   styleC,
	"cython":          stylePython,
	"apex":            styleC,
	"ahk":             styleHash,
	"autoit":          styleHash,
	"nim":             styleHash,
	"zig":             styleC,
	"smali":           styleHash,
	"julia":           styleHash,
	"rego":            styleHash,
	"pip":             styleHash,
	"gradle":          styleC,
	"cmake":           styleHash,
	"procfile":        styleHash,
}

func styleForLanguage(lang string) (commentStyle, bool) {
	cs, ok := languageStyleMap[lang]
	return cs, ok
}
