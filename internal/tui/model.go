// Package tui implements the interactive terminal UI using Bubble Tea.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/munaldi/sessioneer/internal/actions"
	"github.com/munaldi/sessioneer/internal/provider"
	"github.com/munaldi/sessioneer/internal/session"
	"github.com/munaldi/sessioneer/pkg/types"
)

// screen identifies which view is currently rendered.
type screen int

const (
	screenProviderPick screen = iota
	screenMainMenu
	screenSessionList
	screenSessionAction
	screenSearch
	screenSearchResults
	screenStats
	screenInput // generic single-line text input
)

// inputPurpose says what we'll do with the text once confirmed.
type inputPurpose int

const (
	purposeRename inputPurpose = iota
	purposeForkTitle
	purposeMergeTitle
	purposeSearchQuery
)

// Model is the single source of truth for the TUI.
type Model struct {
	screen  screen
	err     error
	width   int
	height  int

	// Provider selection
	providers     []types.Provider
	providerCursor int

	// Loaded sessions
	sessions      []types.Session
	baseDir       string
	activeProvider types.Provider

	// Session list
	sessionCursor int
	sessionPage   int
	pageSize      int

	// Actions
	actionCursor  int
	activeSession types.Session
	pruneOpts     []types.PruneOption
	pruneCursor   int

	// Merge multi-select
	mergeSelected map[int]bool
	mergeCursor   int

	// Search
	searchQuery   string
	searchResults []types.SearchResult
	searchCursor  int

	// Generic text input
	textInput     textinput.Model
	inputPurpose  inputPurpose
	inputLabel    string

	// Stats
	stats types.SessionStats

	// Status message shown briefly after an action
	statusMsg string
}

// New initialises the model. If p is non-empty, skip provider selection.
func New(p types.Provider, baseDir string) Model {
	ti := textinput.New()
	ti.CharLimit = 200

	m := Model{
		textInput: ti,
		pageSize:  15,
	}

	if p != "" {
		m.activeProvider = p
		m.baseDir = baseDir
		m.screen = screenMainMenu
		m.sessions, m.err = session.List(p, baseDir)
	} else {
		m.providers = provider.Detect()
		if len(m.providers) == 0 {
			m.providers = []types.Provider{types.ProviderClaude, types.ProviderCopilot}
		}
		m.screen = screenProviderPick
	}

	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

// --- Update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenProviderPick:
		return m.updateProviderPick(msg)
	case screenMainMenu:
		return m.updateMainMenu(msg)
	case screenSessionList:
		return m.updateSessionList(msg)
	case screenSessionAction:
		return m.updateSessionAction(msg)
	case screenSearch:
		return m.updateSearch(msg)
	case screenSearchResults:
		return m.updateSearchResults(msg)
	case screenStats:
		return m.updateStats(msg)
	case screenInput:
		return m.updateInput(msg)
	}
	return m, nil
}

// --- Provider pick ---

func (m Model) updateProviderPick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
	case "down", "j":
		if m.providerCursor < len(m.providers)-1 {
			m.providerCursor++
		}
	case "enter":
		p := m.providers[m.providerCursor]
		m.activeProvider = p
		dir, err := provider.DefaultBaseDir(p)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.baseDir = dir
		m.sessions, m.err = session.List(p, dir)
		m.screen = screenMainMenu
	case "q", "esc":
		if m.activeProvider != "" {
			m.screen = screenMainMenu
		}
	}
	return m, nil
}

// --- Main menu ---

const (
	mainSessions = iota
	mainSearch
	mainStats
	mainClean
	mainSwitch
	mainQuit
)

var mainMenuItems = []string{
	"Sessions",
	"Search",
	"Project Stats",
	"Clean empty sessions",
	"Switch provider",
	"Quit",
}

func (m Model) updateMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	itemCount := len(mainMenuItems)
	switch msg.String() {
	case "up", "k":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case "down", "j":
		if m.actionCursor < itemCount-1 {
			m.actionCursor++
		}
	case "enter":
		switch m.actionCursor {
		case mainSessions:
			m.screen = screenSessionList
			m.sessionCursor = 0
		case mainSearch:
			m.openInput(purposeSearchQuery, "Search across all sessions:")
		case mainStats:
			m.stats = session.ProjectStats(m.sessions)
			m.screen = screenStats
		case mainClean:
			count, err := actions.CleanEmpty(m.sessions)
			if err != nil {
				m.err = err
				return m, nil
			}
			m.sessions, _ = session.List(m.activeProvider, m.baseDir)
			m.statusMsg = fmt.Sprintf("Removed %d empty session(s).", count)
		case mainSwitch:
			m.screen = screenProviderPick
		case mainQuit:
			return m, tea.Quit
		}
	}
	return m, nil
}

// --- Session list ---

func (m Model) updateSessionList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.sessionCursor > 0 {
			m.sessionCursor--
		} else if m.sessionPage > 0 {
			m.sessionPage--
			m.sessionCursor = m.pageSize - 1
		}
	case "down", "j":
		if m.sessionCursor < m.pageSize-1 && m.sessionCursor < len(m.sessions)-1-m.sessionPage*m.pageSize {
			m.sessionCursor++
		} else if (m.sessionPage+1)*m.pageSize < len(m.sessions) {
			m.sessionPage++
			m.sessionCursor = 0
		}
	case "enter":
		idx := m.sessionPage*m.pageSize + m.sessionCursor
		if idx < len(m.sessions) {
			m.activeSession = m.sessions[idx]
			m.actionCursor = 0
			m.screen = screenSessionAction
		}
	case "esc", "q":
		m.screen = screenMainMenu
		m.actionCursor = 0
	}
	return m, nil
}

// --- Session action menu ---

const (
	actOpen = iota
	actFork
	actMerge
	actPrune
	actTrim
	actStats
	actRename
	actDelete
	actBack
)

var actionMenuItems = []string{
	"Open in file manager",
	"Fork",
	"Merge",
	"Prune",
	"Trim",
	"Stats",
	"Rename",
	"Delete",
	"← Back",
}

var actionMenuDescriptions = []string{
	"Open the session folder in your OS file manager.",
	"Create a copy with a new title and new message IDs.",
	"Combine selected sessions into a single new session.",
	"Remove tool/system noise and keep meaningful messages.",
	"Cut off messages after a chosen point.",
	"Show usage and message statistics for this session.",
	"Change the session title.",
	"Delete this session file from disk.",
	"Return to the sessions list.",
}

func (m Model) updateSessionAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case "down", "j":
		if m.actionCursor < len(actionMenuItems)-1 {
			m.actionCursor++
		}
	case "enter":
		return m.dispatchAction()
	case "esc", "q":
		m.screen = screenSessionList
	}
	return m, nil
}

func (m Model) dispatchAction() (tea.Model, tea.Cmd) {
	switch m.actionCursor {
	case actOpen:
		if err := actions.Open(m.activeSession); err != nil {
			m.err = err
		} else {
			m.statusMsg = "Opened in file manager."
		}
	case actFork:
		m.openInput(purposeForkTitle, "Fork title:")
	case actMerge:
		m.mergeSelected = make(map[int]bool)
		m.mergeCursor = 0
		m.screen = screenSessionList
	case actPrune:
		m.pruneOpts = actions.AnalyzePrune(m.activeSession)
		m.pruneCursor = 0
		opts := actions.PruneOptions{
			RemoveToolBlocks:    true,
			RemoveEmptyMessages: true,
			RemoveSystemTags:    true,
		}
		updated, err := actions.Prune(m.activeSession, opts)
		if err != nil {
			m.err = err
		} else {
			m.activeSession = updated
			m.statusMsg = "Session pruned."
		}
	case actTrim:
		if len(m.activeSession.Messages) > 1 {
			updated, err := actions.Trim(m.activeSession, len(m.activeSession.Messages)-2)
			if err != nil {
				m.err = err
			} else {
				m.activeSession = updated
				m.statusMsg = "Session trimmed."
			}
		}
	case actStats:
		m.stats = session.Stats(m.activeSession)
		m.screen = screenStats
	case actRename:
		m.openInput(purposeRename, "New title:")
	case actDelete:
		if err := actions.Delete(m.activeSession); err != nil {
			m.err = err
		} else {
			m.sessions, _ = session.List(m.activeProvider, m.baseDir)
			m.screen = screenSessionList
			m.statusMsg = "Session deleted."
		}
	case actBack:
		m.screen = screenSessionList
	}
	return m, nil
}

// --- Search ---

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "enter":
		m.searchQuery = m.textInput.Value()
		m.searchResults = actions.Search(m.sessions, m.searchQuery)
		m.searchCursor = 0
		m.screen = screenSearchResults
		m.textInput.Reset()
		return m, nil
	case "esc":
		m.screen = screenMainMenu
		m.textInput.Reset()
		return m, nil
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) updateSearchResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
	case "down", "j":
		if m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
	case "enter":
		if m.searchCursor < len(m.searchResults) {
			m.activeSession = m.searchResults[m.searchCursor].Session
			m.actionCursor = 0
			m.screen = screenSessionAction
		}
	case "esc", "q":
		m.screen = screenMainMenu
	}
	return m, nil
}

// --- Stats ---

func (m Model) updateStats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.screen = screenMainMenu
	}
	return m, nil
}

// --- Generic text input ---

func (m *Model) openInput(purpose inputPurpose, label string) {
	m.inputPurpose = purpose
	m.inputLabel = label
	m.textInput.Reset()
	m.textInput.Focus()
	m.screen = screenInput
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.textInput.Value())
		m.textInput.Reset()
		return m.commitInput(value)
	case "esc":
		m.textInput.Reset()
		m.screen = screenSessionAction
		return m, nil
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) commitInput(value string) (tea.Model, tea.Cmd) {
	switch m.inputPurpose {
	case purposeRename:
		updated, err := actions.Rename(m.activeSession, value)
		if err != nil {
			m.err = err
		} else {
			m.activeSession = updated
			m.statusMsg = "Session renamed."
		}
		m.screen = screenSessionAction
	case purposeForkTitle:
		forked, err := actions.Fork(m.activeSession, value)
		if err != nil {
			m.err = err
		} else {
			m.sessions, _ = session.List(m.activeProvider, m.baseDir)
			m.activeSession = forked
			m.statusMsg = fmt.Sprintf("Forked as %q.", forked.Title)
		}
		m.screen = screenSessionAction
	case purposeMergeTitle:
		var toMerge []types.Session
		for i, s := range m.sessions {
			if m.mergeSelected[i] {
				toMerge = append(toMerge, s)
			}
		}
		merged, err := actions.Merge(toMerge, value)
		if err != nil {
			m.err = err
		} else {
			m.sessions, _ = session.List(m.activeProvider, m.baseDir)
			m.activeSession = merged
			m.statusMsg = fmt.Sprintf("Merged into %q.", merged.Title)
		}
		m.screen = screenSessionAction
	case purposeSearchQuery:
		m.searchQuery = value
		m.searchResults = actions.Search(m.sessions, value)
		m.searchCursor = 0
		m.screen = screenSearchResults
	}
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress Ctrl+C to quit.", m.err)
	}

	var b strings.Builder
	b.WriteString("Sessioneer\n\n")
	if m.statusMsg != "" {
		b.WriteString(m.statusMsg)
		b.WriteString("\n\n")
	}

	switch m.screen {
	case screenProviderPick:
		b.WriteString("Choose a provider:\n")
		for i, p := range m.providers {
			marker := "  "
			if i == m.providerCursor {
				marker = "> "
			}
			b.WriteString(marker)
			b.WriteString(string(p))
			b.WriteString("\n")
		}
	case screenMainMenu:
		b.WriteString("Main menu:\n")
		for i, item := range mainMenuItems {
			marker := "  "
			if i == m.actionCursor {
				marker = "> "
			}
			b.WriteString(marker)
			b.WriteString(item)
			b.WriteString("\n")
		}
	case screenSessionList:
		b.WriteString("Sessions:\n")
		start := m.sessionPage * m.pageSize
		end := start + m.pageSize
		if end > len(m.sessions) {
			end = len(m.sessions)
		}
		for i := start; i < end; i++ {
			s := m.sessions[i]
			marker := "  "
			if i-start == m.sessionCursor {
				marker = "> "
			}
			b.WriteString(marker)
			b.WriteString(formatSessionSummary(s))
			b.WriteString("\n    ")
			b.WriteString(formatSessionDetails(s))
			b.WriteString("\n")
		}
	case screenSessionAction:
		b.WriteString("Session actions for: ")
		b.WriteString(m.activeSession.Title)
		b.WriteString("\n")
		b.WriteString(formatSessionDetails(m.activeSession))
		b.WriteString("\n\n")
		for i, item := range actionMenuItems {
			marker := "  "
			if i == m.actionCursor {
				marker = "> "
			}
			b.WriteString(marker)
			b.WriteString(item)
			if i < len(actionMenuDescriptions) {
				b.WriteString(" - ")
				b.WriteString(actionMenuDescriptions[i])
			}
			b.WriteString("\n")
		}
	case screenSearch:
		b.WriteString(m.inputLabel)
		b.WriteString(" ")
		b.WriteString(m.textInput.View())
		b.WriteString("\n")
	case screenSearchResults:
		b.WriteString("Search results for \"")
		b.WriteString(m.searchQuery)
		b.WriteString("\":\n")
		for i, result := range m.searchResults {
			s := result.Session
			marker := "  "
			if i == m.searchCursor {
				marker = "> "
			}
			b.WriteString(marker)
			b.WriteString(formatSessionSummary(s))
			b.WriteString("\n    ")
			b.WriteString(formatSessionDetails(s))
			b.WriteString("\n    match: ")
			b.WriteString(result.Snippet)
			b.WriteString("\n")
		}
	case screenStats:
		b.WriteString(fmt.Sprintf("Sessions: %d\nMessages: %d\nUsers: %d\nLLM: %d\n", m.stats.SessionCount, m.stats.MessageCount, m.stats.UserMessages, m.stats.LLMMessages))
	case screenInput:
		b.WriteString(m.inputLabel)
		b.WriteString(" ")
		b.WriteString(m.textInput.View())
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.screen == screenMainMenu {
		b.WriteString("Use Enter on \"Quit\" to exit")
	} else {
		b.WriteString("Esc or q: back")
	}
	return b.String()
}

func formatSessionSummary(s types.Session) string {
	provider := strings.ToUpper(string(s.Provider))
	if provider == "" {
		provider = "UNKNOWN"
	}
	title := strings.TrimSpace(s.Title)
	if title == "" {
		title = "(untitled session)"
	}
	return fmt.Sprintf("[%s] %s", provider, title)
}

func formatSessionDetails(s types.Session) string {
	updated := "unknown"
	if !s.UpdatedAt.IsZero() {
		updated = s.UpdatedAt.Local().Format(time.RFC3339)
	}
	return fmt.Sprintf("messages=%d updated=%s file=%s", len(s.Messages), updated, s.FilePath)
}