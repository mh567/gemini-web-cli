package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/harris/gemini-web-cli/internal/api"
)

// chatMsg holds a displayed message.
type chatMsg struct {
	role     string
	text     string
	thoughts string
	images   []api.ImageInfo
}

// streamChunkMsg wraps a chunk from the API stream.
type streamChunkMsg struct {
	chunk api.StreamChunk
	ch    <-chan api.StreamChunk // carry the channel for reading next chunk
}

// initDoneMsg signals that client initialization is complete.
type initDoneMsg struct {
	err error
}

// switchToHistoryMsg signals the app to switch to history view.
type switchToHistoryMsg struct{}

// historyMode represents the current history browsing state.
type historyMode int

const (
	historyNone    historyMode = iota // normal chat
	historyList                       // browsing paginated list
	historyLoading                    // loading a conversation's messages
	historyView                       // viewing conversation messages
)

const historyPageSize = 10

// historyLoadedMsg carries loaded conversation history.
type historyLoadedMsg struct {
	convs []api.Conversation
	err   error
}

// conversationDetailMsg carries a loaded conversation's messages and metadata.
type conversationDetailMsg struct {
	detail *api.ConversationDetail
	convID string
	title  string
	err    error
}

// modelOption holds a model for selection display.
type modelOption struct {
	key   string
	model api.Model
}

// ChatModel is the bubbletea model for the chat view.
type ChatModel struct {
	client            *api.Client
	session           *api.ChatSession
	gemName           string // gem name to resolve after init
	gemID             string
	initializing      bool
	ready             bool // true after first WindowSizeMsg
	showThoughts      bool // false by default: thoughts are collapsed
	input             textarea.Model
	spinner           spinner.Model
	renderer          *glamour.TermRenderer
	width             int
	waiting           bool
	interrupted       bool
	streaming         string
	streamingThoughts string
	streamingImages   []api.ImageInfo
	pendingPrints     []string // queued lines to print via tea.Println
	lastEscTime       time.Time
	pendingFiles      []api.FileRef
	selectingModel    bool
	modelOptions      []modelOption
	// History browsing state
	historyState     historyMode
	historyConvs     []api.Conversation
	historyPage      int
	historyCursor    int // selected item index within current page
	historyDetail    *api.ConversationDetail
	historyConvID    string
	historyConvTitle string
	historyScroll    int // scroll offset for conversation detail view
	height           int // terminal height
}

// NewChatModel creates a new chat view model.
func NewChatModel(client *api.Client, gemName string) ChatModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message... (/upload, /model, /new, /history, /thoughts)"
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(3)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create Glamour renderer: %v\n", err)
	}

	return ChatModel{
		client:       client,
		session:      &api.ChatSession{},
		gemName:      gemName,
		initializing: true,
		input:        ta,
		spinner:      sp,
		renderer:     r,
	}
}

// Init implements tea.Model.
func (m ChatModel) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		m.doInit(),
	)
}

// doInit runs client.Init() asynchronously.
func (m ChatModel) doInit() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.Init(); err != nil {
			return initDoneMsg{err: err}
		}
		m.client.StartCookieRefresh()
		return initDoneMsg{}
	}
}

// Update implements tea.Model.
func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// History browsing mode: intercept all keys
		if m.historyState != historyNone {
			return m.handleHistoryKey(msg)
		}
		// Model selection mode: handle number keys
		if m.selectingModel {
			return m.handleModelSelect(msg)
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			return m.handleEsc()
		case tea.KeyEnter:
			if !m.waiting && !m.initializing {
				return m.handleSubmit()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 2)
		if !m.ready {
			m.ready = true
			m.input.Reset()
		}
		if r, err := glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(msg.Width-4),
		); err == nil {
			m.renderer = r
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Failed to recreate renderer on resize: %v\n", err)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case initDoneMsg:
		return m.handleInitDone(msg)

	case streamChunkMsg:
		return m.handleStreamChunk(msg)

	case uploadDoneMsg:
		return m.handleUploadDone(msg)

	case historyLoadedMsg:
		return m.handleHistoryLoaded(msg)

	case conversationDetailMsg:
		return m.handleConversationDetail(msg)
	}

	// Don't pass keys to textarea during history browsing
	if m.historyState == historyNone {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m ChatModel) handleInitDone(msg initDoneMsg) (tea.Model, tea.Cmd) {
	m.initializing = false

	if msg.err != nil {
		m.addMsg(chatMsg{
			role: "system",
			text: fmt.Sprintf("Session init failed (cookies may be expired): %v\nRun: gemini-web-cli login", msg.err),
		})
		return m, m.flushPrints()
	}

	// Resolve gem name if provided
	if m.gemName != "" {
		gems, err := m.client.ListGems()
		if err == nil {
			for _, g := range gems {
				if strings.EqualFold(g.Name, m.gemName) {
					m.gemID = g.ID
					break
				}
			}
		}
		if m.gemID == "" {
			m.addMsg(chatMsg{
				role: "system",
				text: fmt.Sprintf("Gem %q not found, using default.", m.gemName),
			})
		}
	}

	m.addMsg(chatMsg{role: "system", text: "Ready."})
	return m, m.flushPrints()
}

// View implements tea.Model.
func (m ChatModel) View() string {
	var b strings.Builder

	// History modes render their own full-screen views
	if m.historyState == historyLoading {
		b.WriteString(m.spinner.View() + " Loading conversation...\n")
		return b.String()
	} else if m.historyState == historyList {
		return m.viewHistoryList()
	} else if m.historyState == historyView {
		return m.viewHistoryDetail()
	}

	if m.initializing {
		b.WriteString(m.spinner.View() + " Initializing session...\n")
	} else if m.waiting {
		if m.streamingThoughts != "" {
			b.WriteString(m.renderThoughts(m.streamingThoughts))
			b.WriteString("\n")
		}
		if m.streaming != "" {
			b.WriteString(m.streaming)
			b.WriteString("\n")
		}
		if m.streaming == "" && m.streamingThoughts == "" {
			b.WriteString(m.spinner.View() + " Thinking... (double ESC to cancel)")
		}
		b.WriteString("\n")
	}

	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter: send | Ctrl+C: quit | /new /model /upload /history /thoughts"))

	return b.String()
}

func (m ChatModel) handleEsc() (tea.Model, tea.Cmd) {
	// Cancel model selection on ESC
	if m.selectingModel {
		m.selectingModel = false
		m.modelOptions = nil
		m.addMsg(chatMsg{role: "system", text: "Model selection cancelled."})
		return m, m.flushPrints()
	}

	now := time.Now()
	doublePress := now.Sub(m.lastEscTime) < 500*time.Millisecond
	m.lastEscTime = now

	if m.waiting && doublePress {
		m.interrupted = true
		m.waiting = false
		if m.streaming != "" {
			m.addMsg(chatMsg{
				role:     "model",
				text:     m.streaming,
				thoughts: m.streamingThoughts,
				images:   m.streamingImages,
			})
			m.addMsg(chatMsg{role: "system", text: "[interrupted]"})
		}
		m.streaming = ""
		m.streamingThoughts = ""
		m.streamingImages = nil
		return m, m.flushPrints()
	}

	return m, nil
}

func (m ChatModel) handleModelSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		return m.handleEsc()
	}

	r := msg.String()
	if len(r) == 1 && r[0] >= '1' && r[0] <= '9' {
		idx := int(r[0] - '1')
		if idx < len(m.modelOptions) {
			opt := m.modelOptions[idx]
			m.client.SetModel(opt.model)
			m.selectingModel = false
			m.modelOptions = nil
			m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("Switched to %s", opt.model.DisplayName)})
			return m, m.flushPrints()
		}
	}

	return m, nil
}

func (m ChatModel) handleSubmit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}
	m.input.Reset()

	// Handle slash commands
	if strings.HasPrefix(text, "/") {
		return m.handleSlashCmd(text)
	}

	m.addMsg(chatMsg{role: "user", text: text})
	m.waiting = true
	m.interrupted = false
	m.streaming = ""
	m.streamingThoughts = ""
	m.streamingImages = nil

	files := m.pendingFiles
	m.pendingFiles = nil
	return m, tea.Batch(m.flushPrints(), m.sendMessageWithFiles(text, files))
}

func (m ChatModel) handleSlashCmd(text string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(text)
	cmd := parts[0]

	switch cmd {
	case "/new":
		m.session = &api.ChatSession{}
		m.addMsg(chatMsg{role: "system", text: "New conversation started."})
		return m, m.flushPrints()
	case "/model":
		if len(parts) > 1 {
			if model, ok := api.GetModel(parts[1]); ok {
				m.client.SetModel(model)
				m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("Switched to %s", model.DisplayName)})
				return m, m.flushPrints()
			}
			m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("Unknown model: %s", parts[1])})
			return m, m.flushPrints()
		}
		// Enter model selection mode
		var opts []modelOption
		for k, v := range api.Models {
			opts = append(opts, modelOption{key: k, model: v})
		}
		m.modelOptions = opts
		m.selectingModel = true
		var b strings.Builder
		b.WriteString("Select a model:")
		for i, opt := range opts {
			b.WriteString(fmt.Sprintf("\n  %d) %s (%s)", i+1, opt.model.DisplayName, opt.key))
		}
		b.WriteString("\nPress 1-9 to select, ESC to cancel")
		m.addMsg(chatMsg{role: "system", text: b.String()})
		return m, m.flushPrints()
	case "/upload":
		if len(parts) > 1 {
			filePath := parts[1]
			question := ""
			if len(parts) > 2 {
				question = strings.Join(parts[2:], " ")
			}
			if question != "" {
				m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("Uploading %s...", filePath)})
				m.waiting = true
				return m, tea.Batch(m.flushPrints(), m.uploadFile(filePath, question))
			}
			return m, m.uploadFile(filePath, question)
		}
		m.addMsg(chatMsg{role: "system", text: "Usage: /upload <file-path> [question]"})
		return m, m.flushPrints()
	case "/history":
		m.historyState = historyLoading
		m.historyPage = 0
		m.historyCursor = 0
		m.historyConvs = nil
		return m, m.loadHistory()
	case "/thoughts":
		if len(parts) == 1 {
			m.showThoughts = !m.showThoughts
		} else {
			switch strings.ToLower(parts[1]) {
			case "on", "show", "expand", "expanded":
				m.showThoughts = true
			case "off", "hide", "collapse", "collapsed":
				m.showThoughts = false
			default:
				m.addMsg(chatMsg{role: "system", text: "Usage: /thoughts [on|off]"})
				return m, m.flushPrints()
			}
		}

		state := "collapsed"
		if m.showThoughts {
			state = "expanded"
		}
		m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("Thoughts view %s.", state)})
		return m, m.flushPrints()
	default:
		m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("Unknown command: %s. Available: /new, /model, /upload, /history, /thoughts", cmd)})
		return m, m.flushPrints()
	}
}

func (m ChatModel) sendMessageWithFiles(text string, files []api.FileRef) tea.Cmd {
	return func() tea.Msg {
		ch, err := m.client.StreamGenerateWithGem(text, m.session, files, m.gemID)
		if err != nil {
			return streamChunkMsg{chunk: api.StreamChunk{Error: err}}
		}
		return readNextChunk(ch)
	}
}

func readNextChunk(ch <-chan api.StreamChunk) streamChunkMsg {
	chunk, ok := <-ch
	if !ok {
		return streamChunkMsg{chunk: api.StreamChunk{Done: true}}
	}
	return streamChunkMsg{chunk: chunk, ch: ch}
}

func (m ChatModel) handleStreamChunk(msg streamChunkMsg) (tea.Model, tea.Cmd) {
	if m.interrupted {
		return m, nil
	}

	chunk := msg.chunk
	if chunk.Error != nil {
		m.waiting = false
		m.addMsg(chatMsg{role: "system", text: "Error: " + chunk.Error.Error()})
		return m, m.flushPrints()
	}
	if chunk.Done {
		m.waiting = false
		if m.streaming != "" {
			m.addMsg(chatMsg{
				role:     "model",
				text:     m.streaming,
				thoughts: m.streamingThoughts,
				images:   m.streamingImages,
			})
		}
		m.streaming = ""
		m.streamingThoughts = ""
		m.streamingImages = nil
		return m, m.flushPrints()
	}
	if chunk.Text != "" {
		m.streaming = chunk.Text
	}
	if chunk.Thoughts != "" {
		m.streamingThoughts = chunk.Thoughts
	}
	if len(chunk.Images) > 0 {
		m.streamingImages = append(m.streamingImages, chunk.Images...)
	}
	// Update session metadata from chunk (safe: runs on main Bubbletea goroutine)
	if chunk.ConversationID != "" {
		m.session.ConversationID = chunk.ConversationID
	}
	if chunk.ResponseID != "" {
		m.session.ResponseID = chunk.ResponseID
	}
	if chunk.ChoiceID != "" {
		m.session.ChoiceID = chunk.ChoiceID
	}
	ch := msg.ch
	return m, func() tea.Msg {
		return readNextChunk(ch)
	}
}

func (m *ChatModel) renderMessage(msg chatMsg) string {
	var b strings.Builder
	switch msg.role {
	case "user":
		b.WriteString(userMsgStyle.Render("You: "))
		b.WriteString(msg.text)
	case "model":
		if msg.thoughts != "" {
			b.WriteString(m.renderThoughts(msg.thoughts))
			b.WriteString("\n")
		}
		b.WriteString(modelMsgStyle.Render("Gemini:"))
		if m.renderer != nil {
			if rendered, err := m.renderer.Render(msg.text); err == nil {
				b.WriteString("\n")
				b.WriteString(strings.TrimRight(rendered, "\n"))
			} else {
				b.WriteString(" ")
				b.WriteString(msg.text)
			}
		} else {
			b.WriteString(" ")
			b.WriteString(msg.text)
		}
		for _, img := range msg.images {
			label := "Image"
			if img.Generated {
				label = "Generated"
			}
			if img.Title != "" {
				b.WriteString(fmt.Sprintf("\n[%s: %s] %s", label, img.Title, img.URL))
			} else {
				b.WriteString(fmt.Sprintf("\n[%s] %s", label, img.URL))
			}
		}
	case "system":
		b.WriteString(statusStyle.Render(msg.text))
	}
	return b.String()
}

// addMsg renders a chat message and queues it for printing to scrollback.
func (m *ChatModel) addMsg(msg chatMsg) {
	rendered := m.renderMessage(msg)
	m.pendingPrints = append(m.pendingPrints, rendered)
}

// flushPrints returns a tea.Cmd that prints all queued messages to terminal scrollback.
func (m *ChatModel) flushPrints() tea.Cmd {
	if len(m.pendingPrints) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, line := range m.pendingPrints {
		l := line // capture
		cmds = append(cmds, tea.Println(l))
	}
	m.pendingPrints = nil
	return tea.Batch(cmds...)
}

type uploadDoneMsg struct {
	file     api.FileRef
	err      error
	question string // if non-empty, auto-send this question after upload
}

func (m ChatModel) uploadFile(path, question string) tea.Cmd {
	return func() tea.Msg {
		ref, err := m.client.UploadFile(path)
		return uploadDoneMsg{file: ref, err: err, question: question}
	}
}

func (m ChatModel) handleUploadDone(msg uploadDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.addMsg(chatMsg{role: "system", text: "Upload failed: " + msg.err.Error()})
		return m, m.flushPrints()
	}

	if msg.question != "" {
		m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("File uploaded (ID: %s).", msg.file.ID)})
		m.addMsg(chatMsg{role: "user", text: msg.question})
		m.waiting = true
		m.interrupted = false
		m.streaming = ""
		m.streamingThoughts = ""
		m.streamingImages = nil
		return m, tea.Batch(m.flushPrints(), m.sendMessageWithFiles(msg.question, []api.FileRef{msg.file}))
	}

	m.pendingFiles = append(m.pendingFiles, msg.file)
	m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("File uploaded (ID: %s). It will be attached to your next message.", msg.file.ID)})
	return m, m.flushPrints()
}

func (m ChatModel) loadHistory() tea.Cmd {
	return func() tea.Msg {
		convs, err := m.client.ListConversations()
		return historyLoadedMsg{convs: convs, err: err}
	}
}

func (m ChatModel) handleHistoryLoaded(msg historyLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.historyState = historyNone
		m.addMsg(chatMsg{role: "system", text: "Failed to load history: " + msg.err.Error()})
		return m, m.flushPrints()
	}
	if len(msg.convs) == 0 {
		m.historyState = historyNone
		m.addMsg(chatMsg{role: "system", text: "No conversations found."})
		return m, m.flushPrints()
	}
	m.historyState = historyList
	m.historyConvs = msg.convs
	m.historyPage = 0
	m.historyCursor = 0
	return m, nil
}

func (m ChatModel) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.historyState = historyNone
		m.historyConvs = nil
		m.historyDetail = nil
		return m, nil
	}

	switch m.historyState {
	case historyList:
		return m.handleHistoryListKey(msg)
	case historyView:
		return m.handleHistoryViewKey(msg)
	}
	return m, nil
}

func (m ChatModel) handleHistoryListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pageCount := m.historyPageItemCount()

	switch msg.Type {
	case tea.KeyUp:
		if m.historyCursor > 0 {
			m.historyCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.historyCursor < pageCount-1 {
			m.historyCursor++
		}
		return m, nil
	case tea.KeyLeft:
		if m.historyPage > 0 {
			m.historyPage--
			m.historyCursor = 0
		}
		return m, nil
	case tea.KeyRight:
		maxPage := (len(m.historyConvs) - 1) / historyPageSize
		if m.historyPage < maxPage {
			m.historyPage++
			m.historyCursor = 0
		}
		return m, nil
	case tea.KeyEnter:
		globalIdx := m.historyPage*historyPageSize + m.historyCursor
		if globalIdx < len(m.historyConvs) {
			conv := m.historyConvs[globalIdx]
			m.historyState = historyLoading
			m.historyConvID = conv.ID
			m.historyConvTitle = conv.Title
			return m, m.loadConversationDetail(conv.ID, conv.Title)
		}
	}
	return m, nil
}

func (m ChatModel) handleHistoryViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.historyScroll > 0 {
			m.historyScroll--
		}
		return m, nil
	case tea.KeyDown:
		m.historyScroll++
		return m, nil
	}

	switch msg.String() {
	case "c":
		if m.historyDetail != nil {
			m.session = &api.ChatSession{
				ConversationID: m.historyConvID,
				ResponseID:     m.historyDetail.ResponseID,
				ChoiceID:       m.historyDetail.ChoiceID,
			}
			title := m.historyConvTitle
			m.historyState = historyNone
			m.historyConvs = nil
			m.historyDetail = nil
			m.addMsg(chatMsg{role: "system", text: fmt.Sprintf("Continuing conversation: %s\nType your message to continue.", title)})
			return m, m.flushPrints()
		}
	case "b":
		m.historyState = historyList
		m.historyDetail = nil
		m.historyScroll = 0
		return m, nil
	}
	return m, nil
}

func (m ChatModel) handleConversationDetail(msg conversationDetailMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.historyState = historyList
		m.addMsg(chatMsg{role: "system", text: "Failed to load conversation: " + msg.err.Error()})
		return m, m.flushPrints()
	}
	m.historyState = historyView
	m.historyDetail = msg.detail
	m.historyConvID = msg.convID
	m.historyConvTitle = msg.title
	m.historyScroll = 0
	return m, nil
}

func (m ChatModel) viewHistoryList() string {
	var b strings.Builder
	totalPages := (len(m.historyConvs) + historyPageSize - 1) / historyPageSize

	b.WriteString(titleStyle.Render(fmt.Sprintf("Conversation History (%d/%d)", m.historyPage+1, totalPages)))
	b.WriteString("\n\n")

	start := m.historyPage * historyPageSize
	end := start + historyPageSize
	if end > len(m.historyConvs) {
		end = len(m.historyConvs)
	}

	for i, c := range m.historyConvs[start:end] {
		if i == m.historyCursor {
			b.WriteString(userMsgStyle.Render("▸ "))
			b.WriteString(lipgloss.NewStyle().Bold(true).Render(c.Title))
		} else {
			b.WriteString("  ")
			b.WriteString(c.Title)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑↓ select | ← → page | Enter open | ESC back"))
	return b.String()
}

func (m ChatModel) loadConversationDetail(convID, title string) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.client.GetConversationDetail(convID)
		return conversationDetailMsg{detail: detail, convID: convID, title: title, err: err}
	}
}

func (m ChatModel) historyPageItemCount() int {
	start := m.historyPage * historyPageSize
	end := start + historyPageSize
	if end > len(m.historyConvs) {
		end = len(m.historyConvs)
	}
	return end - start
}

func (m ChatModel) viewHistoryDetail() string {
	if m.historyDetail == nil {
		return ""
	}

	var lines []string
	lines = append(lines, titleStyle.Render("── "+m.historyConvTitle+" ──"))
	lines = append(lines, "")

	for _, msg := range m.historyDetail.Messages {
		chatMsg := chatMsg{
			role: msg.Role,
			text: msg.Text,
		}
		rendered := m.renderMessage(chatMsg)
		// Split rendered output into lines for scrolling
		for _, line := range strings.Split(rendered, "\n") {
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	lines = append(lines, helpStyle.Render("[c] Continue | [b] Back | ↑↓ scroll | ESC exit"))

	// Apply scroll offset within available height
	viewH := m.height - 1
	if viewH < 5 {
		viewH = 20
	}

	if m.historyScroll > len(lines)-viewH {
		m.historyScroll = len(lines) - viewH
	}
	if m.historyScroll < 0 {
		m.historyScroll = 0
	}

	end := m.historyScroll + viewH
	if end > len(lines) {
		end = len(lines)
	}

	visible := lines[m.historyScroll:end]
	return strings.Join(visible, "\n")
}

func (m *ChatModel) renderThoughts(thoughts string) string {
	normalized := normalizeThoughts(thoughts)
	if normalized == "" {
		return ""
	}
	if !m.showThoughts {
		return mutedStyle.Render("Thinking: [collapsed, run /thoughts on to expand]")
	}
	return mutedStyle.Render("Thinking:\n" + normalized)
}

func normalizeThoughts(thoughts string) string {
	if thoughts == "" {
		return ""
	}

	thoughts = strings.ReplaceAll(thoughts, "\r\n", "\n")
	lines := strings.Split(thoughts, "\n")
	out := make([]string, 0, len(lines))
	blankRun := 0

	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			blankRun++
			if blankRun > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blankRun = 0
		out = append(out, line)
	}

	start := 0
	for start < len(out) && out[start] == "" {
		start++
	}
	end := len(out)
	for end > start && out[end-1] == "" {
		end--
	}
	if start >= end {
		return ""
	}
	return strings.Join(out[start:end], "\n")
}
