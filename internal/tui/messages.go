package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type role string

const (
	roleUser      role = "user"
	roleAssistant role = "assistant"
	roleSystem    role = "system"
	roleTool      role = "tool"
)

type message struct {
	role    role
	content string
}

var (
	userLabel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#83b7ff"))
	botLabel  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8be0b1"))
	dim       = lipgloss.NewStyle().Foreground(lipgloss.Color("#9aa9c4"))
	warn      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffcc66"))
)

func renderMessage(msgs []message, width int) string {
	var b strings.Builder
	body := lipgloss.NewStyle().Width(width)

	for i, msg := range msgs {
		if i > 0 {
			b.WriteString("\n\n")
		}

		if msg.role == roleTool {
			b.WriteString(dim.Width(width).Render(msg.content))
			continue
		}

		var label string
		style := body

		switch msg.role {
		case roleUser:
			label = userLabel.Render("You")
		case roleSystem:
			label = dim.Bold(true).Render("system")
			style = dim.Width(width)
		default:
			label = botLabel.Render("godex")
		}

		b.WriteString(label)
		b.WriteString("\n")
		b.WriteString(style.Render(msg.content))
	}

	return b.String()
}
