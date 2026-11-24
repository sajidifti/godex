package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type RunShell struct{ base string }

func NewRunShell(base string) *RunShell { return &RunShell{base: base} }

func (*RunShell) Name() string { return "run_shell" }

func (*RunShell) Description() string {
	return "Run a shell command in the working directory and return its combined output."
}

func (*RunShell) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Shell command to execute."}
		},
		"required": ["command"]
	}`)
}

func (*RunShell) NeedsApproval() bool { return true }

func (t *RunShell) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if a.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	name, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		name, flag = "cmd", "/C"
	}

	cmd := exec.CommandContext(ctx, name, flag, a.Command)
	cmd.Dir = t.base
	out, err := cmd.CombinedOutput()

	text := strings.TrimRight(string(out), "\r\n")
	if err != nil {
		text += "\n[exit error: " + err.Error() + "]"
	}
	if strings.TrimSpace(text) == "" {
		return "(no output)", nil
	}
	return text, nil
}
