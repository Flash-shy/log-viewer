// Package logstore provides read-only access to plain-text log files under a single
// directory: listing files, validating names to avoid path traversal, and reading
// line ranges or tail segments with size limits.
package logstore

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// maxFileBytes caps how large a single log file may be before Read rejects it.
const maxFileBytes = 10 << 20 // 10 MiB

// maxTailLines caps the tail parameter to bound memory for the sliding window.
const maxTailLines = 500_000

// ErrNotFound is returned when the requested log file does not exist under Root.
var ErrNotFound = errors.New("file not found")

// ErrInvalidName is returned when the file name is empty, unsafe, or escapes Root.
var ErrInvalidName = errors.New("invalid file name")

// ErrFileTooLarge is returned when the file exceeds maxFileBytes.
var ErrFileTooLarge = errors.New("file too large")

// ErrTailTooLarge is returned when tail exceeds maxTailLines.
var ErrTailTooLarge = errors.New("tail too large")

// FileMeta is a log file under the log directory.
type FileMeta struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Line is one line of log text with 1-based line number.
type Line struct {
	No   int    `json:"no"`
	Text string `json:"text"`
}

// Content is a slice of lines plus total line count in the file.
type Content struct {
	File       string `json:"file"`
	TotalLines int    `json:"totalLines"`
	Lines      []Line `json:"lines"`
	Truncated  bool   `json:"truncated"`
}

// Store reads log files from a single root directory.
type Store struct {
	Root string
}

// New returns a Store with an absolute root path.
func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Store{Root: abs}, nil
}

// List returns regular files in Root, sorted by name. Directories, symlinks, and
// names that fail safeName are omitted.
func (s *Store) List() ([]FileMeta, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, err
	}
	out := make([]FileMeta, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !safeName(name) {
			continue
		}
		path := filepath.Join(s.Root, name)
		fi, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, FileMeta{Name: name, Size: fi.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Read returns lines from a file. If tail > 0, returns the last tail lines (offset ignored).
// Otherwise returns up to limit lines starting at offset (0-based line index).
func (s *Store) Read(name string, offset, limit, tail int) (*Content, error) {
	if !safeName(name) {
		return nil, ErrInvalidName
	}
	absRoot, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(filepath.Join(s.Root, name))
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, ErrInvalidName
	}
	fi, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidName
	}
	if !fi.Mode().IsRegular() {
		return nil, ErrNotFound
	}
	if fi.Size() > maxFileBytes {
		return nil, ErrFileTooLarge
	}
	f, err := os.Open(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer f.Close()

	if tail > 0 {
		if tail > maxTailLines {
			return nil, ErrTailTooLarge
		}
		return readTail(f, name, tail)
	}
	return readOffsetLimit(f, name, offset, limit)
}

// buildContent maps raw line strings to Line values with 1-based line numbers starting at startIdx.
func buildContent(name string, total int, texts []string, startIdx int, truncated bool) *Content {
	lines := make([]Line, len(texts))
	for i, t := range texts {
		lines[i] = Line{No: startIdx + i + 1, Text: t}
	}
	return &Content{File: name, TotalLines: total, Lines: lines, Truncated: truncated}
}

func newScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	const maxLine = 1024 * 1024
	buf := make([]byte, maxLine)
	sc.Buffer(buf, maxLine)
	return sc
}

func readOffsetLimit(r io.Reader, name string, offset, limit int) (*Content, error) {
	const maxReturn = 5000
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > maxReturn {
		limit = maxReturn
	}

	sc := newScanner(r)
	lineIdx := 0
	var chunk []string
	startIdx := offset

	for sc.Scan() {
		text := sc.Text()
		if lineIdx >= offset && len(chunk) < limit {
			chunk = append(chunk, text)
		}
		lineIdx++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	total := lineIdx
	if total == 0 {
		return &Content{File: name, TotalLines: 0, Lines: nil}, nil
	}
	if offset >= total {
		return &Content{File: name, TotalLines: total, Lines: nil}, nil
	}

	end := offset + len(chunk)
	truncated := end < total
	return buildContent(name, total, chunk, startIdx, truncated), nil
}

func readTail(r io.Reader, name string, tail int) (*Content, error) {
	const maxReturn = 5000
	sc := newScanner(r)

	var window []string
	total := 0
	for sc.Scan() {
		total++
		window = append(window, sc.Text())
		if len(window) > tail {
			window = window[1:]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if total == 0 {
		return &Content{File: name, TotalLines: 0, Lines: nil}, nil
	}

	effectiveTail := tail
	if effectiveTail > total {
		effectiveTail = total
	}
	if len(window) > effectiveTail {
		window = window[len(window)-effectiveTail:]
	}

	startIdx := total - len(window)
	truncated := false
	chunk := window
	if len(chunk) > maxReturn {
		chunk = chunk[len(chunk)-maxReturn:]
		startIdx = total - len(chunk)
		truncated = true
	}
	return buildContent(name, total, chunk, startIdx, truncated), nil
}

// safeName allows only a single path segment of letters, digits, dot, underscore, and hyphen.
func safeName(name string) bool {
	if name == "" || name != filepath.Base(name) {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
