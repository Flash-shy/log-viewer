package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"log-viewer/backend/internal/logstore"
)

// RouterConfig configures optional API key, CORS, and rate limiting.
type RouterConfig struct {
	APIKey         string
	AllowedOrigins []string
	RateLimitRPS   float64
	RateBurst      int
}

// RouterConfigFromEnv builds config from environment (see LOG_VIEWER_* vars).
func RouterConfigFromEnv() RouterConfig {
	cfg := RouterConfig{
		AllowedOrigins: defaultCORSOrigins(),
		RateLimitRPS:   100,
		RateBurst:      50,
	}
	cfg.APIKey = strings.TrimSpace(os.Getenv("LOG_VIEWER_API_KEY"))
	if v := strings.TrimSpace(os.Getenv("LOG_VIEWER_RATE_LIMIT_RPS")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.RateLimitRPS = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("LOG_VIEWER_RATE_BURST")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.RateBurst = n
		}
	}
	if o := strings.TrimSpace(os.Getenv("LOG_VIEWER_CORS_ORIGINS")); o != "" {
		if o == "*" {
			cfg.AllowedOrigins = []string{"*"}
		} else {
			var list []string
			for _, p := range strings.Split(o, ",") {
				if s := strings.TrimSpace(p); s != "" {
					list = append(list, s)
				}
			}
			if len(list) > 0 {
				cfg.AllowedOrigins = list
			}
		}
	}
	return cfg
}

func defaultCORSOrigins() []string {
	return []string{"http://127.0.0.1:5173", "http://localhost:5173"}
}

// NewRouter returns the HTTP API with middleware and the given log store.
func NewRouter(store *logstore.Store, cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		list, err := store.List()
		if err != nil {
			writeInternalError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"files": list}); err != nil {
			log.Printf("encode response: %v", err)
		}
	})
	mux.HandleFunc("/api/logs/content", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "missing name", http.StatusBadRequest)
			return
		}
		tail, _ := strconv.Atoi(r.URL.Query().Get("tail"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		content, err := store.Read(name, offset, limit, tail)
		if err == logstore.ErrNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err == logstore.ErrInvalidName {
			http.Error(w, "invalid name", http.StatusBadRequest)
			return
		}
		if err == logstore.ErrFileTooLarge {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		if err == logstore.ErrTailTooLarge {
			http.Error(w, "tail too large", http.StatusBadRequest)
			return
		}
		if err != nil {
			writeInternalError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(content); err != nil {
			log.Printf("encode response: %v", err)
		}
	})
	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(openAPISpec)
	})
	mux.HandleFunc("/api/docs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(docsHTML)
	})

	var handler http.Handler = mux
	handler = withAPIKey(cfg.APIKey, handler)
	if cfg.RateLimitRPS > 0 {
		burst := cfg.RateBurst
		if burst <= 0 {
			burst = 50
		}
		handler = withRateLimit(cfg.RateLimitRPS, burst, handler)
	}
	handler = withCORS(cfg.AllowedOrigins, handler)
	return handler
}

func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("request error: %v", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func withCORS(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := corsOrigin(allowed, r.Header.Get("Origin")); o != "" {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func corsOrigin(allowed []string, origin string) string {
	if len(allowed) == 1 && allowed[0] == "*" {
		return "*"
	}
	if origin == "" {
		return ""
	}
	for _, a := range allowed {
		if origin == a {
			return origin
		}
	}
	return ""
}

func withAPIKey(key string, next http.Handler) http.Handler {
	if key == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || !apiKeyRequired(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if !validAPIKey(r, key) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func apiKeyRequired(p string) bool {
	switch p {
	case "/api/health", "/openapi.yaml", "/api/docs":
		return false
	default:
		return true
	}
}

func validAPIKey(r *http.Request, want string) bool {
	if got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); got != "" && got == want {
		return true
	}
	if r.Header.Get("X-API-Key") == want {
		return true
	}
	return false
}

// tokenBucket is a simple fixed-interval refill limiter (stdlib only).
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	last     time.Time
	rate     float64 // tokens per second
	capacity float64
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.rate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type ipBuckets struct {
	mu    sync.Mutex
	rps   float64
	burst int
	m     map[string]*tokenBucket
}

func (ib *ipBuckets) forAddr(hostPort string) *tokenBucket {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
	}
	ib.mu.Lock()
	defer ib.mu.Unlock()
	if ib.m == nil {
		ib.m = make(map[string]*tokenBucket)
	}
	b, ok := ib.m[host]
	if !ok {
		cap := float64(ib.burst)
		if cap < 1 {
			cap = 1
		}
		b = &tokenBucket{
			tokens:   cap,
			last:     time.Now(),
			rate:     ib.rps,
			capacity: cap,
		}
		ib.m[host] = b
	}
	return b
}

func withRateLimit(rps float64, burst int, next http.Handler) http.Handler {
	if burst < 1 {
		burst = 1
	}
	ib := &ipBuckets{rps: rps, burst: burst}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ib.forAddr(r.RemoteAddr).allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
