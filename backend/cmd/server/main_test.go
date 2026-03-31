package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log-viewer/backend/internal/apitest"
	"log-viewer/backend/internal/logstore"
)

func subName(id apitest.EndpointID, caseName string) string {
	return string(id) + "|" + caseName
}

func TestAPI(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "sample.log"), "line one\nline two\nline three\n")
	// unsafe / hidden files are skipped by List; still create for invalid-name read tests
	mustWriteFile(t, filepath.Join(dir, "plain.log"), "x\n")

	store, err := logstore.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := NewRouter(store, RouterConfig{
		AllowedOrigins: []string{"http://127.0.0.1:5173"},
		RateLimitRPS:   0,
	})

	t.Run(subName(apitest.Health, "GET_returns_json_ok"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("Content-Type: %q", ct)
		}
		var v struct {
			OK bool `json:"ok"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&v); err != nil || !v.OK {
			t.Fatalf("body: %v ok=%v", err, v.OK)
		}
	})

	t.Run(subName(apitest.Health, "POST_method_not_allowed"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/health", strings.NewReader("{}"))
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run(subName(apitest.ListLogs, "GET_lists_sorted_files"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		var out struct {
			Files []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
			} `json:"files"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		if len(out.Files) != 2 {
			t.Fatalf("files: %+v", out.Files)
		}
		if out.Files[0].Name != "plain.log" || out.Files[1].Name != "sample.log" {
			t.Fatalf("order: %+v", out.Files)
		}
	})

	t.Run(subName(apitest.ListLogs, "GET_empty_directory"), func(t *testing.T) {
		empty := t.TempDir()
		st, err := logstore.New(empty)
		if err != nil {
			t.Fatal(err)
		}
		hh := NewRouter(st, RouterConfig{
			AllowedOrigins: []string{"http://127.0.0.1:5173"},
			RateLimitRPS:   0,
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
		hh.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		var out struct {
			Files []any `json:"files"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil || len(out.Files) != 0 {
			t.Fatalf("files: %+v err=%v", out.Files, err)
		}
	})

	t.Run(subName(apitest.ListLogs, "DELETE_method_not_allowed"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/logs", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run(subName(apitest.LogContent, "GET_missing_name_bad_request"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs/content", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run(subName(apitest.LogContent, "GET_invalid_name_bad_request"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs/content?name=bad%20name", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run(subName(apitest.LogContent, "GET_not_found"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs/content?name=missing.log", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run(subName(apitest.LogContent, "GET_offset_limit_window"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs/content?name=sample.log&offset=0&limit=2", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		var c struct {
			File       string `json:"file"`
			TotalLines int    `json:"totalLines"`
			Truncated  bool   `json:"truncated"`
			Lines      []struct {
				No   int    `json:"no"`
				Text string `json:"text"`
			} `json:"lines"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
			t.Fatal(err)
		}
		if c.TotalLines != 3 || len(c.Lines) != 2 || c.Lines[0].Text != "line one" || !c.Truncated {
			t.Fatalf("%+v", c)
		}
	})

	t.Run(subName(apitest.LogContent, "GET_tail_last_lines"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs/content?name=sample.log&tail=2", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		var c struct {
			TotalLines int `json:"totalLines"`
			Lines      []struct {
				Text string `json:"text"`
			} `json:"lines"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
			t.Fatal(err)
		}
		if len(c.Lines) != 2 || c.Lines[0].Text != "line two" {
			t.Fatalf("%+v", c)
		}
	})

	t.Run(subName(apitest.LogContent, "PUT_method_not_allowed"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/logs/content?name=sample.log", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run(subName(apitest.OpenAPISpec, "GET_returns_yaml"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "yaml") {
			t.Fatalf("Content-Type: %q", ct)
		}
		body, _ := io.ReadAll(rec.Body)
		if !bytes.Contains(body, []byte("openapi:")) {
			t.Fatalf("unexpected body prefix")
		}
	})

	t.Run(subName(apitest.OpenAPISpec, "PATCH_method_not_allowed"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/openapi.yaml", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run(subName(apitest.DocsHTML, "GET_returns_html"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("Content-Type: %q", ct)
		}
		body, _ := io.ReadAll(rec.Body)
		if len(body) < 20 {
			t.Fatalf("short body")
		}
	})

	t.Run(subName(apitest.DocsHTML, "POST_method_not_allowed"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/docs", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("APIKey|GET_logs_without_key_unauthorized", func(t *testing.T) {
		hk := NewRouter(store, RouterConfig{
			APIKey:         "secret-test-key",
			AllowedOrigins: []string{"http://127.0.0.1:5173"},
			RateLimitRPS:   0,
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
		hk.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("APIKey|GET_logs_with_bearer_ok", func(t *testing.T) {
		hk := NewRouter(store, RouterConfig{
			APIKey:         "secret-test-key",
			AllowedOrigins: []string{"http://127.0.0.1:5173"},
			RateLimitRPS:   0,
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
		req.Header.Set("Authorization", "Bearer secret-test-key")
		hk.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run(subName(apitest.CORSPreflight, "OPTIONS_returns_204_and_headers"), func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/api/logs", nil)
		req.Header.Set("Origin", "http://127.0.0.1:5173")
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status %d", rec.Code)
		}
		if got, want := rec.Header().Get("Access-Control-Allow-Origin"), "http://127.0.0.1:5173"; got != want {
			t.Fatalf("CORS header: got %q want %q", got, want)
		}
	})
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
