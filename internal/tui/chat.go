package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sajidifti/godex/internal/llm"
	"github.com/sajidifti/godex/internal/tools"
)

const (
	maxSteps     = 8
	agentTimeout = 5 * time.Minute
)

const systemPrompt = `You are godex, a coding assistant running in the user's terminal.
You can inspect their project with tools. Prefer calling a tool over guessing.
Use list_dir and read_file to explore before answering. Keep replies concise.`

type streamDeltaMsg struct{ text string }
type toolCallMsg struct {
	name string
	args string
}
type toolResultMsg struct {
	name   string
	output string
}
type agentDoneMsg struct {
	history []llm.Message
	err     error
}

// toolDefs converts our Tool interface into the wire format the model expects.
func toolDefs(reg *tools.Registry) []llm.ToolDef {
	all := reg.All()
	defs := make([]llm.ToolDef, 0, len(all))
	for _, t := range all {
		defs = append(defs, llm.ToolDef{
			Type: "function",
			Function: llm.ToolFunc{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Schema(),
			},
		})
	}
	return defs
}

// runAgent drives the model <-> tool loop until the model stops asking for tools.
func runAgent(c *llm.Client, model string, reg *tools.Registry, history []llm.Message, ch chan<- tea.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()

	defs := toolDefs(reg)

	for step := 0; step < maxSteps; step++ {
		resp, err := c.ChatStream(ctx, model, history, defs, func(delta string) {
			ch <- streamDeltaMsg{text: delta}
		})
		if err != nil {
			ch <- agentDoneMsg{history: history, err: err}
			return
		}

		history = append(history, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			ch <- agentDoneMsg{history: history}
			return
		}

		for _, tc := range resp.ToolCalls {
			ch <- toolCallMsg{name: tc.Function.Name, args: tc.Function.Arguments}

			result := runTool(ctx, reg, tc)

			ch <- toolResultMsg{name: tc.Function.Name, output: result}
			history = append(history, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	ch <- agentDoneMsg{history: history, err: errors.New("stopped after too many tool steps")}
}

// runTool executes one tool call, turning any failure into text the model can read.
func runTool(ctx context.Context, reg *tools.Registry, tc llm.ToolCall) string {
	t, ok := reg.Get(tc.Function.Name)
	if !ok {
		return fmt.Sprintf("error: unknown tool %q", tc.Function.Name)
	}
	out, err := t.Run(ctx, json.RawMessage(tc.Function.Arguments))
	if err != nil {
		return "error: " + err.Error()
	}
	return out
}

// waitForStream receives the next event from the agent channel.
func waitForStream(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
