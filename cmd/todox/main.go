package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/example/todox/internal/engine"
	"github.com/example/todox/internal/util"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		serveCmd(os.Args[2:])
		return
	}
	scanCmd(os.Args[1:])
}

type scanConfig struct {
	opts        engine.Options
	output      string
	withComment bool
	withMessage bool
	withAge     bool
	showHelp    bool
	helpLang    string
	sortKey     string
}

func parseScanArgs(args []string, envLang string) (scanConfig, error) {
	cfg := scanConfig{helpLang: strings.ToLower(envLang)}
	if cfg.helpLang == "" {
		cfg.helpLang = "en"
	}

	fs := flag.NewFlagSet("todox", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	typ := fs.String("type", "both", "todo|fixme|both")
	mode := fs.String("mode", "last", "last|first")
	author := fs.String("author", "", "filter by author name/email (regexp)")
	output := fs.String("output", "table", "table|tsv|json")
	withComment := fs.Bool("with-comment", false, "show line text (from TODO/FIXME)")
	withMessage := fs.Bool("with-message", false, "show commit subject (1st line)")
	withAge := fs.Bool("with-age", false, "show AGE column (days since author date)")
	full := fs.Bool("full", false, "shortcut for --with-comment --with-message (with default truncate)")
	withSnippet := fs.Bool("with-snippet", false, "alias of --with-comment")
	truncAll := fs.Int("truncate", 0, "truncate comment/message to N runes (0=unlimited)")
	truncComment := fs.Int("truncate-comment", 0, "truncate comment only (0=unlimited)")
	truncMessage := fs.Int("truncate-message", 0, "truncate message only (0=unlimited)")
	noIgnoreWS := fs.Bool("no-ignore-ws", false, "include whitespace-only changes in blame")
	noProgress := fs.Bool("no-progress", false, "disable progress/ETA")
	forceProg := fs.Bool("progress", false, "force progress even when piped")
	lang := fs.String("lang", "", "help language (en|ja)")
	jobs := fs.Int("jobs", runtime.NumCPU(), "max parallel workers")
	repo := fs.String("repo", ".", "repo root (default: current dir)")
	sortKey := fs.String("sort", "", "sort order (first step: -age)")

	shortMap := map[string]string{
		"-t": "--type",
		"-m": "--mode",
		"-a": "--author",
		"-o": "--output",
	}

	normalized := make([]string, 0, len(args))
	valueExpect := map[string]bool{"--sort": true}
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
			if len(normalized) > 0 && valueExpect[normalized[len(normalized)-1]] {
				normalized = append(normalized, arg)
				continue
			}
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

	if err := fs.Parse(normalized); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			cfg.showHelp = true
			return cfg, nil
		}
		return cfg, err
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

	cfg.opts = engine.Options{
		Type:         *typ,
		Mode:         *mode,
		AuthorRegex:  *author,
		WithComment:  *withComment,
		WithMessage:  *withMessage,
		WithAge:      *withAge,
		TruncAll:     *truncAll,
		TruncComment: *truncComment,
		TruncMessage: *truncMessage,
		IgnoreWS:     !*noIgnoreWS,
		Jobs:         *jobs,
		RepoDir:      *repo,
		Progress:     util.ShouldShowProgress(*forceProg, *noProgress),
	}
	cfg.output = *output
	cfg.withComment = *withComment
	cfg.withMessage = *withMessage
	cfg.withAge = *withAge
	cfg.sortKey = strings.TrimSpace(*sortKey)

	return cfg, nil
}

func scanCmd(args []string) {
	envLang := os.Getenv("GIT_TODO_AUTHORS_LANG")
	if envLang == "" {
		envLang = os.Getenv("GTA_LANG")
	}

	cfg, err := parseScanArgs(args, envLang)
	if err != nil {
		log.Fatal(err)
	}

	if cfg.showHelp {
		printHelp(cfg.helpLang)
		return
	}

	res, err := engine.Run(cfg.opts)
	if err != nil {
		log.Fatal(err)
	}

	if err := applySort(res.Items, cfg.sortKey); err != nil {
		log.Fatal(err)
	}

	switch strings.ToLower(cfg.output) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			log.Fatal(err)
		}
	case "tsv":
		printTSV(res, cfg.opts)
	default: // table
		printTable(res, cfg.opts)
	}

	if res.ErrorCount > 0 {
		reportErrors(res)
		os.Exit(2)
	}
}

func applySort(items []engine.Item, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	switch key {
	case "-age":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].AgeDays == items[j].AgeDays {
				if items[i].File == items[j].File {
					return items[i].Line < items[j].Line
				}
				return items[i].File < items[j].File
			}
			return items[i].AgeDays > items[j].AgeDays
		})
		return nil
	default:
		return fmt.Errorf("invalid --sort: %s", key)
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

Output:
  -o, --output {table|tsv|json}  Output format (default: table)

Extra columns (hidden by default):
      --full                     Show both COMMENT and MESSAGE columns
      --with-comment             Show COMMENT (line text trimmed to start at TODO/FIXME)
      --with-message             Show MESSAGE (commit subject = 1st line)
      --with-snippet             Alias of --with-comment (backward compatible)
      --with-age                 Show AGE column (days since author date)

Truncation (applies to COMMENT / MESSAGE only):
      --truncate N               Truncate both to N chars (0 = unlimited)
      --truncate-comment N       Truncate comment to N chars (0 = unlimited)
      --truncate-message N       Truncate message to N chars (0 = unlimited)
                                 Tip: --full alone defaults to 120 chars for both.

Sorting (first step):
      --sort -age                Oldest TODO/FIXME first (fallback to file:line)

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

  7) Machine-friendly TSV:
       todox --full -o tsv > todo_full.tsv

  8) Progress control:
       todox --no-progress
       todox --progress | head   # force progress even when piped

  9) Include whitespace-only changes in blame:
       todox --no-ignore-ws

 10) Oldest TODO/FIXME first:
       todox --with-age --sort -age
`

const helpJapanese = `todox — リポジトリ内の TODO / FIXME の「誰が書いたか」を特定するツール。

使い方:
  todox [options]

検索と属性付け:
  -t, --type {todo|fixme|both}   検索対象（既定: both）
  -m, --mode {last|first}        last : その行を最後に変更した人（git blame で高速）
                                 first: その TODO/FIXME を最初に入れた人（git log -L で低速）
  -a, --author REGEX             作者名またはメールを正規表現でフィルタ

出力:
  -o, --output {table|tsv|json}  出力形式（既定: table）

追加カラム（既定は非表示）:
      --full                     COMMENT と MESSAGE を両方表示
      --with-comment             COMMENT（行テキスト。TODO/FIXME から表示）
      --with-message             MESSAGE（コミットメッセージの1行目）
      --with-snippet             --with-comment の別名（後方互換）
      --with-age                 AGE列（日数）を表示

トランケート（COMMENT/MESSAGE のみ対象）:
      --truncate N               両方を N 文字で切り詰め（0=無制限）
      --truncate-comment N       コメントのみ N 文字で切り詰め（0=無制限）
      --truncate-message N       メッセージのみ N 文字で切り詰め（0=無制限）
                                 ※ --full だけ指定した場合は既定で 120 文字

並び替え（第一歩）:
      --sort -age                古い順（AGE降順、同値は file:line）

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

  7) 機械処理向け TSV 出力:
       todox --full -o tsv > todo_full.tsv

  8) 進捗制御:
       todox --no-progress
       todox --progress | head   # パイプでも進捗を表示

  9) 空白変更も blame 対象にする:
       todox --no-ignore-ws

 10) 古いTODO/FIXMEを優先:
       todox --with-age --sort -age
`

const webAppHTML = `<!doctype html>
<html><head><meta charset="utf-8"/><title>todox</title>
<style>
body{font:14px/1.45 system-ui, sans-serif; margin:20px;}
table{border-collapse:collapse;width:100%;}
th,td{border:1px solid #ddd;padding:6px 8px;vertical-align:top;}
thead{background:#fafafa;position:sticky;top:0;}
code{font-family:ui-monospace, SFMono-Regular, Menlo, Consolas, monospace}
label{margin-right:12px}
input[type=text]{width:240px}
.small{color:#666}
.errors{background:#fff4f4;border:1px solid #f2c6c6;padding:8px;margin:12px 0;}
.error-banner{display:none;align-items:center;justify-content:space-between;background:#ffecec;border:1px solid #f5a9a9;color:#8a1f1f;padding:8px 12px;margin:12px 0;}
.error-banner button{background:transparent;border:0;font-size:18px;line-height:1;cursor:pointer;color:inherit;padding:0;margin-left:12px;}
</style></head><body>
<h2>todox</h2>
<div id="error-banner" class="error-banner" role="alert">
 <span id="error-message"></span>
 <button type="button" id="error-close" aria-label="Close">&times;</button>
</div>
<form id="f">
<label>type:
<select name="type">
	<option>both</option>
	<option>todo</option>
	<option>fixme</option>
</select></label>
<label>mode:
<select name="mode">
	<option>last</option>
	<option>first</option>
</select></label>
<label>author (regexp): <input name="author" type="text"></label>
<label><input type="checkbox" name="with_comment"> comment</label>
<label><input type="checkbox" name="with_message"> message</label>
<label>truncate: <input type="text" name="truncate" value="120"></label>
<button>Scan</button>
</form>
<p class="small">Tip: Same params as CLI. Example: <code>/api/scan?type=todo&mode=first&with_comment=1</code></p>
<div id="out"></div>
<script>
const f=document.getElementById('f'), out=document.getElementById('out');
const banner=document.getElementById('error-banner');
const bannerMsg=document.getElementById('error-message');
const bannerClose=document.getElementById('error-close');
function showError(msg){
 bannerMsg.textContent=msg;
 banner.style.display='flex';
}
function hideError(){
 banner.style.display='none';
 bannerMsg.textContent='';
}
bannerClose.addEventListener('click',(e)=>{
 e.preventDefault();
 hideError();
});
f.onsubmit=async (e)=>{
 e.preventDefault();
 hideError();
 try{
  const q=new URLSearchParams(new FormData(f));
  const res=await fetch('/api/scan?'+q.toString());
  if(!res.ok){
   let msg='HTTP '+res.status;
   if(res.statusText){msg+=' '+res.statusText;}
   try{
    const text=(await res.text()).trim();
    if(text){msg+=': '+text;}
   }catch(_){}
   throw new Error(msg);
  }
  const data=await res.json();
  out.innerHTML=render(data);
 }catch(err){
  const msg=err&&err.message?err.message:'予期しないエラーが発生しました';
  showError(msg);
 }
}
function escText(s){
 const value=s==null?'':String(s);
 return value.replace(/[&<>]/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));
}
function escAttr(s){
 const value=s==null?'':String(s);
 return value.replace(/[&<>"']/g, c=>({
  '&':'&amp;',
  '<':'&lt;',
  '>':'&gt;',
  '"':'&quot;',
  "'":'&#39;'
 }[c]));
}
function render(data){
 const rows=data.items||[];
 const errs=data.errors||[];
 let parts=[];
 if(errs.length){
        let list='<ul>';
        for(const e of errs){
                const fileRaw=e.file||'(unknown)';
                const lineRaw=e.line>0?String(e.line):'—';
                const loc=fileRaw+':'+lineRaw;
                list+='<li><code>'+escText(loc)+'</code> ['+escText(e.stage||'git')+'] '+escText(e.message||'')+'</li>';
        }
        list+='</ul>';
        parts.push('<div class="errors"><strong>'+errs.length+' error(s)</strong>'+list+'</div>');
 }
 if(!rows||rows.length===0){
        parts.push('<p>No results.</p>');
        return parts.join('');
 }
 let h='<table><thead><tr><th>TYPE</th><th>AUTHOR</th><th>EMAIL</th><th>DATE</th><th>COMMIT</th><th>LOCATION</th><th>COMMENT</th><th>MESSAGE</th></tr></thead><tbody>';
 for(const r of rows){
       h+='<tr>'+
               '<td>'+escText(r.kind||'')+'</td>'+
               '<td>'+escText(r.author||'')+'</td>'+
               '<td>'+escText(r.email||'')+'</td>'+
               '<td>'+escText(r.date||'')+'</td>'+
               '<td><code>'+escText((r.commit||'').slice(0,8))+'</code></td>'+
               (()=>{
                       const fileRaw=r.file==null?'':String(r.file);
                       const lineRaw=r.line==null||r.line===0?'':String(r.line);
                       const loc=fileRaw+':'+lineRaw;
                       return '<td><code>'+escText(loc)+'</code></td>';
               })()+
               '<td>'+escText(r.comment||'')+'</td>'+
               '<td>'+escText(r.message||'')+'</td>'+
               '</tr>';
 }
 h+='</tbody></table>';
 parts.push(h);
 return parts.join('');
}
</script></body></html>`

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var port = fs.Int("p", 8080, "port")
	var repo = fs.String("repo", ".", "repo root")
	_ = fs.Parse(args)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		_, _ = io.WriteString(w, webAppHTML)
	})

	http.HandleFunc("/api/scan", apiScanHandler(*repo))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("todox serve listening on %s (repo=%s)", addr, mustAbs(*repo))
	log.Fatal(http.ListenAndServe(addr, nil))
}

func get(q map[string][]string, k, def string) string {
	if v := q[k]; len(v) > 0 && v[0] != "" {
		return v[0]
	}
	return def
}

func parseBoolParam(q map[string][]string, key string) (bool, error) {
	vals, ok := q[key]
	if !ok || len(vals) == 0 {
		return false, nil
	}
	raw := strings.TrimSpace(vals[0])
	if raw == "" {
		return false, nil
	}

	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid value for %s: %q", key, raw)
	}
}

func apiScanHandler(repoDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		withComment, err := parseBoolParam(q, "with_comment")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		withMessage, err := parseBoolParam(q, "with_message")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		withAge, err := parseBoolParam(q, "with_age")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		truncAll, err := parseIntParam(q, "truncate")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		truncComment, err := parseIntParam(q, "truncate_comment")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		truncMessage, err := parseIntParam(q, "truncate_message")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		opts := engine.Options{
			Type:         get(q, "type", "both"),
			Mode:         get(q, "mode", "last"),
			AuthorRegex:  q.Get("author"),
			WithComment:  withComment,
			WithMessage:  withMessage,
			WithAge:      withAge,
			TruncAll:     truncAll,
			TruncComment: truncComment,
			TruncMessage: truncMessage,
			IgnoreWS:     true,
			Jobs:         runtime.NumCPU(),
			RepoDir:      repoDir,
			Progress:     false,
		}
		if opts.WithComment && opts.WithMessage &&
			opts.TruncAll == 0 && opts.TruncComment == 0 && opts.TruncMessage == 0 {
			opts.TruncAll = 120
		}
		res, err := engine.Run(opts)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		sortKey := strings.TrimSpace(q.Get("sort"))
		if err := applySort(res.Items, sortKey); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}
}

func parseIntParam(q map[string][]string, key string) (int, error) {
	vals, ok := q[key]
	if !ok || len(vals) == 0 {
		return 0, nil
	}
	raw := strings.TrimSpace(vals[0])
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %q", key, raw)
	}
	return n, nil
}

func printTSV(res *engine.Result, opts engine.Options) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0) // tabs only
	write := func(text string) {
		mustFprintln(w, text)
	}
	header := []string{"TYPE", "AUTHOR", "EMAIL", "DATE"}
	if opts.WithAge {
		header = append(header, "AGE")
	}
	header = append(header, "COMMIT", "LOCATION")
	if res.HasComment && res.HasMessage {
		header = append(header, "COMMENT", "MESSAGE")
	} else if res.HasComment {
		header = append(header, "COMMENT")
	} else if res.HasMessage {
		header = append(header, "MESSAGE")
	}
	write(strings.Join(header, "\t"))
	for _, it := range res.Items {
		loc := fmt.Sprintf("%s:%d", it.File, it.Line)
		base := []string{it.Kind, it.Author, it.Email, it.Date}
		if opts.WithAge {
			base = append(base, strconv.Itoa(it.AgeDays))
		}
		base = append(base, short(it.Commit), loc)
		if res.HasComment && res.HasMessage {
			base = append(base, it.Comment, it.Message)
		} else if res.HasComment {
			base = append(base, it.Comment)
		} else if res.HasMessage {
			base = append(base, it.Message)
		}
		for i := range base {
			base[i] = sanitizeField(base[i])
		}
		write(strings.Join(base, "\t"))
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

func printTable(res *engine.Result, opts engine.Options) {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	write := func(text string) {
		mustFprintln(w, text)
	}
	header := []string{"TYPE", "AUTHOR", "EMAIL", "DATE"}
	if opts.WithAge {
		header = append(header, "AGE")
	}
	header = append(header, "COMMIT", "LOCATION")
	if res.HasComment && res.HasMessage {
		header = append(header, "COMMENT", "MESSAGE")
	} else if res.HasComment {
		header = append(header, "COMMENT")
	} else if res.HasMessage {
		header = append(header, "MESSAGE")
	}
	write(strings.Join(header, "\t"))
	for _, it := range res.Items {
		loc := fmt.Sprintf("%s:%d", it.File, it.Line)
		base := []string{it.Kind, it.Author, it.Email, it.Date}
		if opts.WithAge {
			base = append(base, strconv.Itoa(it.AgeDays))
		}
		base = append(base, short(it.Commit), loc)
		if res.HasComment && res.HasMessage {
			base = append(base, it.Comment, it.Message)
		} else if res.HasComment {
			base = append(base, it.Comment)
		} else if res.HasMessage {
			base = append(base, it.Message)
		}
		for i := range base {
			base[i] = sanitizeField(base[i])
		}
		write(strings.Join(base, "\t"))
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
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
