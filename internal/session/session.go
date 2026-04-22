package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/munaldi/sessioneer/pkg/types"
)

// List loads every session file for a provider from the given base directory.
func List(p types.Provider, baseDir string) ([]types.Session, error) {
	if baseDir == "" {
		return []types.Session{}, nil
	}
	info, err := os.Stat(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []types.Session{}, nil
		}
		return nil, fmt.Errorf("stat %q: %w", baseDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", baseDir)
	}

	var sessions []types.Session
	err = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		switch p {
		case types.ProviderClaude:
			if strings.HasSuffix(path, ".jsonl") {
				if s, err := loadClaude(path); err == nil {
					sessions = append(sessions, s)
				}
			}
		case types.ProviderCopilot:
			if strings.HasSuffix(path, ".json") {
				if s, err := loadCopilot(path); err == nil {
					sessions = append(sessions, s)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].UpdatedAt.Equal(sessions[j].UpdatedAt) {
			return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
		}
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Save writes a normalized session back to disk using the provider format.
func Save(s types.Session) error {
	if err := os.MkdirAll(filepath.Dir(s.FilePath), 0o755); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	switch s.Provider {
	case types.ProviderClaude:
		return saveClaude(s)
	case types.ProviderCopilot:
		return saveCopilot(s)
	default:
		return fmt.Errorf("unknown provider %q", s.Provider)
	}
}

// Stats calculates summary metrics for one session.
func Stats(s types.Session) types.SessionStats {
	stats := types.SessionStats{ToolUsage: map[string]int{}}
	stats.SessionCount = 1
	stats.MessageCount = len(s.Messages)
	stats.Duration = s.UpdatedAt.Sub(s.CreatedAt)
	for _, msg := range s.Messages {
		switch msg.Role {
		case types.RoleUser:
			stats.UserMessages++
		case types.RoleAssistant:
			stats.LLMMessages++
		}
		if msg.IsTool {
			stats.ToolUsage[msg.ToolName]++
		}
		if msg.TokenUsage != nil {
			stats.Tokens.InputTokens += msg.TokenUsage.InputTokens
			stats.Tokens.OutputTokens += msg.TokenUsage.OutputTokens
			stats.Tokens.CacheCreation += msg.TokenUsage.CacheCreation
			stats.Tokens.CacheRead += msg.TokenUsage.CacheRead
			if stats.Tokens.ModelID == "" {
				stats.Tokens.ModelID = msg.TokenUsage.ModelID
			}
		}
	}
	return stats
}

// ProjectStats aggregates metrics across many sessions.
func ProjectStats(sessions []types.Session) types.SessionStats {
	stats := types.SessionStats{SessionCount: len(sessions), ToolUsage: map[string]int{}}
	for _, s := range sessions {
		stats.MessageCount += len(s.Messages)
		stats.Duration += s.UpdatedAt.Sub(s.CreatedAt)
		for _, msg := range s.Messages {
			switch msg.Role {
			case types.RoleUser:
				stats.UserMessages++
			case types.RoleAssistant:
				stats.LLMMessages++
			}
			if msg.IsTool {
				stats.ToolUsage[msg.ToolName]++
			}
			if msg.TokenUsage != nil {
				stats.Tokens.InputTokens += msg.TokenUsage.InputTokens
				stats.Tokens.OutputTokens += msg.TokenUsage.OutputTokens
				stats.Tokens.CacheCreation += msg.TokenUsage.CacheCreation
				stats.Tokens.CacheRead += msg.TokenUsage.CacheRead
			}
		}
	}
	return stats
}

func providerExt(p types.Provider) string {
	switch p {
	case types.ProviderClaude:
		return ".jsonl"
	case types.ProviderCopilot:
		return ".json"
	default:
		return ""
	}
}

func baseSessionName(filePath string, provider types.Provider) string {
	name := strings.TrimSuffix(filepath.Base(filePath), providerExt(provider))
	if name == "" {
		return "session"
	}
	return name
}

func newMessageID(base string, index int) string {
	return fmt.Sprintf("%s-%03d", base, index+1)
}

func cloneSessionMessages(messages []types.Message, base string) []types.Message {
	cloned := make([]types.Message, 0, len(messages))
	for i, msg := range messages {
		msg.ID = newMessageID(base, i)
		if i == 0 {
			msg.ParentID = ""
		} else {
			msg.ParentID = cloned[i-1].ID
		}
		cloned = append(cloned, msg)
	}
	return cloned
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

func uniqueSessionPath(dir, title string, provider types.Provider) string {
	base := normalizeTitle(title)
	ext := providerExt(provider)
	if ext == "" {
		ext = ".json"
	}
	candidate := filepath.Join(dir, base+ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
	}
}

// newForkedSession returns a copy of a session with a new file path and IDs.
func newForkedSession(s types.Session, title string) types.Session {
	copySession := s
	copySession.Title = title
	copySession.ID = normalizeTitle(title)
	copySession.FilePath = uniqueSessionPath(filepath.Dir(s.FilePath), title, s.Provider)
	copySession.Messages = cloneSessionMessages(s.Messages, copySession.ID)
	copySession.CreatedAt = time.Now()
	copySession.UpdatedAt = copySession.CreatedAt
	return copySession
}