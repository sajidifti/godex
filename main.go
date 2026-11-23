package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/sajidifti/godex/internal/config"
	"github.com/sajidifti/godex/internal/llm"
	"github.com/sajidifti/godex/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	client := llm.NewClient(cfg.APIKey, llm.DefaultBaseURL)

	p := tea.NewProgram(tui.New(client, cfg.Model))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
