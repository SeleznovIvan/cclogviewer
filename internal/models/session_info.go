package models

import "time"

// SessionInfo represents metadata about a Claude Code session.
type SessionInfo struct {
	SessionID        string    `json:"session_id"`
	Project          string    `json:"project"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	MessageCount     int       `json:"message_count"`
	AgentTypesUsed   []string  `json:"agent_types_used,omitempty"`
	FirstUserMessage string    `json:"first_user_message,omitempty"`
	CWD              string    `json:"cwd,omitempty"`
	GitBranch        string    `json:"git_branch,omitempty"`
	FilePath         string    `json:"-"` // Internal use only
}

// SessionLogs represents full processed logs for a session.
type SessionLogs struct {
	SessionID  string              `json:"session_id"`
	Project    string              `json:"project"`
	Entries    []SessionLogEntry   `json:"entries"`
	TokenStats *SessionTokenStats  `json:"token_stats,omitempty"`
}

// SessionLogEntry represents a single entry in session logs.
type SessionLogEntry struct {
	UUID        string              `json:"uuid"`
	Timestamp   string              `json:"timestamp"`
	Role        string              `json:"role"`
	Content     string              `json:"content"`
	IsSidechain bool                `json:"is_sidechain,omitempty"`
	AgentID     string              `json:"agent_id,omitempty"`
	ToolCalls   []SessionToolCall   `json:"tool_calls,omitempty"`
}

// SessionToolCall represents a tool call in session logs.
type SessionToolCall struct {
	Name   string      `json:"name"`
	Input  interface{} `json:"input,omitempty"`
	Output string      `json:"output,omitempty"`
}

// SessionTokenStats represents token usage statistics.
type SessionTokenStats struct {
	TotalInput     int `json:"total_input"`
	TotalOutput    int `json:"total_output"`
	CacheRead      int `json:"cache_read"`
	CacheCreation  int `json:"cache_creation"`
}
