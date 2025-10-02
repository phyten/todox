package main

import (
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
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/phyten/todox/internal/engine"
	engineopts "github.com/phyten/todox/internal/engine/opts"
	"github.com/phyten/todox/internal/progress"
	"github.com/phyten/todox/internal/termcolor"
	"github.com/phyten/todox/internal/textutil"
)

var debugProgressDrops = envBool("TODOX_DEBUG_PROGRESS")

func main() {
	log.SetFlags(0)
	envEA := strings.TrimSpace(os.Getenv("TODOX_EASTASIAN"))
	if envEA == "1" || strings.EqualFold(envEA, "true") {
		runewidth.EastAsianWidth = true
	} else {
		runewidth.EastAsianWidth = false
	}
	runewidth.DefaultCondition = runewidth.NewCondition()
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
	sortKey     string
	fields      string
	showHelp    bool
	helpLang    string
	colorMode   termcolor.ColorMode
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
	values []string
}

func (m *multiFlag) String() string {
	if m == nil {
		return ""
	}
	return strings.Join(m.values, ",")
}

func (m *multiFlag) Set(value string) error {
	if value == "" {
		return nil
	}
	for _, piece := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(piece)
		if trimmed == "" {
			continue
		}
		m.values = append(m.values, trimmed)
	}
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
	colorMode := fs.String("color", "auto", "color output for tables: auto|always|never")
	withComment := fs.Bool("with-comment", false, "show line text (from TODO/FIXME)")
	withMessage := fs.Bool("with-message", false, "show commit subject (1st line)")
	withAge := fs.Bool("with-age", false, "show AGE column (table/tsv)")
	fields := fs.String("fields", "", "comma-separated columns for table/tsv (overrides --with-*)")
	full := fs.Bool("full", false, "shortcut for --with-comment --with-message (with default truncate)")
	withSnippet := fs.Bool("with-snippet", false, "alias of --with-comment")
	truncAll := fs.Int("truncate", 0, "truncate comment/message to N runes (0=unlimited)")
	truncComment := fs.Int("truncate-comment", 0, "truncate comment only (0=unlimited)")
	truncMessage := fs.Int("truncate-message", 0, "truncate message only (0=unlimited)")
	noIgnoreWS := fs.Bool("no-ignore-ws", false, "include whitespace-only changes in blame")
	noProgress := fs.Bool("no-progress", false, "disable progress/ETA")
	forceProg := fs.Bool("progress", false, "force progress even when piped")
	sortKey := fs.String("sort", "", "sort order (e.g. author,-date; default: file,line)")
	lang := fs.String("lang", "", "help language (en|ja)")
	jobsDefault := engineopts.Defaults(".").Jobs
	jobs := fs.Int("jobs", jobsDefault, "max parallel workers")
	repo := fs.String("repo", ".", "repo root (default: current dir)")
	var paths multiFlag
	var excludes multiFlag
	var pathRegex multiFlag
	fs.Var(&paths, "path", "limit search to given pathspec(s). repeatable / CSV")
	fs.Var(&excludes, "exclude", "exclude pathspec/glob(s). repeatable / CSV")
	fs.Var(&pathRegex, "path-regex", "post-filter files by Go regexp (OR). repeatable / CSV")
	excludeTypical := fs.Bool("exclude-typical", false, "apply typical excludes (vendor/**, node_modules/**, dist/**, build/**, target/**, *.min.*)")

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

	cfg.opts = engineopts.Defaults(*repo)
	cfg.opts.Type = *typ
	cfg.opts.Mode = *mode
	cfg.opts.AuthorRegex = *author
	cfg.opts.WithComment = *withComment
	cfg.opts.WithMessage = *withMessage
	cfg.opts.TruncAll = *truncAll
	cfg.opts.TruncComment = *truncComment
	cfg.opts.TruncMessage = *truncMessage
	cfg.opts.IgnoreWS = !*noIgnoreWS
	cfg.opts.Jobs = *jobs
	cfg.opts.RepoDir = *repo
	cfg.opts.Progress = progress.ShouldShowProgress(*forceProg, *noProgress)
	cfg.opts.Paths = paths.Slice()
	cfg.opts.Excludes = excludes.Slice()
	cfg.opts.PathRegex = pathRegex.Slice()
	cfg.opts.ExcludeTypical = *excludeTypical

	normalizedOutput, err := engineopts.NormalizeOutput(*output)
	if err != nil {
		return cfg, err
	}
	cfg.output = normalizedOutput
	cfg.withComment = *withComment
	cfg.withMessage = *withMessage
	cfg.withAge = *withAge
	cfg.sortKey = *sortKey
	cfg.fields = *fields

	parsedMode, err := termcolor.ParseMode(*colorMode)
	if err != nil {
		return cfg, &usageError{err: err}
	}
	cfg.colorMode = parsedMode

	return cfg, nil
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

	fieldSel, err := ResolveFields(cfg.fields, cfg.withComment, cfg.withMessage, cfg.withAge)
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

	res, err := engine.Run(cfg.opts)
	if err != nil {
		log.Fatal(err)
	}

	ApplySort(res.Items, sortSpec)
	res.HasComment = fieldSel.ShowComment
	res.HasMessage = fieldSel.ShowMessage
	res.HasAge = fieldSel.ShowAge

	switch strings.ToLower(cfg.output) {
	case "json":
		// NOTE: JSON は機械可読フォーマットのため常に非カラー。--color の指定は無視する。
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			log.Fatal(err)
		}
	case "tsv":
		// NOTE: TSV もターミナル以外で扱われることが多いため常に非カラー。--color の指定は無視する。
		printTSV(res, fieldSel)
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

Output:
  -o, --output {table|tsv|json}  Output format (default: table)
      --color {auto|always|never} Colorize table output (default: auto)
      --fields LIST             Columns for table/TSV (comma-separated; overrides --with-*)

Extra columns (hidden by default):
      --full                     Show both COMMENT and MESSAGE columns
      --with-comment             Show COMMENT (line text trimmed to start at TODO/FIXME)
      --with-message             Show MESSAGE (commit subject = 1st line)
      --with-snippet             Alias of --with-comment (backward compatible)
      --with-age                 Show AGE (days since author date) in table/TSV

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

出力:
  -o, --output {table|tsv|json}  出力形式（既定: table）
      --color {auto|always|never} 表形式に色付け（既定: auto）
      --fields LIST             table/TSV の列を指定（カンマ区切り。--with-* より優先）

追加カラム（既定は非表示）:
      --full                     COMMENT と MESSAGE を両方表示
      --with-comment             COMMENT（行テキスト。TODO/FIXME から表示）
      --with-message             MESSAGE（コミットメッセージの1行目）
      --with-snippet             --with-comment の別名（後方互換）
      --with-age                 AGE（日数）列を table/TSV に追加

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

  7) 機械処理向け TSV 出力:
       todox --full -o tsv > todo_full.tsv

  8) 進捗制御:
       todox --no-progress
       todox --progress | head   # パイプでも進捗を表示

  9) 空白変更も blame 対象にする:
       todox --no-ignore-ws
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
<label>path (CSV ok): <input name="path" type="text" placeholder="src,pkg"></label>
<label>exclude (CSV ok): <input name="exclude" type="text" placeholder="vendor/**"></label>
<label>path regex: <input name="path_regex" type="text" placeholder="\\.go$"></label>
<label><input type="checkbox" name="with_comment"> comment</label>
<label><input type="checkbox" name="with_message"> message</label>
<label><input type="checkbox" name="ignore_ws" checked> ignore whitespace</label>
<label><input type="checkbox" name="exclude_typical"> exclude typical dirs</label>
<label>jobs: <input type="number" name="jobs" min="1" max="64" inputmode="numeric" pattern="[0-9]*" placeholder="auto"></label>
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
  const fd=new FormData(f);
  const q=new URLSearchParams(fd);

  // ensure ignore_ws follows server default semantics (true by default)
  {
    const el=f.elements.namedItem('ignore_ws');
    if(el instanceof HTMLInputElement){
      if(el.checked){
        q.delete('ignore_ws');
      }else{
        q.set('ignore_ws','0');
      }
    }
  }

  // trim CSV inputs and drop empties for path filters
  for(const key of ['path','exclude','path_regex']){
    const values=q.getAll(key);
    q.delete(key);
    const cleaned=[];
    for(const value of values){
      if(value==null){continue;}
      for(const piece of String(value).split(',')){
        const trimmed=piece.trim();
        if(trimmed){cleaned.push(trimmed);}
      }
    }
    for(const entry of cleaned){
      q.append(key, entry);
    }
  }

  // checkbox only when enabled
  {
    const el=f.elements.namedItem('exclude_typical');
    if(el instanceof HTMLInputElement){
      if(el.checked){
        q.set('exclude_typical','1');
      }else{
        q.delete('exclude_typical');
      }
    }
  }

  // only send jobs when explicitly provided
  {
    const el=f.elements.namedItem('jobs');
    if(el instanceof HTMLInputElement){
      if((el.value||'').trim()===''){
        q.delete('jobs');
      }
    }
  }
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
 const hasAge=!!data.has_age;
 const hasComment=!!data.has_comment;
 const hasMessage=!!data.has_message;
 const headerCells=['TYPE','AUTHOR','EMAIL','DATE'];
 if(hasAge){headerCells.push('AGE');}
 headerCells.push('COMMIT','LOCATION');
 if(hasComment){headerCells.push('COMMENT');}
 if(hasMessage){headerCells.push('MESSAGE');}
 let h='<table><thead><tr>'+headerCells.map(hd=>'<th>'+hd+'</th>').join('')+'</tr></thead><tbody>';
 for(const r of rows){
       const cells=[];
       cells.push('<td>'+escText(r.kind||'')+'</td>');
       cells.push('<td>'+escText(r.author||'')+'</td>');
       cells.push('<td>'+escText(r.email||'')+'</td>');
       cells.push('<td>'+escText(r.date||'')+'</td>');
       if(hasAge){
               const ageRaw=r.age_days==null?'':String(r.age_days);
               cells.push('<td>'+escText(ageRaw)+'</td>');
       }
       cells.push('<td><code>'+escText((r.commit||'').slice(0,8))+'</code></td>');
       const fileRaw=r.file==null?'':String(r.file);
       const lineRaw=r.line==null||r.line===0?'':String(r.line);
       const loc=fileRaw+':'+lineRaw;
       cells.push('<td><code>'+escText(loc)+'</code></td>');
       if(hasComment){
               cells.push('<td>'+escText(r.comment||'')+'</td>');
       }
       if(hasMessage){
               cells.push('<td>'+escText(r.message||'')+'</td>');
       }
       h+='<tr>'+cells.join('')+'</tr>';
 }
 h+='</tbody></table>';
 parts.push(h);
 return parts.join('');
}
</script></body></html>`

type scanInputs struct {
	Options  engine.Options
	FieldSel FieldSelection
	SortSpec SortSpec
}

func prepareScanInputs(repoDir string, q url.Values) (scanInputs, error) {
	options := engineopts.Defaults(repoDir)
	options, err := engineopts.ApplyWebQueryToOptions(options, q)
	if err != nil {
		return scanInputs{}, err
	}

	withAge := false
	if vals := engineopts.SplitMulti(q["with_age"]); len(vals) > 0 {
		raw := vals[len(vals)-1]
		v, parseErr := engineopts.ParseBool(raw, "with_age")
		if parseErr != nil {
			return scanInputs{}, parseErr
		}
		withAge = v
	}

	fieldsParam := strings.Join(engineopts.SplitMulti(q["fields"]), ",")
	sortParam := ""
	if rawSort := q["sort"]; len(rawSort) > 0 {
		sortParam = strings.TrimSpace(rawSort[len(rawSort)-1])
	}

	fieldSel, err := ResolveFields(fieldsParam, options.WithComment, options.WithMessage, withAge)
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

	return scanInputs{Options: options, FieldSel: fieldSel, SortSpec: sortSpec}, nil
}

type streamObserver struct {
	ch   chan progress.Snapshot
	once sync.Once
}

func newStreamObserver(buffer int) (*streamObserver, <-chan progress.Snapshot) {
	ch := make(chan progress.Snapshot, buffer)
	return &streamObserver{ch: ch}, ch
}

func (o *streamObserver) Publish(s progress.Snapshot) {
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
	o.once.Do(func() {
		close(o.ch)
	})
}

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

		res, err := engine.Run(inputs.Options)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ApplySort(res.Items, inputs.SortSpec)
		res.HasComment = inputs.FieldSel.ShowComment
		res.HasMessage = inputs.FieldSel.ShowMessage
		res.HasAge = inputs.FieldSel.ShowAge
		w.Header().Set("Content-Type", "application/json")
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

		obs, snapCh := newStreamObserver(64)
		inputs.Options.Progress = false
		inputs.Options.ProgressObserver = obs

		resCh := make(chan *engine.Result, 1)
		errCh := make(chan error, 1)

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

		for snapCh != nil || resCh != nil || errCh != nil {
			select {
			case <-ctx.Done():
				return
			case <-pingTicker.C:
				if err := writeSSE(w, flusher, "ping", map[string]string{"ts": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
					return
				}
			case snap, ok := <-snapCh:
				if !ok {
					snapCh = nil
					continue
				}
				if err := writeSSE(w, flusher, "progress", progressPayload(snap)); err != nil {
					return
				}
			case res := <-resCh:
				ApplySort(res.Items, inputs.SortSpec)
				res.HasComment = inputs.FieldSel.ShowComment
				res.HasMessage = inputs.FieldSel.ShowMessage
				res.HasAge = inputs.FieldSel.ShowAge
				if err := writeSSE(w, flusher, "result", res); err != nil {
					return
				}
				resCh = nil
				errCh = nil
			case runErr := <-errCh:
				_ = writeSSE(w, flusher, "error", map[string]string{"message": runErr.Error()})
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		_, _ = io.WriteString(w, webAppHTML)
	})

	http.HandleFunc("/api/scan", apiScanHandler(*repo))
	http.HandleFunc("/api/scan/stream", apiScanStreamHandler(*repo))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("todox serve listening on %s (repo=%s)", addr, mustAbs(*repo))
	log.Fatal(http.ListenAndServe(addr, nil))
}

func printTSV(res *engine.Result, sel FieldSelection) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0) // tabs only
	write := func(text string) {
		mustFprintln(w, text)
	}
	headers := make([]string, len(sel.Fields))
	for i, f := range sel.Fields {
		headers[i] = f.Header
	}
	write(strings.Join(headers, "\t"))
	for _, it := range res.Items {
		row := make([]string, len(sel.Fields))
		for i, f := range sel.Fields {
			row[i] = sanitizeField(formatFieldValue(it, f.Key))
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

func printTable(res *engine.Result, sel FieldSelection, colors tableColorConfig) {
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
			val := sanitizeField(formatFieldValue(it, f.Key))
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
