package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type command struct {
	name string
	help string
	run  func(m *Model, args string) tea.Cmd
}

func commands() []command {
	return []command{
		{"help", "show avilable commands", cmdHelp},
		{"clear", "clear the conversation", cmdClear},
		{"exit", "quit godex", cmdExit},
	}
}

func (m *Model) runCommand(line string) tea.Cmd {
	name, args, _ := strings.Cut(strings.TrimPrefix(line, "/"), " ")
	name = strings.ToLower(name)

	for _, c := range commands() {
		if c.name == name {
			return c.run(m, strings.TrimSpace(args))
		}
	}
	m.addSystem(fmt.Sprintf("Unknown command /%s. Try /help.", name))
	return nil
}

func (m *Model) addSystem(text string) {
	m.messages = append(m.messages, message{roleSystem, text})
	m.refresh()
}

func cmdHelp(m *Model, _ string) tea.Cmd {
	var b strings.Builder
	b.WriteString("Commands:\n")
	for _, c := range commands() {
		fmt.Fprintf(&b, "  /%-6s %s\n", c.name, c.help)
	}
	m.addSystem(strings.TrimRight(b.String(), "\n"))
	return nil
}

func cmdClear(m *Model, _ string) tea.Cmd {
	m.messages = nil
	m.refresh()
	return nil
}

func cmdExit(_ *Model, _ string) tea.Cmd {
	return tea.Quit
}
