package logstore

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestListSkipsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires Unix")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.log"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "ok.log")
	if err := os.Symlink(target, filepath.Join(dir, "link.log")); err != nil {
		t.Fatal(err)
	}
	st, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "ok.log" {
		t.Fatalf("got %+v", list)
	}
}

func TestReadRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires Unix")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.log"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "ok.log"), filepath.Join(dir, "link.log")); err != nil {
		t.Fatal(err)
	}
	st, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.Read("link.log", 0, 10, 0)
	if err != ErrInvalidName {
		t.Fatalf("got %v want ErrInvalidName", err)
	}
}

func TestReadOffsetDoesNotLoadUnboundedSlice(t *testing.T) {
	dir := t.TempDir()
	var b []byte
	for i := range 100 {
		b = append(b, byte('a'+i%26))
		b = append(b, '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "big.log"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.Read("big.log", 0, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if c.TotalLines != 100 || len(c.Lines) != 2 {
		t.Fatalf("total=%d lines=%d", c.TotalLines, len(c.Lines))
	}
}
