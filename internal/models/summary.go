package models

// TokenStats represents token usage statistics.
type TokenStats struct {
	TotalInput    int `json:"total_input"`
	TotalOutput   int `json:"total_output"`
	CacheRead     int `json:"cache_read"`
	CacheCreation int `json:"cache_creation"`
}

// ToolCallStats represents tool call statistics.
type ToolCallStats struct {
	Total       int `json:"total"`
	UniqueTools int `json:"unique_tools"`
	Success     int `json:"success"`
	Failed      int `json:"failed"`
}

// SidechainStats represents sidechain/subagent statistics.
type SidechainStats struct {
	Count      int      `json:"count"`
	AgentTypes []string `json:"agent_types"`
}

// SessionSummary is a lightweight overview of a session.
type SessionSummary struct {
	SessionID       string          `json:"session_id"`
	AgentID         *string         `json:"agent_id"`
	Project         string          `json:"project"`
	Date            string          `json:"date"`
	DurationMinutes int             `json:"duration_minutes"`
	MessageCount    int             `json:"message_count"`
	UserMessages    int             `json:"user_messages"`
	AssistantMsgs   int             `json:"assistant_messages"`
	Tokens          *TokenStats     `json:"tokens"`
	ToolCalls       *ToolCallStats  `json:"tool_calls"`
	Sidechains      *SidechainStats `json:"sidechains"`
	HasErrors       bool            `json:"has_errors"`
	ErrorCount      int             `json:"error_count"`
}

// ToolUsageStat represents usage statistics for a single tool.
type ToolUsageStat struct {
	Name    string `json:"name"`
	Count   int    `json:"count"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// ToolPatterns represents patterns in tool usage.
type ToolPatterns struct {
	MostUsed   string `json:"most_used"`
	MostFailed string `json:"most_failed"`
	FirstTool  string `json:"first_tool"`
	LastTool   string `json:"last_tool"`
}

// ToolSequenceEntry represents a single tool in the execution sequence.
type ToolSequenceEntry struct {
	Name      string `json:"name"`
	ToolUseID string `json:"tool_use_id"`
}

// ToolUsageStats represents tool usage statistics for a session.
type ToolUsageStats struct {
	SessionID    string              `json:"session_id"`
	AgentID      *string             `json:"agent_id"`
	Tools        []ToolUsageStat     `json:"tools"`
	ToolSequence []ToolSequenceEntry `json:"tool_sequence"`
	Patterns     *ToolPatterns       `json:"patterns"`
}

// ContextLog represents a log entry surrounding an error for context.
type ContextLog struct {
	Offset       int         `json:"offset"`                  // Position relative to error (-3, -2, -1, 1, 2, 3)
	Timestamp    string      `json:"timestamp"`
	Role         string      `json:"role"`
	Content      string      `json:"content"`                 // Full content for understanding context
	ToolName     string      `json:"tool_name,omitempty"`
	ToolUseID    string      `json:"tool_use_id,omitempty"`
	ToolInput    interface{} `json:"tool_input,omitempty"`    // Raw tool input parameters
	ToolOutput   string      `json:"tool_output,omitempty"`   // Tool result/output
	IsToolResult bool        `json:"is_tool_result,omitempty"`
	IsError      bool        `json:"is_error,omitempty"`
}

// SessionError represents a single error entry.
type SessionError struct {
	UUID       string `json:"uuid"`                 // Unique ID of the error entry (use with get_logs_around_entry)
	Timestamp  string `json:"timestamp"`
	Type       string `json:"type"`
	ToolName   string `json:"tool_name,omitempty"`
	Message    string `json:"message"`
	Context    string `json:"context,omitempty"`
	Sidechain  string `json:"sidechain,omitempty"`
	EntryIndex int    `json:"entry_index"`          // Index in the session for reference
}

// ErrorCategories represents error counts by category.
type ErrorCategories struct {
	ToolError       int `json:"tool_error"`
	ConsoleError    int `json:"console_error"`
	ValidationError int `json:"validation_error"`
}

// SessionErrors represents errors found in a session.
type SessionErrors struct {
	SessionID   string           `json:"session_id"`
	AgentID     *string          `json:"agent_id"`
	TotalErrors int              `json:"total_errors"`
	Errors      []SessionError   `json:"errors"`
	Categories  *ErrorCategories `json:"categories"`
}

// TimelineEntry represents a single entry in the session timeline.
type TimelineEntry struct {
	Step      int    `json:"step"`
	Timestamp string `json:"timestamp"`
	Role      string `json:"role"`
	Type      string `json:"type"`
	Tool      string `json:"tool,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Summary   string `json:"summary"`
	Status    string `json:"status,omitempty"`
	Tokens    int    `json:"tokens,omitempty"`
	Sidechain string `json:"sidechain,omitempty"`
}

// SessionTimeline represents a condensed timeline of session events.
type SessionTimeline struct {
	SessionID       string          `json:"session_id"`
	AgentID         *string         `json:"agent_id"`
	TotalEntries    int             `json:"total_entries"`
	ReturnedEntries int             `json:"returned_entries"`
	Timeline        []TimelineEntry `json:"timeline"`
}

// LogsAroundEntry represents logs surrounding a specific entry by UUID.
type LogsAroundEntry struct {
	SessionID   string       `json:"session_id"`
	Project     string       `json:"project"`
	TargetUUID  string       `json:"target_uuid"`
	TargetIndex int          `json:"target_index"`
	Offset      int          `json:"offset"`          // The requested offset
	Entries     []ContextLog `json:"entries"`         // Logs around the target entry
	TotalCount  int          `json:"total_count"`     // Total entries in session
}

// OutputFiles represents paths to generated output files.
type OutputFiles struct {
	JSONPath      string `json:"json_path"`
	HTMLPath      string `json:"html_path,omitempty"`
	OpenedBrowser bool   `json:"opened_browser"`
}

// SessionStats is the aggregated session statistics.
type SessionStats struct {
	SessionID   string          `json:"session_id"`
	AgentID     *string         `json:"agent_id"`
	Project     string          `json:"project"`
	GeneratedAt string          `json:"generated_at"`
	Files       *OutputFiles    `json:"files,omitempty"`
	Summary     *SessionSummary `json:"summary"`
	ToolStats   *ToolUsageStats `json:"tool_stats"`
	Errors      *SessionErrors  `json:"errors"`
}
