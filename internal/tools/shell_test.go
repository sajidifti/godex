package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

var (
	_ Tool    = (*RunShell)(nil)
	_ Guarded = (*RunShell)(nil)
)

func TestRunShell(t *testing.T) {
	out, err := NewRunShell(t.TempDir()).Run(context.Background(), json.RawMessage(`{"command":"echo godex"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "godex") {
		t.Errorf("output = %q, want it to contain %q", out, "godex")
	}
}
