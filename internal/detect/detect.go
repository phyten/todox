package detect

import (
	"bytes"
	"path/filepath"
	"strings"
)

type Info struct {
	Name string
}

func FromPathAndContent(p string, data []byte) Info {
	name := detectByPath(p)
	if name != "" {
		if strings.EqualFold(filepath.Ext(p), ".m") && name == "objective-c" && looksLikeMatlab(data) {
			return Info{Name: ""}
		}
		return Info{Name: name}
	}
	if shebang := detectByShebang(data); shebang != "" {
		return Info{Name: shebang}
	}
	return Info{Name: ""}
}

func detectByPath(p string) string {
	base := filepath.Base(p)
	lowerBase := strings.ToLower(base)
	if lang, ok := basenameLanguages[lowerBase]; ok {
		return lang
	}
	if lang, ok := stemLanguages[lowerBase]; ok {
		return lang
	}
	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		return ""
	}
	if lang, ok := extensionLanguages[ext]; ok {
		return lang
	}
	stem := strings.TrimSuffix(lowerBase, ext)
	if stem == lowerBase {
		return ""
	}
	if lang, ok := extensionLanguages[filepath.Ext(stem)+ext]; ok {
		return lang
	}
	if lang, ok := extensionLanguages[filepath.Ext(stem)]; ok {
		return lang
	}
	if lang, ok := stemLanguages[stem]; ok {
		return lang
	}
	return ""
}

func detectByShebang(data []byte) string {
	if len(data) == 0 || !bytes.HasPrefix(data, []byte("#!")) {
		return ""
	}
	end := bytes.IndexByte(data, '\n')
	if end == -1 {
		end = len(data)
	}
	line := strings.ToLower(string(data[:end]))
	for key, lang := range shebangLanguages {
		if strings.Contains(line, key) {
			return lang
		}
	}
	return ""
}

func NormalizeLangName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}
	if canon, ok := langAliases[n]; ok {
		return canon
	}
	return n
}

func MatchesLang(info Info, allow []string) bool {
	if len(allow) == 0 {
		return true
	}
	detected := NormalizeLangName(info.Name)
	if detected == "" {
		return false
	}
	for _, raw := range allow {
		if NormalizeLangName(raw) == detected {
			return true
		}
	}
	return false
}

func KnownLanguage(name string) bool {
	if name == "" {
		return false
	}
	_, ok := languageStyles[NormalizeLangName(name)]
	return ok
}

var basenameLanguages = map[string]string{
	"makefile":          "make",
	"gnumakefile":       "make",
	"cmakelists.txt":    "cmake",
	"dockerfile":        "dockerfile",
	"podfile":           "ruby",
	"jenkinsfile":       "groovy",
	"vagrantfile":       "ruby",
	"justfile":          "make",
	"procfile":          "procfile",
	"gradle.properties": "properties",
	"gradlew":           "bash",
	"gradlew.bat":       "batch",
	"gemfile":           "ruby",
	"rakefile":          "ruby",
	"berksfile":         "ruby",
	"pyproject.toml":    "toml",
	"cargo.toml":        "toml",
	"cargo.lock":        "toml",
	"package.json":      "json",
	"package-lock.json": "json",
	"composer.json":     "json",
	"requirements.txt":  "pip",
	"setup.py":          "python",
	"pipfile":           "toml",
	"pipfile.lock":      "json",
	"pom.xml":           "xml",
	"tsconfig.json":     "json",
	"jsconfig.json":     "json",
	"config.ru":         "ruby",
}

var stemLanguages = map[string]string{
	"dockerfile":  "dockerfile",
	"justfile":    "make",
	"gradlew":     "bash",
	"gradlew.bat": "batch",
}

var extensionLanguages = map[string]string{
	".c":          "c",
	".h":          "c",
	".cc":         "cpp",
	".cp":         "cpp",
	".cpp":        "cpp",
	".cxx":        "cpp",
	".hh":         "cpp",
	".hpp":        "cpp",
	".hxx":        "cpp",
	".m":          "objective-c",
	".mm":         "objective-cpp",
	".go":         "go",
	".js":         "javascript",
	".mjs":        "javascript",
	".cjs":        "javascript",
	".jsx":        "javascriptreact",
	".ts":         "typescript",
	".tsx":        "typescriptreact",
	".coffee":     "coffeescript",
	".litcoffee":  "coffeescript",
	".py":         "python",
	".pyw":        "python",
	".pyi":        "python",
	".rb":         "ruby",
	".rake":       "ruby",
	".gemspec":    "ruby",
	".php":        "php",
	".php5":       "php",
	".phtml":      "php",
	".cs":         "csharp",
	".vb":         "vb",
	".fs":         "fsharp",
	".java":       "java",
	".kt":         "kotlin",
	".kts":        "kotlin",
	".scala":      "scala",
	".groovy":     "groovy",
	".gradle":     "gradle",
	".swift":      "swift",
	".rs":         "rust",
	".dart":       "dart",
	".erl":        "erlang",
	".hrl":        "erlang",
	".ex":         "elixir",
	".exs":        "elixir",
	".hs":         "haskell",
	".lhs":        "haskell",
	".clj":        "clojure",
	".cljs":       "clojure",
	".cljc":       "clojure",
	".edn":        "clojure",
	".elm":        "elm",
	".ml":         "ocaml",
	".mli":        "ocaml",
	".pas":        "pascal",
	".adb":        "ada",
	".ads":        "ada",
	".sh":         "shell",
	".bash":       "shell",
	".zsh":        "shell",
	".ksh":        "shell",
	".fish":       "fish",
	".csh":        "shell",
	".tcsh":       "shell",
	".ps1":        "powershell",
	".psm1":       "powershell",
	".psd1":       "powershell",
	".bat":        "batch",
	".cmd":        "batch",
	".sql":        "sql",
	".psql":       "sql",
	".plsql":      "sql",
	".pgsql":      "sql",
	".json":       "json",
	".json5":      "json",
	".hjson":      "json",
	".yaml":       "yaml",
	".yml":        "yaml",
	".toml":       "toml",
	".ini":        "ini",
	".cfg":        "ini",
	".conf":       "ini",
	".properties": "properties",
	".env":        "dotenv",
	".txt":        "text",
	".md":         "markdown",
	".markdown":   "markdown",
	".mdx":        "markdown",
	".rst":        "rst",
	".adoc":       "asciidoc",
	".tex":        "latex",
	".bib":        "bibtex",
	".html":       "html",
	".htm":        "html",
	".xhtml":      "html",
	".vue":        "vue",
	".svelte":     "svelte",
	".xml":        "xml",
	".svg":        "xml",
	".plist":      "xml",
	".xaml":       "xml",
	".wsdl":       "xml",
	".csproj":     "xml",
	".fsproj":     "xml",
	".vbproj":     "xml",
	".sln":        "ini",
	".css":        "css",
	".scss":       "scss",
	".sass":       "sass",
	".less":       "less",
	".styl":       "stylus",
	".proto":      "proto",
	".thrift":     "thrift",
	".avdl":       "avro",
	".graphql":    "graphql",
	".gql":        "graphql",
	".hcl":        "hcl",
	".tf":         "terraform",
	".tfvars":     "terraform",
	".nomad":      "hcl",
	".cue":        "cue",
	".bzl":        "starlark",
	".star":       "starlark",
	".bazel":      "starlark",
	".build":      "starlark",
	".dockerfile": "dockerfile",
	".mk":         "make",
	".make":       "make",
	".ninja":      "ninja",
	".sqlx":       "sql",
	".tpl":        "gotemplate",
	".tmpl":       "gotemplate",
	".jinja":      "jinja",
	".jinja2":     "jinja",
	".twig":       "twig",
	".hbs":        "handlebars",
	".mustache":   "handlebars",
	".djhtml":     "django",
	".liquid":     "liquid",
	".pug":        "pug",
	".jade":       "pug",
	".haml":       "haml",
	".ejs":        "ejs",
	".erb":        "erb",
	".aspx":       "aspnet",
	".ascx":       "aspnet",
	".cshtml":     "aspnet",
	".vbhtml":     "aspnet",
	".cl":         "common-lisp",
	".lisp":       "common-lisp",
	".scm":        "scheme",
	".ss":         "scheme",
	".rkt":        "racket",
	".v":          "verilog",
	".sv":         "systemverilog",
	".svh":        "systemverilog",
	".vh":         "verilog",
	".pyx":        "cython",
	".pxd":        "cython",
	".pxi":        "cython",
	".apex":       "apex",
	".cls":        "apex",
	".trigger":    "apex",
	".ps1xml":     "xml",
	".pssc":       "powershell",
	".ahk":        "ahk",
	".au3":        "autoit",
	".nim":        "nim",
	".zig":        "zig",
	".smali":      "smali",
	".jl":         "julia",
	".rego":       "rego",
	".qy":         "sql",
}

var langAliases = map[string]string{
	"c#":       "csharp",
	"cs":       "csharp",
	"c++":      "cpp",
	"cc":       "cpp",
	"h++":      "cpp",
	"hpp":      "cpp",
	"hh":       "cpp",
	"htm":      "html",
	"js":       "javascript",
	"mjs":      "javascript",
	"cjs":      "javascript",
	"jsx":      "javascriptreact",
	"ts":       "typescript",
	"tsx":      "typescriptreact",
	"kt":       "kotlin",
	"rb":       "ruby",
	"py":       "python",
	"ps":       "powershell",
	"ps1":      "powershell",
	"psm1":     "powershell",
	"bash":     "shell",
	"sh":       "shell",
	"zsh":      "shell",
	"mk":       "make",
	"tf":       "terraform",
	"yml":      "yaml",
	"yaml":     "yaml",
	"md":       "markdown",
	"markdown": "markdown",
	"sql":      "sql",
}

func looksLikeMatlab(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	lines := strings.Split(string(sample), "\n")
	sawMatlabKeyword := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "%") {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "@interface") || strings.HasPrefix(lower, "@implementation") || strings.HasPrefix(lower, "#import") {
			return false
		}
		if strings.HasPrefix(lower, "function") || strings.HasPrefix(lower, "classdef") {
			return true
		}
		if strings.HasPrefix(lower, "properties") || strings.HasPrefix(lower, "methods") {
			sawMatlabKeyword = true
		}
	}
	return sawMatlabKeyword
}

func CanonicalDetectLangs(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		norm := NormalizeLangName(raw)
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	return out
}

var shebangLanguages = map[string]string{
	"python":        "python",
	"python3":       "python",
	"python2":       "python",
	"pypy":          "python",
	"node":          "javascript",
	"deno":          "javascript",
	"perl":          "perl",
	"ruby":          "ruby",
	"php":           "php",
	"bash":          "shell",
	"sh":            "shell",
	"zsh":           "shell",
	"ksh":           "shell",
	"fish":          "fish",
	"pwsh":          "powershell",
	"powershell":    "powershell",
	"dotnet-script": "csharp",
	"lua":           "lua",
	"groovy":        "groovy",
	"swift":         "swift",
	"r":             "r",
	"scheme":        "scheme",
	"guile":         "scheme",
	"awk":           "awk",
	"sed":           "sed",
	"elixir":        "elixir",
	"escript":       "erlang",
}

var languageStyles = map[string]struct{}{
	"c":               {},
	"cpp":             {},
	"objective-c":     {},
	"objective-cpp":   {},
	"go":              {},
	"javascript":      {},
	"javascriptreact": {},
	"typescript":      {},
	"typescriptreact": {},
	"coffeescript":    {},
	"python":          {},
	"ruby":            {},
	"php":             {},
	"csharp":          {},
	"vb":              {},
	"fsharp":          {},
	"java":            {},
	"kotlin":          {},
	"scala":           {},
	"groovy":          {},
	"swift":           {},
	"rust":            {},
	"dart":            {},
	"erlang":          {},
	"elixir":          {},
	"haskell":         {},
	"clojure":         {},
	"elm":             {},
	"ocaml":           {},
	"pascal":          {},
	"ada":             {},
	"shell":           {},
	"fish":            {},
	"powershell":      {},
	"batch":           {},
	"sql":             {},
	"json":            {},
	"yaml":            {},
	"toml":            {},
	"ini":             {},
	"properties":      {},
	"dotenv":          {},
	"markdown":        {},
	"rst":             {},
	"asciidoc":        {},
	"latex":           {},
	"html":            {},
	"vue":             {},
	"svelte":          {},
	"xml":             {},
	"css":             {},
	"scss":            {},
	"sass":            {},
	"less":            {},
	"stylus":          {},
	"proto":           {},
	"thrift":          {},
	"graphql":         {},
	"hcl":             {},
	"terraform":       {},
	"cue":             {},
	"starlark":        {},
	"dockerfile":      {},
	"make":            {},
	"ninja":           {},
	"gotemplate":      {},
	"jinja":           {},
	"twig":            {},
	"handlebars":      {},
	"django":          {},
	"liquid":          {},
	"pug":             {},
	"haml":            {},
	"ejs":             {},
	"erb":             {},
	"aspnet":          {},
	"common-lisp":     {},
	"scheme":          {},
	"racket":          {},
	"verilog":         {},
	"systemverilog":   {},
	"cython":          {},
	"apex":            {},
	"ahk":             {},
	"autoit":          {},
	"nim":             {},
	"zig":             {},
	"smali":           {},
	"julia":           {},
	"rego":            {},
	"text":            {},
	"pip":             {},
	"gradle":          {},
	"cmake":           {},
	"procfile":        {},
}
