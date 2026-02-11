package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harris/gemini-web-cli/internal/api"
)

// AppMode represents the current view mode.
type AppMode int

const (
	ModeChat AppMode = iota
	ModeHistory
)

// AppModel is the top-level bubbletea model.
type AppModel struct {
	mode    AppMode
	chat    ChatModel
	history HistoryModel
	client  *api.Client
	width   int
	height  int
}

// NewApp creates the top-level TUI application.
func NewApp(client *api.Client, gemName string) AppModel {
	return AppModel{
		mode:   ModeChat,
		chat:   NewChatModel(client, gemName),
		client: client,
	}
}

// Init implements tea.Model.
func (m AppModel) Init() tea.Cmd {
	return m.chat.Init()
}

// Update implements tea.Model.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Track window size at top level
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsm.Width
		m.height = wsm.Height
	}

	switch m.mode {
	case ModeChat:
		// Intercept switchToHistoryMsg before passing to chat
		if _, ok := msg.(switchToHistoryMsg); ok {
			m.history = NewHistoryModel(m.client, m.width, m.height)
			m.mode = ModeHistory
			return m, m.history.Init()
		}
		updated, cmd := m.chat.Update(msg)
		m.chat = updated.(ChatModel)
		return m, cmd
	case ModeHistory:
		updated, cmd := m.history.Update(msg)
		m.history = updated.(HistoryModel)
		if m.history.Back() || m.history.Selected() != nil {
			m.mode = ModeChat
			return m, m.chat.Init()
		}
		return m, cmd
	}
	return m, nil
}

// View implements tea.Model.
func (m AppModel) View() string {
	switch m.mode {
	case ModeChat:
		return m.chat.View()
	case ModeHistory:
		return m.history.View()
	}
	return ""
}
