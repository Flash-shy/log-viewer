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

const maxFileBytes = 10 << 20 // 10 MiB

var ErrNotFound = errors.New("file not found")
var ErrInvalidName = errors.New("invalid file name")

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

// List returns regular files in Root, sorted by name.
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
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := e.Name()
		if !safeName(name) {
			continue
		}
		out = append(out, FileMeta{Name: name, Size: info.Size()})
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
	fi, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if fi.IsDir() {
		return nil, ErrNotFound
	}
	if fi.Size() > maxFileBytes {
		return nil, errors.New("file too large")
	}
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lines, err := readAllLines(f)
	if err != nil {
		return nil, err
	}
	total := len(lines)
	if total == 0 {
		return &Content{File: name, TotalLines: 0, Lines: nil}, nil
	}

	const maxReturn = 5000
	truncated := false

	if tail > 0 {
		if tail > total {
			tail = total
		}
		start := total - tail
		chunk := lines[start:]
		startIdx := start
		if len(chunk) > maxReturn {
			chunk = chunk[len(chunk)-maxReturn:]
			startIdx = total - len(chunk)
			truncated = true
		}
		return buildContent(name, total, chunk, startIdx, truncated), nil
	}

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > maxReturn {
		limit = maxReturn
	}
	if offset >= total {
		return &Content{File: name, TotalLines: total, Lines: nil}, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	chunk := lines[offset:end]
	truncated = end < total
	return buildContent(name, total, chunk, offset, truncated), nil
}

func buildContent(name string, total int, texts []string, startIdx int, truncated bool) *Content {
	lines := make([]Line, len(texts))
	for i, t := range texts {
		lines[i] = Line{No: startIdx + i + 1, Text: t}
	}
	return &Content{File: name, TotalLines: total, Lines: lines, Truncated: truncated}
}

func readAllLines(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	const maxLine = 1024 * 1024
	buf := make([]byte, maxLine)
	sc.Buffer(buf, maxLine)
	var out []string
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out, sc.Err()
}

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
