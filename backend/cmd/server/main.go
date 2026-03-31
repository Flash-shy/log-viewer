package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"log-viewer/backend/internal/logstore"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
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

	cfg := RouterConfigFromEnv()
	srv := &http.Server{
		Addr:              *addr,
		Handler:           NewRouter(store, cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("logs directory: %s", abs)
	log.Printf("listening on http://%s", *addr)
	log.Printf("OpenAPI UI: http://%s/api/docs", *addr)
	log.Fatal(srv.ListenAndServe())
}
