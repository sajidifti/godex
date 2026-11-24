package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxReadBytes = 256 * 1024

type ListDir struct {
	base string
}

func NewListDir(base string) *ListDir {
	return &ListDir{base: base}
}

func (*ListDir) Name() string {
	return "list_dir"
}

func (*ListDir) Description() string {
	return "List files and subdirectories inside a directory."
}

func (*ListDir) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Directory to list, relative to the working directory. Defaults to \".\"."
			}
		}
	}`)
}

func (t *ListDir) Run(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if a.Path == "" {
		a.Path = "."
	}

	entries, err := os.ReadDir(filepath.Join(t.base, a.Path))
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		b.WriteString(name)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

type ReadFile struct{ base string }

func NewReadFile(base string) *ReadFile { return &ReadFile{base: base} }

func (*ReadFile) Name() string { return "read_file" }

func (*ReadFile) Description() string {
	return "Read the contents of a text file."
}

func (*ReadFile) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path relative to the working directory."}
		},
		"required": ["path"]
	}`)
}

func (t *ReadFile) Run(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if a.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	data, err := os.ReadFile(filepath.Join(t.base, a.Path))
	if err != nil {
		return "", err
	}
	if len(data) > maxReadBytes {
		return string(data[:maxReadBytes]) + "\n\n[truncated]", nil
	}
	return string(data), nil
}

type WriteFile struct{ base string }

func NewWriteFile(base string) *WriteFile { return &WriteFile{base: base} }

func (*WriteFile) Name() string { return "write_file" }

func (*WriteFile) Description() string {
	return "Create or overwrite a file with the given content."
}

func (*WriteFile) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path relative to the working directory."},
			"content": {"type": "string", "description": "Full file content to write."}
		},
		"required": ["path", "content"]
	}`)
}

func (*WriteFile) NeedsApproval() bool { return true }

func (t *WriteFile) Run(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if a.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	full := filepath.Join(t.base, a.Path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(a.Content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), a.Path), nil
}

// Default builds the registry with every built-in tool.
func Default(base string) *Registry {
	return NewRegistry(
		NewListDir(base),
		NewReadFile(base),
		NewWriteFile(base),
		NewRunShell(base),
	)
}
