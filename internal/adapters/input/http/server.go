package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/ports"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Server wires the HTTP routes. Handlers live in AdminHandlers, HueHandlers
// and LogsHandlers, embedded here.
type Server struct {
	*AdminHandlers
	*HueHandlers
	*LogsHandlers
	authService ports.AuthService
	ip          string
	logger      *slog.Logger
}

func NewServer(hue ports.HueEmulationPort, admin ports.AdminPort, authService ports.AuthService, ip string, logger *slog.Logger) *Server {
	return &Server{
		AdminHandlers: NewAdminHandlers(admin, authService, logger),
		HueHandlers:   NewHueHandlers(hue, logger),
		LogsHandlers:  NewLogsHandlers(logger),
		authService:   authService,
		ip:            ip,
		logger:        logger,
	}
}

func (s *Server) Mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/description.xml", s.handleDescription)

	// Hue API. Method-less patterns let handlers keep their own method checks.
	mux.HandleFunc("POST /api", s.handleRegister)
	mux.HandleFunc("POST /api/{$}", s.handleRegister)
	for _, p := range []string{"/api", "/api/{$}", "/api/{user}", "/api/{user}/{$}"} {
		mux.HandleFunc(p, s.handleFullState)
	}
	for _, p := range []string{"/api/{user}/lights", "/api/{user}/lights/{$}"} {
		mux.HandleFunc(p, s.handleGetLights)
	}
	mux.HandleFunc("/api/{user}/lights/{id}", s.handleGetLight)
	mux.HandleFunc("/api/{user}/lights/{id}/{$}", s.handleGetLight)
	mux.HandleFunc("/api/{user}/lights/{id}/state", s.handleSetLightState)
	mux.HandleFunc("/api/", s.handleAPIUnknown)

	// Admin routes
	mux.Handle("GET /admin/setup", s.withSetupGuard(http.HandlerFunc(s.handleAdminSetupPage)))
	mux.Handle("POST /admin/setup", s.withSetupGuard(http.HandlerFunc(s.handleAdminSetupSubmit)))
	mux.HandleFunc("/admin/setup", s.handleMethodNotAllowed)
	mux.Handle("/admin", s.withBasicAuth(http.HandlerFunc(s.handleAdmin)))
	mux.Handle("GET /admin/config", s.withBasicAuth(http.HandlerFunc(s.handleConfigGet)))
	mux.Handle("POST /admin/config", s.withBasicAuth(http.HandlerFunc(s.handleConfigPost)))
	mux.Handle("/admin/config", s.withBasicAuth(http.HandlerFunc(s.handleMethodNotAllowed)))
	mux.Handle("/admin/ha-entities", s.withBasicAuth(http.HandlerFunc(s.handleHAEntities)))
	mux.Handle("/admin/test-action", s.withBasicAuth(http.HandlerFunc(s.handleAdminTestAction)))
	mux.Handle("/admin/logs", s.withBasicAuth(http.HandlerFunc(s.handleLogsPage)))
	mux.Handle("/admin/logs/data", s.withBasicAuth(http.HandlerFunc(s.handleLogsData)))

	return mux
}

func (s *Server) Handler() http.Handler {
	return s.loggingMiddleware(s.Mux())
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) withBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authService.Exists() {
			s.logger.Warn("HTTP: admin access refused, initial setup required", "path", r.URL.Path, "from", r.RemoteAddr)
			http.Error(w, "Forbidden - Initial setup required at /admin/setup", http.StatusForbidden)
			return
		}

		u, p, ok := r.BasicAuth()
		if !ok {
			s.logger.Debug("HTTP: no basic auth credentials", "path", r.URL.Path, "from", r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ok, err := s.authService.Verify(r.Context(), u, p)
		if err != nil {
			s.logger.Error("HTTP: credential verification failed", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !ok {
			s.logger.Warn("HTTP: invalid credentials", "path", r.URL.Path, "from", r.RemoteAddr, "username", u)
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		target := "/admin"
		if !s.authService.Exists() {
			target = "/admin/setup"
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleDescription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8" ?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
<specVersion>
<major>1</major>
<minor>0</minor>
</specVersion>
<URLBase>http://%s:80/</URLBase>
<device>
<deviceType>urn:schemas-upnp-org:device:Basic:1</deviceType>
<friendlyName>Philips hue (%s)</friendlyName>
<manufacturer>Royal Philips Electronics</manufacturer>
<manufacturerURL>http://www.philips.com</manufacturerURL>
<modelDescription>Philips hue Personal Wireless Lighting</modelDescription>
<modelName>Philips hue bridge 2012</modelName>
<modelNumber>929000226503</modelNumber>
<modelURL>http://www.meethue.com</modelURL>
<serialNumber>001788102201</serialNumber>
<UDN>uuid:2f402f80-da50-11e1-9b23-001788102201</UDN>
<presentationURL>admin</presentationURL>
</device>
</root>`, s.ip, s.ip)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(data)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Check if we should log the body (Alexa related routes)
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			path := r.URL.Path
			if path == "/description.xml" || path == "/api" || path == "/api/" || strings.HasPrefix(path, "/api/") {
				bodyBytes, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				bodyStr := string(bodyBytes)
				s.logger.Debug("HTTP request body", "method", r.Method, "path", r.URL.Path, "from", r.RemoteAddr, "body", bodyStr)
			}
		}

		next.ServeHTTP(lrw, r)
		// The logs page polls this route; logging it would flood the ring and evict real entries.
		if r.URL.Path == "/admin/logs/data" {
			return
		}
		s.logger.Info("HTTP request", "method", r.Method, "path", r.URL.Path, "from", r.RemoteAddr, "status", lrw.statusCode, "duration", time.Since(start))
	})
}
