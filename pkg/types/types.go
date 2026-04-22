// Package types defines all shared data structures for sessioneer.
// No interfaces are defined here — only concrete types and enums.
package types

import "time"

// Provider identifies which AI agent owns the session.
type Provider string

const (
	ProviderClaude  Provider = "claude"
	ProviderCopilot Provider = "copilot"
)

// Role identifies the author of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Session is the normalized, provider-agnostic representation of a session.
type Session struct {
	ID        string
	Title     string
	Provider  Provider
	FilePath  string
	CreatedAt time.Time
	UpdatedAt time.Time
	Messages  []Message
	IsEmpty   bool
}

// Message is a single turn in a session conversation.
type Message struct {
	ID        string
	ParentID  string
	Role      Role
	Content   string
	Timestamp time.Time
	// TokenUsage is non-nil only for assistant messages that report token counts.
	TokenUsage *TokenUsage
	// ToolName is set when the message represents a tool call or tool result.
	ToolName string
	IsTool   bool
}

// TokenUsage holds per-message or per-session token statistics.
type TokenUsage struct {
	InputTokens   int
	OutputTokens  int
	CacheCreation int
	CacheRead     int
	ModelID       string
}

// SessionStats aggregates metrics for one or many sessions.
type SessionStats struct {
	SessionCount  int
	MessageCount  int
	UserMessages  int
	LLMMessages   int
	Duration      time.Duration
	Tokens        TokenUsage
	CostUSD       float64
	ToolUsage     map[string]int
	SubagentCount int
}

// SearchResult is a single hit when searching across sessions.
type SearchResult struct {
	Session    Session
	MessageIdx int
	Snippet    string
	MatchStart int
	MatchEnd   int
}

// PruneOption describes a category of removable content in a session.
type PruneOption struct {
	Label    string
	Count    int
	Selected bool
}

// AppConfig holds resolved CLI flags and defaults.
type AppConfig struct {
	Provider    Provider // empty = auto-detect
	ProjectPath string
	BaseDir     string
}