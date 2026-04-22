package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/munaldi/sessioneer/pkg/types"
)

// copilotFile is the top-level structure of a Copilot conversation JSON file.
type copilotFile struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	CreatedAt string            `json:"createdAt"`
	UpdatedAt string            `json:"updatedAt"`
	Exchanges []copilotExchange  `json:"exchanges"`
}

// copilotExchange is one user→assistant turn.
type copilotExchange struct {
	ID        string         `json:"id"`
	CreatedAt string         `json:"createdAt"`
	Request   copilotMessage `json:"request"`
	Response  copilotMessage `json:"response"`
}

type copilotMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func loadCopilot(filePath string) (types.Session, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return types.Session{}, fmt.Errorf("read %q: %w", filePath, err)
	}

	var cf copilotFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return types.Session{}, fmt.Errorf("parse %q: %w", filePath, err)
	}

	createdAt, _ := time.Parse(time.RFC3339Nano, cf.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, cf.UpdatedAt)

	if updatedAt.IsZero() {
		if info, err := os.Stat(filePath); err == nil {
			updatedAt = info.ModTime()
		}
	}

	title := cf.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filePath), ".json")
	}

	messages := make([]types.Message, 0, len(cf.Exchanges)*2)
	for _, ex := range cf.Exchanges {
		ts, _ := time.Parse(time.RFC3339Nano, ex.CreatedAt)

		messages = append(messages, types.Message{
			ID:        ex.ID + "-req",
			ParentID:  "",
			Role:      types.RoleUser,
			Content:   ex.Request.Content,
			Timestamp: ts,
		})
		messages = append(messages, types.Message{
			ID:        ex.ID + "-res",
			ParentID:  ex.ID + "-req",
			Role:      types.RoleAssistant,
			Content:   ex.Response.Content,
			Timestamp: ts,
		})
	}

	return types.Session{
		ID:        cf.ID,
		Title:     title,
		Provider:  types.ProviderCopilot,
		FilePath:  filePath,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Messages:  messages,
		IsEmpty:   len(cf.Exchanges) == 0,
	}, nil
}

func saveCopilot(s types.Session) error {
	var exchanges []copilotExchange
	for i := 0; i+1 < len(s.Messages); i += 2 {
		req := s.Messages[i]
		res := s.Messages[i+1]
		ex := copilotExchange{
			ID:        strings.TrimSuffix(req.ID, "-req"),
			CreatedAt: req.Timestamp.Format(time.RFC3339Nano),
			Request:   copilotMessage{Role: "user", Content: req.Content},
			Response:  copilotMessage{Role: "assistant", Content: res.Content},
		}
		exchanges = append(exchanges, ex)
	}

	cf := copilotFile{
		ID:        s.ID,
		Title:     s.Title,
		CreatedAt: s.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: time.Now().Format(time.RFC3339Nano),
		Exchanges: exchanges,
	}

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return os.WriteFile(s.FilePath, data, 0o644)
}