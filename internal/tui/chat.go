package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sajidifti/godex/internal/llm"
)

type replyMsg struct {
	text string
	err  error
}

func toLLM(msgs []message) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.role == roleSystem {
			continue
		}
		out = append(out, llm.Message{Role: string(m.role), Content: m.content})
	}
	return out
}

func chatCmd(c *llm.Client, model string, history []llm.Message) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		text, err := c.Chat(ctx, model, history)
		return replyMsg{text: text, err: err}
	}
}
