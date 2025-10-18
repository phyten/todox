package web

import (
	_ "embed"
	"html/template"
	"net/http"
	"sync"
)

const (
	stylesPath = "/assets/styles.css"
	scriptPath = "/assets/ui.js"
)

var (
	//go:embed templates/index.html
	indexHTML string
	indexOnce sync.Once
	indexTmpl *template.Template

	//go:embed assets/styles.css
	stylesCSS string

	//go:embed assets/ui.js
	scriptJS string
)

type indexData struct {
	StylesPath string
	ScriptPath string
}

// Register attaches handlers for the web UI assets to the provided mux.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc(stylesPath, stylesHandler)
	mux.HandleFunc(scriptPath, scriptHandler)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplate()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'")
	if err := tmpl.Execute(w, indexData{StylesPath: stylesPath, ScriptPath: scriptPath}); err != nil {
		http.Error(w, "template rendering failed", http.StatusInternalServerError)
	}
}

func stylesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(stylesCSS))
}

func scriptHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(scriptJS))
}

func loadTemplate() *template.Template {
	indexOnce.Do(func() {
		indexTmpl = template.Must(template.New("index").Parse(indexHTML))
	})
	return indexTmpl
}
