package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harris/gemini-web-cli/internal/api"
)

// convItem implements list.Item for conversations.
type convItem struct {
	conv api.Conversation
}

func (i convItem) Title() string       { return i.conv.Title }
func (i convItem) Description() string { return i.conv.ID }
func (i convItem) FilterValue() string { return i.conv.Title }

// conversationsLoadedMsg carries loaded conversations.
type conversationsLoadedMsg struct {
	convs []api.Conversation
	err   error
}

// HistoryModel is the bubbletea model for history browsing.
type HistoryModel struct {
	client   *api.Client
	list     list.Model
	selected *api.Conversation
	back     bool
	loading  bool
	err      error
}

// NewHistoryModel creates a new history view.
func NewHistoryModel(client *api.Client, width, height int) HistoryModel {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Conversation History"
	l.SetShowHelp(true)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to chat")),
		}
	}
	// Disable built-in quit keys (q and esc) so they don't exit the app
	l.KeyMap.Quit = key.NewBinding(key.WithDisabled())
	return HistoryModel{
		client:  client,
		list:    l,
		loading: true,
	}
}

// Init implements tea.Model.
func (m HistoryModel) Init() tea.Cmd {
	return m.loadConversations()
}

// Update implements tea.Model.
func (m HistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			m.back = true
			return m, nil
		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(convItem); ok {
				m.selected = &item.conv
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)

	case conversationsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		items := make([]list.Item, len(msg.convs))
		for i, c := range msg.convs {
			items[i] = convItem{conv: c}
		}
		m.list.SetItems(items)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m HistoryModel) View() string {
	if m.loading {
		return "Loading conversations..."
	}
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	return m.list.View()
}

// Selected returns the selected conversation, if any.
func (m HistoryModel) Selected() *api.Conversation {
	return m.selected
}

// Back returns true if the user wants to go back to chat.
func (m HistoryModel) Back() bool {
	return m.back
}

func (m HistoryModel) loadConversations() tea.Cmd {
	return func() tea.Msg {
		convs, err := m.client.ListConversations()
		return conversationsLoadedMsg{convs: convs, err: err}
	}
}
