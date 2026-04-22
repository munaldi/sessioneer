package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/munaldi/sessioneer/pkg/types"
)

// claudeEntry is a single line in a Claude Code .jsonl file.
type claudeEntry struct {
	UUID        string         `json:"uuid"`
	ParentUUID  string         `json:"parentUuid"`
	Type        string         `json:"type"` // "user", "assistant", "summary", "custom-title"
	Timestamp   string         `json:"timestamp"`
	Message     *claudeMessage `json:"message,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	CustomTitle string         `json:"customTitle,omitempty"`
}

type claudeMessage struct {
	Role    string        `json:"role"`
	Content claudeContent  `json:"content"`
	Usage   *claudeUsage   `json:"usage,omitempty"`
	Model   string        `json:"model,omitempty"`
}

// claudeContent can be either a string or a list of content blocks.
type claudeContent struct {
	Text   string
	Blocks []claudeContentBlock
}

func (c *claudeContent) UnmarshalJSON(data []byte) error {
	var s string
	if json.Unmarshal(data, &s) == nil {
		c.Text = s
		return nil
	}
	return json.Unmarshal(data, &c.Blocks)
}

func (c claudeContent) MarshalJSON() ([]byte, error) {
	if len(c.Blocks) > 0 {
		return json.Marshal(c.Blocks)
	}
	return json.Marshal(c.Text)
}

type claudeContentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text,omitempty"`
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	ToolUse string          `json:"tool_use_id,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
}

type claudeUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
}

func loadClaude(filePath string) (types.Session, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return types.Session{}, fmt.Errorf("open %q: %w", filePath, err)
	}
	defer f.Close()

	info, _ := f.Stat()

	var (
		entries     []claudeEntry
		title       string
		latestTitle string
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry claudeEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
		if entry.Type == "custom-title" && entry.CustomTitle != "" {
			latestTitle = entry.CustomTitle
		}
	}

	if latestTitle != "" {
		title = latestTitle
	} else {
		title = strings.TrimSuffix(filepath.Base(filePath), ".jsonl")
	}

	isEmpty := len(entries) == 0
	messages := make([]types.Message, 0, len(entries))
	var createdAt, updatedAt time.Time

	for _, e := range entries {
		ts, _ := time.Parse(time.RFC3339Nano, e.Timestamp)
		if createdAt.IsZero() || ts.Before(createdAt) {
			createdAt = ts
		}
		if ts.After(updatedAt) {
			updatedAt = ts
		}

		if e.Message == nil {
			continue
		}

		content, isTool, toolName := extractClaudeContent(e.Message)
		var usage *types.TokenUsage
		if e.Message.Usage != nil {
			usage = &types.TokenUsage{
				InputTokens:   e.Message.Usage.InputTokens,
				OutputTokens:  e.Message.Usage.OutputTokens,
				CacheCreation: e.Message.Usage.CacheCreationTokens,
				CacheRead:     e.Message.Usage.CacheReadTokens,
				ModelID:       e.Message.Model,
			}
		}

		messages = append(messages, types.Message{
			ID:         e.UUID,
			ParentID:   e.ParentUUID,
			Role:       types.Role(e.Message.Role),
			Content:    content,
			Timestamp:  ts,
			TokenUsage: usage,
			IsTool:     isTool,
			ToolName:   toolName,
		})
	}

	if info != nil && updatedAt.IsZero() {
		updatedAt = info.ModTime()
	}

	sessionID := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")

	return types.Session{
		ID:        sessionID,
		Title:     title,
		Provider:  types.ProviderClaude,
		FilePath:  filePath,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Messages:  messages,
		IsEmpty:   isEmpty,
	}, nil
}

func extractClaudeContent(msg *claudeMessage) (text string, isTool bool, toolName string) {
	if msg.Content.Text != "" {
		return msg.Content.Text, false, ""
	}
	var parts []string
	for _, b := range msg.Content.Blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "tool_use":
			isTool = true
			toolName = b.Name
		case "tool_result":
			isTool = true
		}
	}
	return strings.Join(parts, "\n"), isTool, toolName
}

func saveClaude(s types.Session) error {
	f, err := os.Create(s.FilePath)
	if err != nil {
		return fmt.Errorf("create %q: %w", s.FilePath, err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, msg := range s.Messages {
		content := claudeContent{Text: msg.Content}
		entry := claudeEntry{
			UUID:       msg.ID,
			ParentUUID: msg.ParentID,
			Type:       string(msg.Role),
			Timestamp:  msg.Timestamp.Format(time.RFC3339Nano),
			Message: &claudeMessage{
				Role:    string(msg.Role),
				Content: content,
			},
		}
		if err := enc.Encode(entry); err != nil {
			return fmt.Errorf("encode entry: %w", err)
		}
	}
	titleEntry := claudeEntry{
		Type:        "custom-title",
		CustomTitle: s.Title,
		Timestamp:   time.Now().Format(time.RFC3339Nano),
	}
	return enc.Encode(titleEntry)
}