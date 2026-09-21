package signaling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	inputsink "github.com/danilostorm/storplay/host/internal/input"
	storagemedia "github.com/danilostorm/storplay/host/internal/media"
	"github.com/danilostorm/storplay/host/internal/session"
	"github.com/danilostorm/storplay/host/internal/sunshine"
)

type Server struct {
	Addr        string
	StunURL     string
	WebDir      string
	Sunshine    *sunshine.Client
	TestPattern bool
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	mux.HandleFunc("/api/info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":               "StorPlay Host",
			"version":            "0.1.0-dev",
			"signaling":          "/ws/session",
			"mediaReady":         true,
			"inputReady":         true,
			"sunshineConfigured": s.Sunshine != nil,
			"testPattern":        s.TestPattern,
		})
	})

	mux.HandleFunc("/api/sunshine/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.Sunshine == nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"available": false,
				"error":     "Sunshine adapter is disabled",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
		defer cancel()

		info, err := s.Sunshine.Probe(ctx)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"available": false,
				"error":     err.Error(),
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"available": true,
			"server":    info,
			"pairing":   s.Sunshine.PairingStatus(),
		})
	})

	mux.HandleFunc("/api/sunshine/pair/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !requireLocalAdmin(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST required"})
			return
		}
		if s.Sunshine == nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "Sunshine adapter is disabled"})
			return
		}

		status, err := s.Sunshine.StartPairing()
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"state": "error", "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(status)
	})

	mux.HandleFunc("/api/sunshine/pair/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.Sunshine == nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"state": "error", "error": "Sunshine adapter is disabled"})
			return
		}
		_ = json.NewEncoder(w).Encode(s.Sunshine.PairingStatus())
	})

	mux.HandleFunc("/api/sunshine/apps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.Sunshine == nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "Sunshine adapter is disabled"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()
		apps, err := s.Sunshine.AppList(ctx)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"apps": apps})
	})

	mux.HandleFunc("/api/sunshine/session", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.Sunshine == nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "Sunshine adapter is disabled"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"session": s.Sunshine.ActiveSession()})
	})

	mux.HandleFunc("/api/sunshine/launch", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !requireLocalAdmin(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST required"})
			return
		}
		if s.Sunshine == nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "Sunshine adapter is disabled"})
			return
		}

		var cfg sunshine.LaunchConfig
		decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid launch request: " + err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		launched, err := s.Sunshine.Launch(ctx, cfg)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		log.Printf("sunshine: launched app=%d session=%s", launched.AppID, launched.SessionURL)
		_ = json.NewEncoder(w).Encode(map[string]any{"session": launched})
	})

	mux.HandleFunc("/api/sunshine/cancel", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !requireLocalAdmin(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST required"})
			return
		}
		if s.Sunshine == nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "Sunshine adapter is disabled"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()
		if err := s.Sunshine.Cancel(ctx); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		log.Printf("sunshine: session cancelled")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	upgrader := websocket.Upgrader{
		HandshakeTimeout: 5 * time.Second,
		CheckOrigin:      sameHostOrLocalOrigin,
	}

	mux.HandleFunc("/ws/session", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("signaling: websocket upgrade: %v", err)
			return
		}

		remote := r.RemoteAddr
		log.Printf("signaling: browser connected from %s", remote)

		current, err := session.New(conn, inputsink.LoggingSink{}, s.StunURL)
		if err != nil {
			log.Printf("signaling: create session: %v", err)
			_ = conn.Close()
			return
		}
		defer current.Close()

		if s.TestPattern {
			pattern, err := storagemedia.NewDiagnosticPattern()
			if err != nil {
				log.Printf("media: create diagnostic source: %v", err)
			} else {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				defer pattern.Close()

				current.Media().OnKeyframeRequest = pattern.RequestKeyframe
				go func() {
					if err := pattern.Start(ctx, current.Media()); err != nil &&
						!errors.Is(err, context.Canceled) {
						log.Printf("media: diagnostic source ended: %v", err)
					}
				}()
				log.Printf("media: diagnostic H264 pattern enabled")
			}
		}

		if err := current.Start(); err != nil {
			log.Printf("signaling: session ended with error: %v", err)
		}
	})

	if s.WebDir != "" {
		webDir := filepath.Clean(s.WebDir)
		if info, err := os.Stat(webDir); err == nil && info.IsDir() {
			log.Printf("web: serving %s", webDir)
			mux.Handle("/", http.FileServer(http.Dir(webDir)))
		} else {
			log.Printf("web: directory %q not found; API/signaling only", webDir)
		}
	}

	return withHeaders(mux)
}

func (s Server) ListenAndServe() error {
	log.Printf("StorPlay Host listening on http://%s", s.Addr)
	if s.StunURL == "" {
		log.Printf("WebRTC ICE: host candidates only (LAN mode)")
	} else {
		log.Printf("WebRTC ICE: STUN %s", s.StunURL)
	}
	if s.TestPattern {
		log.Printf("media: diagnostic test pattern mode is ON")
	}
	return http.ListenAndServe(s.Addr, s.Handler())
}

func requireLocalAdmin(w http.ResponseWriter, r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || !ip.IsLoopback() {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "this development admin action is restricted to localhost",
		})
		return false
	}
	return true
}

func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func sameHostOrLocalOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	originHost := stripPort(u.Host)
	requestHost := stripPort(r.Host)

	if strings.EqualFold(originHost, requestHost) {
		return true
	}

	return isLoopback(originHost) && isLoopback(requestHost)
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(hostport, "[]")
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func DescribeURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}
