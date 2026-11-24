package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Compile-time proof that our types satisfy the Tool interface. If a method
// signature ever drifts, the build fails right here.
var (
	_ Tool    = (*ListDir)(nil)
	_ Tool    = (*ReadFile)(nil)
	_ Tool    = (*WriteFile)(nil)
	_ Guarded = (*WriteFile)(nil)
)

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewReadFile(dir).Run(context.Background(), json.RawMessage(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestListDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := NewListDir(dir).Run(context.Background(), json.RawMessage(`{"path":"."}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "sub/" {
		t.Errorf("got %q, want %q", got, "sub/")
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriteFile(dir).Run(context.Background(), json.RawMessage(`{"path":"sub/a.txt","content":"hi"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out == "" {
		t.Error("expected a status message")
	}

	data, err := os.ReadFile(filepath.Join(dir, "sub", "a.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "hi" {
		t.Errorf("got %q, want %q", data, "hi")
	}
}
