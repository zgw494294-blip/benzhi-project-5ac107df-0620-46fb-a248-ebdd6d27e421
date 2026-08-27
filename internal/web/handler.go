package web

import (
	"net/http"
	"stagecaption/internal/service"
)

type Handler struct {
	Service *service.Service
	mux     *http.ServeMux
}

func New(s *service.Service) http.Handler {
	h := &Handler{Service: s, mux: http.NewServeMux()}
	h.routes()
	return securityHeaders(h.mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
