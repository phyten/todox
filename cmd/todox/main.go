package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

func scanCmd(args []string) {
	fs := flag.NewFlagSet("todox", flag.ExitOnError)

	var (
		typ          = fs.String("type", "both", "todo|fixme|both")
		mode         = fs.String("mode", "last", "last|first")
		author       = fs.String("author", "", "filter by author name/email (regexp)")
		output       = fs.String("output", "table", "table|tsv|json")
		withComment  = fs.Bool("with-comment", false, "show line text (from TODO/FIXME)")
		withMessage  = fs.Bool("with-message", false, "show commit subject (1st line)")
		full         = fs.Bool("full", false, "shortcut for --with-comment --with-message (with default truncate)")
		truncAll     = fs.Int("truncate", 0, "truncate comment/message to N runes (0=unlimited)")
		truncComment = fs.Int("truncate-comment", 0, "truncate comment only (0=unlimited)")
		truncMessage = fs.Int("truncate-message", 0, "truncate message only (0=unlimited)")
		noIgnoreWS   = fs.Bool("no-ignore-ws", false, "include whitespace-only changes in blame")
		noProgress   = fs.Bool("no-progress", false, "disable progress/ETA")
		forceProg    = fs.Bool("progress", false, "force progress even when piped")
		jobs         = fs.Int("jobs", runtime.NumCPU(), "max parallel workers")
		repo         = fs.String("repo", ".", "repo root (default: current dir)")
	)
	_ = fs.Parse(args)

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

	opts := engine.Options{
		Type:           *typ,
		Mode:           *mode,
		AuthorRegex:    *author,
		WithComment:    *withComment,
		WithMessage:    *withMessage,
		TruncAll:       *truncAll,
		TruncComment:   *truncComment,
		TruncMessage:   *truncMessage,
		IgnoreWS:       !*noIgnoreWS,
		Jobs:           *jobs,
		RepoDir:        *repo,
		Progress:       util.ShouldShowProgress(*forceProg, *noProgress),
	}

	res, err := engine.Run(opts)
	if err != nil {
		log.Fatal(err)
	}

	switch strings.ToLower(*output) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			log.Fatal(err)
		}
	case "tsv":
		printTSV(res, opts)
	default: // table
		printTable(res, opts)
	}
}

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var port = fs.Int("p", 8080, "port")
	var repo = fs.String("repo", ".", "repo root")
	_ = fs.Parse(args)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `<!doctype html>
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
</style></head><body>
<h2>todox</h2>
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
f.onsubmit=async (e)=>{
 e.preventDefault();
 const q=new URLSearchParams(new FormData(f));
 const res=await fetch('/api/scan?'+q.toString());
 const data=await res.json();
 out.innerHTML=render(data.items);
}
function esc(s){return (s||'').replace(/[&<>]/g, c=>({ '&':'&amp;','<':'&lt;','>':'&gt;'}[c]));}
function render(rows){
 if(!rows||rows.length===0) return '<p>No results.</p>';
 let h='<table><thead><tr><th>TYPE</th><th>AUTHOR</th><th>EMAIL</th><th>DATE</th><th>COMMIT</th><th>LOCATION</th><th>COMMENT</th><th>MESSAGE</th></tr></thead><tbody>';
 for(const r of rows){
	h+='<tr>'+
		'<td>'+esc(r.kind||'')+'</td>'+
		'<td>'+esc(r.author||'')+'</td>'+
		'<td>'+esc(r.email||'')+'</td>'+
		'<td>'+esc(r.date||'')+'</td>'+
		'<td><code>'+esc((r.commit||'').slice(0,8))+'</code></td>'+
		'<td><code>'+esc(r.file||'')+':'+(r.line||'')+'</code></td>'+
		'<td>'+esc(r.comment||'')+'</td>'+
		'<td>'+esc(r.message||'')+'</td>'+
		'</tr>';
 }
 h+='</tbody></table>'; return h;
}
</script></body></html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})

	http.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		opts := engine.Options{
			Type:         get(q, "type", "both"),
			Mode:         get(q, "mode", "last"),
			AuthorRegex:  q.Get("author"),
			WithComment:  q.Get("with_comment") != "",
			WithMessage:  q.Get("with_message") != "",
			TruncAll:     atoi(q.Get("truncate")),
			TruncComment: atoi(q.Get("truncate_comment")),
			TruncMessage: atoi(q.Get("truncate_message")),
			IgnoreWS:     true,
			Jobs:         runtime.NumCPU(),
			RepoDir:      *repo,
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})

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

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func printTSV(res *engine.Result, _ engine.Options) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0) // tabs only
	if res.HasComment && res.HasMessage {
		fmt.Fprintln(w, "TYPE\tAUTHOR\tEMAIL\tDATE\tCOMMIT\tLOCATION\tCOMMENT\tMESSAGE")
	} else if res.HasComment {
		fmt.Fprintln(w, "TYPE\tAUTHOR\tEMAIL\tDATE\tCOMMIT\tLOCATION\tCOMMENT")
	} else if res.HasMessage {
		fmt.Fprintln(w, "TYPE\tAUTHOR\tEMAIL\tDATE\tCOMMIT\tLOCATION\tMESSAGE")
	} else {
		fmt.Fprintln(w, "TYPE\tAUTHOR\tEMAIL\tDATE\tCOMMIT\tLOCATION")
	}
	for _, it := range res.Items {
		loc := fmt.Sprintf("%s:%d", it.File, it.Line)
		base := []string{it.Kind, it.Author, it.Email, it.Date, short(it.Commit), loc}
		if res.HasComment && res.HasMessage {
			base = append(base, it.Comment, it.Message)
		} else if res.HasComment {
			base = append(base, it.Comment)
		} else if res.HasMessage {
			base = append(base, it.Message)
		}
		fmt.Fprintln(w, strings.Join(base, "\t"))
	}
	_ = w.Flush()
}

func printTable(res *engine.Result, _ engine.Options) {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	if res.HasComment && res.HasMessage {
		fmt.Fprintln(w, "TYPE\tAUTHOR\tEMAIL\tDATE\tCOMMIT\tLOCATION\tCOMMENT\tMESSAGE")
	} else if res.HasComment {
		fmt.Fprintln(w, "TYPE\tAUTHOR\tEMAIL\tDATE\tCOMMIT\tLOCATION\tCOMMENT")
	} else if res.HasMessage {
		fmt.Fprintln(w, "TYPE\tAUTHOR\tEMAIL\tDATE\tCOMMIT\tLOCATION\tMESSAGE")
	} else {
		fmt.Fprintln(w, "TYPE\tAUTHOR\tEMAIL\tDATE\tCOMMIT\tLOCATION")
	}
	for _, it := range res.Items {
		loc := fmt.Sprintf("%s:%d", it.File, it.Line)
		base := []string{it.Kind, it.Author, it.Email, it.Date, short(it.Commit), loc}
		if res.HasComment && res.HasMessage {
			base = append(base, it.Comment, it.Message)
		} else if res.HasComment {
			base = append(base, it.Comment)
		} else if res.HasMessage {
			base = append(base, it.Message)
		}
		fmt.Fprintln(w, strings.Join(base, "\t"))
	}
	_ = w.Flush()
}

func short(s string) string {
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

func mustAbs(p string) string {
	a, _ := filepath.Abs(p)
	return a
}
