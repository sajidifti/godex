package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sajidifti/godex/internal/llm"
)

type streamDeltaMsg struct{ text string }
type streamDoneMsg struct{ err error }

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

// streamInto forwards model deltas onto ch until the response completes or fails.
func streamInto(c *llm.Client, model string, history []llm.Message, ch chan<- tea.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	err := c.ChatStream(ctx, model, history, func(delta string) {
		ch <- streamDeltaMsg{text: delta}
	})
	ch <- streamDoneMsg{err: err}
}

// waitForStream receives the next event from the stream channel.
func waitForStream(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}
