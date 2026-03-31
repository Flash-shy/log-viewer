package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"log-viewer/backend/internal/logstore"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	logsDir := flag.String("logs", "", "absolute or relative path to logs directory (default: LOG_VIEWER_LOG_DIR or ../logs from cwd)")
	flag.Parse()

	dir := *logsDir
	if dir == "" {
		dir = os.Getenv("LOG_VIEWER_LOG_DIR")
	}
	if dir == "" {
		dir = "../logs"
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		log.Fatal(err)
	}
	store, err := logstore.New(abs)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("logs directory: %s", abs)
	log.Printf("backend listening on http://127.0.0.1%s", *addr)
	log.Printf("OpenAPI UI: http://127.0.0.1%s/api/docs", *addr)
	log.Fatal(http.ListenAndServe(*addr, NewRouter(store)))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
