package actions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/munaldi/sessioneer/internal/session"
	"github.com/munaldi/sessioneer/pkg/types"
)

// PruneOptions controls which categories are removed from a session.
type PruneOptions struct {
	RemoveToolBlocks    bool
	RemoveEmptyMessages bool
	RemoveSystemTags    bool
	RemoveShortMessages bool
}

// Open reveals the session directory in the system file manager.
func Open(s types.Session) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", filepath.Dir(s.FilePath))
	case "windows":
		cmd = exec.Command("explorer", filepath.Dir(s.FilePath))
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(s.FilePath))
	}
	return cmd.Start()
}

// CleanEmpty removes empty session files.
func CleanEmpty(sessions []types.Session) (int, error) {
	removed := 0
	for _, s := range sessions {
		if !s.IsEmpty {
			continue
		}
		if err := os.Remove(s.FilePath); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// AnalyzePrune reports candidate counts for each prune category.
func AnalyzePrune(s types.Session) []types.PruneOption {
	var toolBlocks, emptyMessages, systemTags, shortMessages int
	for _, msg := range s.Messages {
		if msg.IsTool {
			toolBlocks++
		}
		if strings.TrimSpace(msg.Content) == "" {
			emptyMessages++
		}
		if strings.Contains(msg.Content, "<system-reminder>") || strings.Contains(msg.Content, "<ide_selection>") || strings.Contains(msg.Content, "<ide_opened_file>") {
			systemTags++
		}
		if len(strings.TrimSpace(msg.Content)) > 0 && len(strings.TrimSpace(msg.Content)) < 50 {
			shortMessages++
		}
	}
	return []types.PruneOption{
		{Label: "Tool blocks", Count: toolBlocks},
		{Label: "Empty messages", Count: emptyMessages},
		{Label: "System/IDE tags", Count: systemTags},
		{Label: "Short messages", Count: shortMessages},
	}
}

// Prune removes selected categories and saves the session.
func Prune(s types.Session, opts PruneOptions) (types.Session, error) {
	filtered := make([]types.Message, 0, len(s.Messages))
	for _, msg := range s.Messages {
		if opts.RemoveToolBlocks && msg.IsTool {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if opts.RemoveEmptyMessages && content == "" {
			continue
		}
		if opts.RemoveSystemTags && (strings.Contains(content, "<system-reminder>") || strings.Contains(content, "<ide_selection>") || strings.Contains(content, "<ide_opened_file>")) {
			continue
		}
		if opts.RemoveShortMessages && len(content) > 0 && len(content) < 50 {
			continue
		}
		filtered = append(filtered, msg)
	}
	s.Messages = filtered
	s.IsEmpty = len(filtered) == 0
	s.UpdatedAt = time.Now()
	return s, session.Save(s)
}

// Trim keeps messages through the cutoff index, inclusive.
func Trim(s types.Session, cutoff int) (types.Session, error) {
	if cutoff < 0 || cutoff >= len(s.Messages) {
		return s, fmt.Errorf("trim cutoff %d out of range", cutoff)
	}
	s.Messages = append([]types.Message(nil), s.Messages[:cutoff+1]...)
	s.IsEmpty = len(s.Messages) == 0
	s.UpdatedAt = time.Now()
	return s, session.Save(s)
}

// Delete removes the session file from disk.
func Delete(s types.Session) error {
	return os.Remove(s.FilePath)
}

// Rename updates the session title and saves it.
func Rename(s types.Session, title string) (types.Session, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return s, fmt.Errorf("title cannot be empty")
	}
	s.Title = title
	s.UpdatedAt = time.Now()
	return s, session.Save(s)
}

// Fork creates a copy of the session with a new title and file path.
func Fork(s types.Session, title string) (types.Session, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return s, fmt.Errorf("title cannot be empty")
	}
	cloned := cloneForNewFile(s, title)
	return cloned, session.Save(cloned)
}

// Merge combines multiple sessions into a single new session.
func Merge(sessions []types.Session, title string) (types.Session, error) {
	if len(sessions) == 0 {
		return types.Session{}, fmt.Errorf("no sessions to merge")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return types.Session{}, fmt.Errorf("title cannot be empty")
	}
	merged := types.Session{
		ID:        normalizeTitle(title),
		Title:     title,
		Provider:  sessions[0].Provider,
		FilePath:  uniqueMergedPath(filepath.Dir(sessions[0].FilePath), title, sessions[0].Provider),
		CreatedAt: sessions[0].CreatedAt,
		UpdatedAt: time.Now(),
	}

	var messages []types.Message
	for _, s := range sessions {
		messages = append(messages, s.Messages...)
		if s.CreatedAt.Before(merged.CreatedAt) {
			merged.CreatedAt = s.CreatedAt
		}
		if s.UpdatedAt.After(merged.UpdatedAt) {
			merged.UpdatedAt = s.UpdatedAt
		}
	}
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].Timestamp.Equal(messages[j].Timestamp) {
			return messages[i].ID < messages[j].ID
		}
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})
	merged.Messages = remapMessages(messages, merged.ID)
	return merged, session.Save(merged)
}

// Search looks for a query in titles and messages.
func Search(sessions []types.Session, query string) []types.SearchResult {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil
	}
	results := make([]types.SearchResult, 0)
	for _, s := range sessions {
		if idx := strings.Index(strings.ToLower(s.Title), query); idx >= 0 {
			results = append(results, types.SearchResult{Session: s, MessageIdx: 0, Snippet: s.Title, MatchStart: idx, MatchEnd: idx + len(query)})
			continue
		}
		for i, msg := range s.Messages {
			content := strings.ToLower(msg.Content)
			idx := strings.Index(content, query)
			if idx < 0 {
				continue
			}
			snippet := msg.Content
			if len(snippet) > 120 {
				start := idx - 30
				if start < 0 {
					start = 0
				}
				end := idx + len(query) + 30
				if end > len(snippet) {
					end = len(snippet)
				}
				snippet = snippet[start:end]
			}
			results = append(results, types.SearchResult{Session: s, MessageIdx: i, Snippet: snippet, MatchStart: idx, MatchEnd: idx + len(query)})
		}
	}
	return results
}

func cloneForNewFile(s types.Session, title string) types.Session {
	cloned := s
	cloned.Title = title
	cloned.ID = normalizeTitle(title)
	cloned.FilePath = uniqueNewPath(filepath.Dir(s.FilePath), title, s.Provider)
	cloned.Messages = remapMessages(s.Messages, cloned.ID)
	cloned.CreatedAt = time.Now()
	cloned.UpdatedAt = cloned.CreatedAt
	cloned.IsEmpty = len(cloned.Messages) == 0
	return cloned
}

func remapMessages(messages []types.Message, base string) []types.Message {
	remapped := make([]types.Message, 0, len(messages))
	for i, msg := range messages {
		msg.ID = fmt.Sprintf("%s-%03d", base, i+1)
		if i == 0 {
			msg.ParentID = ""
		} else {
			msg.ParentID = remapped[i-1].ID
		}
		remapped = append(remapped, msg)
	}
	return remapped
}

func normalizeTitle(title string) string {
	title = strings.TrimSpace(strings.ToLower(title))
	if title == "" {
		return "session"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "session"
	}
	return result
}

func providerExt(p types.Provider) string {
	switch p {
	case types.ProviderClaude:
		return ".jsonl"
	case types.ProviderCopilot:
		return ".json"
	default:
		return ".json"
	}
}

func uniqueNewPath(dir, title string, provider types.Provider) string {
	base := normalizeTitle(title)
	ext := providerExt(provider)
	candidate := filepath.Join(dir, base+ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
	}
}

func uniqueMergedPath(dir, title string, provider types.Provider) string {
	return uniqueNewPath(dir, title, provider)
}