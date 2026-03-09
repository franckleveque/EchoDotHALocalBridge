package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/amimof/huego"
)

type Server struct {
	hue         ports.HueEmulationPort
	admin       ports.AdminPort
	authService ports.AuthService
	ip          string
}

func NewServer(bridge ports.BridgePort, authService ports.AuthService, ip string) *Server {
	return &Server{
		hue:         bridge,
		admin:       bridge,
		authService: authService,
		ip:          ip,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/description.xml", s.handleDescription)
	mux.HandleFunc("/api", s.handleAPI)
	mux.HandleFunc("/api/", s.handleAPI)

	// Admin routes with Basic Auth
	mux.HandleFunc("/admin/setup", s.handleAdminSetup)
	mux.Handle("/admin", s.withBasicAuth(http.HandlerFunc(s.handleAdmin)))
	mux.Handle("/admin/config", s.withBasicAuth(http.HandlerFunc(s.handleConfig)))
	mux.Handle("/admin/ha-entities", s.withBasicAuth(http.HandlerFunc(s.handleHAEntities)))
	mux.Handle("/admin/test-action", s.withBasicAuth(http.HandlerFunc(s.handleAdminTestAction)))

	return http.ListenAndServe(addr, s.loggingMiddleware(mux))
}

func (s *Server) withBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authService.Exists() {
			http.Error(w, "Forbidden - Initial setup required at /admin/setup", http.StatusForbidden)
			return
		}

		u, p, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ok, err := s.authService.Verify(r.Context(), u, p)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
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

func (s *Server) formatUniqueID(id string) string {
	numericID, _ := strconv.Atoi(id)
	return fmt.Sprintf("00:17:88:01:00:%02x:%02x:%02x-0b", (numericID>>16)&0xFF, (numericID>>8)&0xFF, numericID&0xFF)
}

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(data)
}

func (s *Server) toHueState(ds *model.DeviceState) *huego.State {
	if ds == nil {
		return nil
	}
	return &huego.State{
		On:        ds.On,
		Bri:       ds.Bri,
		Hue:       ds.Hue,
		Sat:       ds.Sat,
		Xy:        ds.Xy,
		Ct:        ds.Ct,
		Reachable: ds.Reachable,
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
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
				slog.Debug("HTTP request body", "method", r.Method, "path", r.URL.Path, "from", r.RemoteAddr, "body", bodyStr)
			}
		}

		next.ServeHTTP(lrw, r)
		slog.Info("HTTP request", "method", r.Method, "path", r.URL.Path, "from", r.RemoteAddr, "status", lrw.statusCode, "duration", time.Since(start))
	})
}
