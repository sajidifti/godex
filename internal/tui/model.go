package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sajidifti/godex/internal/llm"
)

const inputHeight = 3

type Model struct {
	width, height int
	ready         bool
	waiting       bool
	model         string
	client        *llm.Client
	input         textarea.Model
	viewport      viewport.Model
	messages      []message
	streamCh      chan tea.Msg
}

func New(client *llm.Client, modelName string) Model {
	ta := textarea.New()
	ta.Placeholder = "What should we cook?"
	ta.ShowLineNumbers = false
	ta.SetVirtualCursor(true)
	ta.SetHeight(inputHeight)
	ta.Focus()

	return Model{
		client:   client,
		model:    modelName,
		input:    ta,
		viewport: viewport.New(),
		messages: []message{
			{roleSystem, "Connected via OpenRouter. Type a message or /help."}},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			cmd := m.submit()
			return m, cmd
		case "pgup", "pgdown":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case streamDeltaMsg:
		if n := len(m.messages); n > 0 {
			m.messages[n-1].content += msg.text
		}
		m.refresh()
		return m, waitForStream(m.streamCh)
	case streamDoneMsg:
		m.waiting = false
		m.streamCh = nil
		if msg.err != nil {
			if n := len(m.messages); n > 0 && m.messages[n-1].role == roleAssistant && m.messages[n-1].content == "" {
				m.messages = m.messages[:n-1]
			}
			m.addSystem("Error: " + msg.err.Error())
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) layout() {
	m.input.SetWidth(m.width)
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(max(1, m.height-inputHeight-2))
	m.refresh()
}

func (m *Model) refresh() {
	m.viewport.SetContent(renderMessage(m.messages, m.width))
	m.viewport.GotoBottom()
}

func (m *Model) submit() tea.Cmd {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return nil
	}
	isCmd := strings.HasPrefix(text, "/")
	if m.waiting && !isCmd {
		return nil
	}
	m.input.Reset()
	if isCmd {
		return m.runCommand(text)
	}

	m.messages = append(m.messages, message{roleUser, text})
	history := toLLM(m.messages)
	m.messages = append(m.messages, message{roleAssistant, ""})
	m.waiting = true
	m.streamCh = make(chan tea.Msg, 64)
	m.refresh()

	go streamInto(m.client, m.model, history, m.streamCh)
	return waitForStream(m.streamCh)
}

func (m Model) View() tea.View {
	content := "starting...."
	if m.ready {
		sep := dim.Render(strings.Repeat("-", m.width))
		status := dim.Render(m.model + "Enter send . PgUp/PgDown scroll . Ctrl+C quit")
		if m.waiting {
			status = dim.Render(m.model + " . thinking...")
		}
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			m.viewport.View(),
			sep,
			m.input.View(),
			status,
		)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
