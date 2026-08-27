package web

import (
	_ "embed"
	"net/http"
)

//go:embed static/index.html
var indexHTML []byte

//go:embed static/app.css
var appCSS []byte

//go:embed static/app-extra.css
var appExtraCSS []byte

//go:embed static/app.js
var appJS []byte

func (h *Handler) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}
func (h *Handler) CSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(appCSS)
}
func (h *Handler) ExtraCSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(appExtraCSS)
}
func (h *Handler) JSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write(appJS)
}
