package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brads3290/cclogviewer/internal/browser"
	"github.com/brads3290/cclogviewer/internal/models"
	"github.com/brads3290/cclogviewer/internal/service"
)

// SaveResult represents the result of saving data to a file.
type SaveResult struct {
	FilePath string      `json:"file_path"`
	Data     interface{} `json:"data"`
}

// saveToFile saves data as JSON to the specified path, creating directories if needed.
func saveToFile(data interface{}, outputPath string) (*SaveResult, error) {
	// Create parent directories if they don't exist
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &SaveResult{
		FilePath: outputPath,
		Data:     data,
	}, nil
}

// Services is an alias for service.Services for backward compatibility.
type Services = service.Services

// NewServices creates a new Services instance.
// This is a convenience wrapper for service.NewServices.
func NewServices(claudeDir string) *Services {
	return service.NewServices(claudeDir)
}

// ListProjectsTool implements the list_projects tool.
type ListProjectsTool struct {
	services *Services
}

func NewListProjectsTool(services *Services) *ListProjectsTool {
	return &ListProjectsTool{services: services}
}

func (t *ListProjectsTool) Name() string {
	return "list_projects"
}

func (t *ListProjectsTool) Description() string {
	return "List all Claude Code projects with session counts and metadata"
}

func (t *ListProjectsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"sort_by": {
				"type": "string",
				"enum": ["last_modified", "name", "session_count"],
				"description": "Sort projects by this field",
				"default": "last_modified"
			}
		}
	}`)
}

func (t *ListProjectsTool) Execute(args map[string]interface{}) (interface{}, error) {
	sortBy, _ := args["sort_by"].(string)
	if sortBy == "" {
		sortBy = "last_modified"
	}

	projects, err := t.services.Project.ListProjects(sortBy)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	return map[string]interface{}{
		"projects": projects,
		"total":    len(projects),
	}, nil
}

// ListSessionsTool implements the list_sessions tool.
type ListSessionsTool struct {
	services *Services
}

func NewListSessionsTool(services *Services) *ListSessionsTool {
	return &ListSessionsTool{services: services}
}

func (t *ListSessionsTool) Name() string {
	return "list_sessions"
}

func (t *ListSessionsTool) Description() string {
	return "List sessions for a project with optional time filtering and agent type extraction"
}

func (t *ListSessionsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"project": {
				"type": "string",
				"description": "Project name or path (can be partial match)"
			},
			"days": {
				"type": "integer",
				"description": "Only include sessions from the last N days",
				"minimum": 1
			},
			"include_agent_types": {
				"type": "boolean",
				"description": "Extract and include subagent_types used in each session",
				"default": false
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of sessions to return",
				"default": 50
			}
		},
		"required": ["project"]
	}`)
}

func (t *ListSessionsTool) Execute(args map[string]interface{}) (interface{}, error) {
	project, _ := args["project"].(string)
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	days := 0
	if d, ok := args["days"].(float64); ok {
		days = int(d)
	}

	includeAgentTypes := false
	if b, ok := args["include_agent_types"].(bool); ok {
		includeAgentTypes = b
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	sessions, err := t.services.Session.ListSessions(project, days, includeAgentTypes, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return map[string]interface{}{
		"project":  project,
		"sessions": sessions,
		"count":    len(sessions),
	}, nil
}

// GetSessionLogsTool implements the get_session_logs tool.
type GetSessionLogsTool struct {
	services *Services
}

func NewGetSessionLogsTool(services *Services) *GetSessionLogsTool {
	return &GetSessionLogsTool{services: services}
}

func (t *GetSessionLogsTool) Name() string {
	return "get_session_logs"
}

func (t *GetSessionLogsTool) Description() string {
	return "Get full processed logs for a specific session. Accepts either a session_id or a direct file_path to a JSONL file."
}

func (t *GetSessionLogsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID (use this OR file_path)"
			},
			"file_path": {
				"type": "string",
				"description": "Direct path to a JSONL log file (use this OR session_id)"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional, only used with session_id)"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Include sidechain (agent) conversations",
				"default": true
			},
			"output_path": {
				"type": "string",
				"description": "File path to save the logs as JSON. If provided, creates parent directories automatically."
			}
		}
	}`)
}

func (t *GetSessionLogsTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID := getString(args, "session_id")
	filePath := getString(args, "file_path")

	if sessionID == "" && filePath == "" {
		return nil, fmt.Errorf("either session_id or file_path is required")
	}

	includeSidechains := getBool(args, "include_sidechains", true)

	var logs *models.SessionLogs
	var err error

	if filePath != "" {
		logs, err = t.services.Session.GetSessionLogsFromFile(filePath, includeSidechains)
	} else {
		project := getString(args, "project")
		logs, err = t.services.Session.GetSessionLogs(sessionID, project, includeSidechains)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get session logs: %w", err)
	}

	if logs == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Save to file if output_path is provided
	outputPath := getString(args, "output_path")
	if outputPath != "" {
		return saveToFile(logs, outputPath)
	}

	return logs, nil
}

// ListAgentsTool implements the list_agents tool.
type ListAgentsTool struct {
	services *Services
}

func NewListAgentsTool(services *Services) *ListAgentsTool {
	return &ListAgentsTool{services: services}
}

func (t *ListAgentsTool) Name() string {
	return "list_agents"
}

func (t *ListAgentsTool) Description() string {
	return "List available agent definitions (global and project-specific)"
}

func (t *ListAgentsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"project": {
				"type": "string",
				"description": "Project path to include project-specific agents"
			},
			"include_global": {
				"type": "boolean",
				"description": "Include global agents from ~/.claude/agents/",
				"default": true
			}
		}
	}`)
}

func (t *ListAgentsTool) Execute(args map[string]interface{}) (interface{}, error) {
	projectPath, _ := args["project"].(string)

	includeGlobal := true
	if b, ok := args["include_global"].(bool); ok {
		includeGlobal = b
	}

	// If project name given, resolve to path
	if projectPath != "" {
		project, err := t.services.Project.FindProjectByName(projectPath)
		if err == nil && project != nil {
			projectPath = project.Path
		}
	}

	agents, err := t.services.Agent.ListAgents(projectPath, includeGlobal)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	}, nil
}

// GetAgentSessionsTool implements the get_agent_sessions tool.
type GetAgentSessionsTool struct {
	services *Services
}

func NewGetAgentSessionsTool(services *Services) *GetAgentSessionsTool {
	return &GetAgentSessionsTool{services: services}
}

func (t *GetAgentSessionsTool) Name() string {
	return "get_agent_sessions"
}

func (t *GetAgentSessionsTool) Description() string {
	return "Find sessions where a specific agent/subagent type was used"
}

func (t *GetAgentSessionsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"agent_type": {
				"type": "string",
				"description": "Agent/subagent type (e.g., 'Explore', 'Plan', 'flutter-coder')"
			},
			"project": {
				"type": "string",
				"description": "Limit search to a specific project"
			},
			"days": {
				"type": "integer",
				"description": "Only search sessions from the last N days"
			},
			"limit": {
				"type": "integer",
				"description": "Maximum sessions to return",
				"default": 20
			}
		},
		"required": ["agent_type"]
	}`)
}

func (t *GetAgentSessionsTool) Execute(args map[string]interface{}) (interface{}, error) {
	agentType, _ := args["agent_type"].(string)
	if agentType == "" {
		return nil, fmt.Errorf("agent_type is required")
	}

	project, _ := args["project"].(string)

	days := 0
	if d, ok := args["days"].(float64); ok {
		days = int(d)
	}

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	sessions, err := t.services.Session.FindSessionsByAgentType(agentType, project, days, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent sessions: %w", err)
	}

	return map[string]interface{}{
		"agent_type": agentType,
		"sessions":   sessions,
		"count":      len(sessions),
	}, nil
}

// SearchLogsTool implements the search_logs tool.
type SearchLogsTool struct {
	services *Services
}

func NewSearchLogsTool(services *Services) *SearchLogsTool {
	return &SearchLogsTool{services: services}
}

func (t *SearchLogsTool) Name() string {
	return "search_logs"
}

func (t *SearchLogsTool) Description() string {
	return "Search across sessions by content, tool usage, or other criteria"
}

func (t *SearchLogsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Text to search for in log content"
			},
			"tool_name": {
				"type": "string",
				"description": "Filter by tool name (e.g., 'Bash', 'Edit')"
			},
			"role": {
				"type": "string",
				"enum": ["user", "assistant"],
				"description": "Filter by message role"
			},
			"project": {
				"type": "string",
				"description": "Limit search to a specific project"
			},
			"days": {
				"type": "integer",
				"description": "Only search sessions from the last N days"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Search in sidechain conversations too",
				"default": true
			},
			"limit": {
				"type": "integer",
				"description": "Maximum results to return",
				"default": 50
			}
		}
	}`)
}

func (t *SearchLogsTool) Execute(args map[string]interface{}) (interface{}, error) {
	criteria := service.SearchCriteria{
		Query:             getString(args, "query"),
		ToolName:          getString(args, "tool_name"),
		Role:              getString(args, "role"),
		Project:           getString(args, "project"),
		Days:              getInt(args, "days"),
		IncludeSidechains: getBool(args, "include_sidechains", true),
		Limit:             getInt(args, "limit"),
	}

	if criteria.Limit == 0 {
		criteria.Limit = 50
	}

	results, err := t.services.Search.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return results, nil
}

// GenerateHTMLTool implements the generate_html tool.
type GenerateHTMLTool struct {
	services *Services
}

func NewGenerateHTMLTool(services *Services) *GenerateHTMLTool {
	return &GenerateHTMLTool{services: services}
}

func (t *GenerateHTMLTool) Name() string {
	return "generate_html"
}

func (t *GenerateHTMLTool) Description() string {
	return "Generate an interactive HTML file from session logs. Accepts either a session_id or a direct file_path to a JSONL file. If no output path is specified, creates a temporary file and opens it in the browser."
}

func (t *GenerateHTMLTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID (use this OR file_path)"
			},
			"file_path": {
				"type": "string",
				"description": "Direct path to a JSONL log file (use this OR session_id)"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional, only used with session_id)"
			},
			"output_path": {
				"type": "string",
				"description": "Output HTML file path (optional, creates temp file if not specified)"
			},
			"open_browser": {
				"type": "boolean",
				"description": "Open the generated HTML file in browser (default: true when output_path not specified)",
				"default": false
			}
		}
	}`)
}

func (t *GenerateHTMLTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID, _ := args["session_id"].(string)
	filePath, _ := args["file_path"].(string)

	if sessionID == "" && filePath == "" {
		return nil, fmt.Errorf("either session_id or file_path is required")
	}

	outputPath, _ := args["output_path"].(string)

	openBrowser := false
	if b, ok := args["open_browser"].(bool); ok {
		openBrowser = b
	}

	// If file_path is provided, use it directly
	if filePath != "" {
		result, err := t.services.Session.GenerateHTMLFromFile(filePath, outputPath, openBrowser)
		if err != nil {
			return nil, fmt.Errorf("failed to generate HTML: %w", err)
		}
		return result, nil
	}

	// Otherwise use session_id lookup
	project, _ := args["project"].(string)
	result, err := t.services.Session.GenerateSessionHTML(sessionID, project, outputPath, openBrowser)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HTML: %w", err)
	}

	return result, nil
}

// GetSessionSummaryTool implements the get_session_summary tool.
type GetSessionSummaryTool struct {
	services *Services
}

func NewGetSessionSummaryTool(services *Services) *GetSessionSummaryTool {
	return &GetSessionSummaryTool{services: services}
}

func (t *GetSessionSummaryTool) Name() string {
	return "get_session_summary"
}

func (t *GetSessionSummaryTool) Description() string {
	return "Get a lightweight summary of a session including message counts, token usage, tool statistics, and error counts. Accepts either a session_id or a direct file_path to a JSONL file."
}

func (t *GetSessionSummaryTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID (use this OR file_path)"
			},
			"file_path": {
				"type": "string",
				"description": "Direct path to a JSONL log file (use this OR session_id)"
			},
			"agent_id": {
				"type": "string",
				"description": "Specific subagent ID to analyze (optional, only used with session_id)"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional, only used with session_id)"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Include sidechain (agent) conversations in analysis",
				"default": true
			},
			"output_path": {
				"type": "string",
				"description": "File path to save the summary as JSON. If provided, creates parent directories automatically."
			}
		}
	}`)
}

func (t *GetSessionSummaryTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID := getString(args, "session_id")
	filePath := getString(args, "file_path")

	if sessionID == "" && filePath == "" {
		return nil, fmt.Errorf("either session_id or file_path is required")
	}

	includeSidechains := getBool(args, "include_sidechains", true)

	var summary *models.SessionSummary
	var err error

	if filePath != "" {
		summary, err = t.services.Session.GetSessionSummaryFromFile(filePath, includeSidechains)
	} else {
		agentID := getString(args, "agent_id")
		project := getString(args, "project")
		summary, err = t.services.Session.GetSessionSummary(sessionID, agentID, project, includeSidechains)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get session summary: %w", err)
	}

	if summary == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Save to file if output_path is provided
	outputPath := getString(args, "output_path")
	if outputPath != "" {
		return saveToFile(summary, outputPath)
	}

	return summary, nil
}

// GetToolUsageStatsTool implements the get_tool_usage_stats tool.
type GetToolUsageStatsTool struct {
	services *Services
}

func NewGetToolUsageStatsTool(services *Services) *GetToolUsageStatsTool {
	return &GetToolUsageStatsTool{services: services}
}

func (t *GetToolUsageStatsTool) Name() string {
	return "get_tool_usage_stats"
}

func (t *GetToolUsageStatsTool) Description() string {
	return "Get tool usage statistics for a session including tool counts, success/failure rates, and usage patterns. Accepts either a session_id or a direct file_path to a JSONL file."
}

func (t *GetToolUsageStatsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID (use this OR file_path)"
			},
			"file_path": {
				"type": "string",
				"description": "Direct path to a JSONL log file (use this OR session_id)"
			},
			"agent_id": {
				"type": "string",
				"description": "Specific subagent ID to analyze (optional, only used with session_id)"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional, only used with session_id)"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Include sidechain (agent) conversations in analysis",
				"default": true
			},
			"output_path": {
				"type": "string",
				"description": "File path to save the stats as JSON. If provided, creates parent directories automatically."
			}
		}
	}`)
}

func (t *GetToolUsageStatsTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID := getString(args, "session_id")
	filePath := getString(args, "file_path")

	if sessionID == "" && filePath == "" {
		return nil, fmt.Errorf("either session_id or file_path is required")
	}

	includeSidechains := getBool(args, "include_sidechains", true)

	var stats *models.ToolUsageStats
	var err error

	if filePath != "" {
		stats, err = t.services.Session.GetToolUsageStatsFromFile(filePath, includeSidechains)
	} else {
		agentID := getString(args, "agent_id")
		project := getString(args, "project")
		stats, err = t.services.Session.GetToolUsageStats(sessionID, agentID, project, includeSidechains)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get tool usage stats: %w", err)
	}

	if stats == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Save to file if output_path is provided
	outputPath := getString(args, "output_path")
	if outputPath != "" {
		return saveToFile(stats, outputPath)
	}

	return stats, nil
}

// GetSessionErrorsTool implements the get_session_errors tool.
type GetSessionErrorsTool struct {
	services *Services
}

func NewGetSessionErrorsTool(services *Services) *GetSessionErrorsTool {
	return &GetSessionErrorsTool{services: services}
}

func (t *GetSessionErrorsTool) Name() string {
	return "get_session_errors"
}

func (t *GetSessionErrorsTool) Description() string {
	return "Get errors and blockers found in a session for debugging and analysis. Accepts either a session_id or a direct file_path to a JSONL file."
}

func (t *GetSessionErrorsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID (use this OR file_path)"
			},
			"file_path": {
				"type": "string",
				"description": "Direct path to a JSONL log file (use this OR session_id)"
			},
			"agent_id": {
				"type": "string",
				"description": "Specific subagent ID to analyze (optional, only used with session_id)"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional, only used with session_id)"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Include sidechain (agent) conversations in analysis",
				"default": true
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of errors to return",
				"default": 20
			},
			"output_path": {
				"type": "string",
				"description": "File path to save the errors as JSON. If provided, creates parent directories automatically."
			}
		}
	}`)
}

func (t *GetSessionErrorsTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID := getString(args, "session_id")
	filePath := getString(args, "file_path")

	if sessionID == "" && filePath == "" {
		return nil, fmt.Errorf("either session_id or file_path is required")
	}

	includeSidechains := getBool(args, "include_sidechains", true)
	limit := getInt(args, "limit")
	if limit == 0 {
		limit = 20
	}

	var errors *models.SessionErrors
	var err error

	if filePath != "" {
		errors, err = t.services.Session.GetSessionErrorsFromFile(filePath, includeSidechains, limit)
	} else {
		agentID := getString(args, "agent_id")
		project := getString(args, "project")
		errors, err = t.services.Session.GetSessionErrors(sessionID, agentID, project, includeSidechains, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get session errors: %w", err)
	}

	if errors == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Save to file if output_path is provided
	outputPath := getString(args, "output_path")
	if outputPath != "" {
		return saveToFile(errors, outputPath)
	}

	return errors, nil
}

// GetSessionTimelineTool implements the get_session_timeline tool.
type GetSessionTimelineTool struct {
	services *Services
}

func NewGetSessionTimelineTool(services *Services) *GetSessionTimelineTool {
	return &GetSessionTimelineTool{services: services}
}

func (t *GetSessionTimelineTool) Name() string {
	return "get_session_timeline"
}

func (t *GetSessionTimelineTool) Description() string {
	return "Get a condensed timeline of session events without full content. Shows step-by-step progression. Accepts either a session_id or a direct file_path to a JSONL file."
}

func (t *GetSessionTimelineTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID (use this OR file_path)"
			},
			"file_path": {
				"type": "string",
				"description": "Direct path to a JSONL log file (use this OR session_id)"
			},
			"agent_id": {
				"type": "string",
				"description": "Specific subagent ID to analyze (optional, only used with session_id)"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional, only used with session_id)"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Include sidechain (agent) conversations in analysis",
				"default": true
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of timeline entries to return",
				"default": 100
			},
			"output_path": {
				"type": "string",
				"description": "File path to save the timeline as JSON. If provided, creates parent directories automatically."
			}
		}
	}`)
}

func (t *GetSessionTimelineTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID := getString(args, "session_id")
	filePath := getString(args, "file_path")

	if sessionID == "" && filePath == "" {
		return nil, fmt.Errorf("either session_id or file_path is required")
	}

	includeSidechains := getBool(args, "include_sidechains", true)
	limit := getInt(args, "limit")
	if limit == 0 {
		limit = 100
	}

	var timeline *models.SessionTimeline
	var err error

	if filePath != "" {
		timeline, err = t.services.Session.GetSessionTimelineFromFile(filePath, includeSidechains, limit)
	} else {
		agentID := getString(args, "agent_id")
		project := getString(args, "project")
		timeline, err = t.services.Session.GetSessionTimeline(sessionID, agentID, project, includeSidechains, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get session timeline: %w", err)
	}

	if timeline == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Save to file if output_path is provided
	outputPath := getString(args, "output_path")
	if outputPath != "" {
		return saveToFile(timeline, outputPath)
	}

	return timeline, nil
}

// GetSessionStatsTool implements the get_session_stats tool.
type GetSessionStatsTool struct {
	services *Services
}

func NewGetSessionStatsTool(services *Services) *GetSessionStatsTool {
	return &GetSessionStatsTool{services: services}
}

func (t *GetSessionStatsTool) Name() string {
	return "get_session_stats"
}

func (t *GetSessionStatsTool) Description() string {
	return "Get comprehensive session statistics combining summary, tool usage, and errors. Optionally generates HTML visualization. Accepts either a session_id or a direct file_path to a JSONL file."
}

func (t *GetSessionStatsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID (use this OR file_path)"
			},
			"file_path": {
				"type": "string",
				"description": "Direct path to a JSONL log file (use this OR session_id)"
			},
			"agent_id": {
				"type": "string",
				"description": "Specific subagent ID to analyze (optional, only used with session_id)"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional, only used with session_id)"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Include sidechain (agent) conversations in analysis",
				"default": true
			},
			"errors_limit": {
				"type": "integer",
				"description": "Maximum errors to include",
				"default": 10
			},
			"output_path": {
				"type": "string",
				"description": "Base path for output files (without extension). If not specified, uses temp directory."
			},
			"generate_html": {
				"type": "boolean",
				"description": "Generate HTML visualization alongside JSON",
				"default": false
			},
			"open_browser": {
				"type": "boolean",
				"description": "Open HTML in browser (requires generate_html=true)",
				"default": false
			}
		}
	}`)
}

func (t *GetSessionStatsTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID := getString(args, "session_id")
	filePath := getString(args, "file_path")

	if sessionID == "" && filePath == "" {
		return nil, fmt.Errorf("either session_id or file_path is required")
	}

	includeSidechains := getBool(args, "include_sidechains", true)
	errorsLimit := getInt(args, "errors_limit")
	if errorsLimit == 0 {
		errorsLimit = 10
	}

	var stats *models.SessionStats
	var err error

	if filePath != "" {
		stats, err = t.services.Session.GetSessionStatsFromFile(filePath, includeSidechains, errorsLimit)
	} else {
		agentID := getString(args, "agent_id")
		project := getString(args, "project")
		stats, err = t.services.Session.GetSessionStats(sessionID, agentID, project, includeSidechains, errorsLimit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get session stats: %w", err)
	}

	if stats == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Handle file output and HTML generation
	outputPath := getString(args, "output_path")
	generateHTML := getBool(args, "generate_html", false)
	openBrowser := getBool(args, "open_browser", false)

	if outputPath != "" || generateHTML {
		files, err := t.saveStatsFiles(stats, outputPath, generateHTML, openBrowser)
		if err != nil {
			return nil, fmt.Errorf("failed to save stats files: %w", err)
		}
		stats.Files = files
	}

	return stats, nil
}

// saveStatsFiles saves stats to JSON and optionally HTML files.
func (t *GetSessionStatsTool) saveStatsFiles(stats *models.SessionStats, outputPath string, generateHTML, openBrowser bool) (*models.OutputFiles, error) {
	// Determine output path
	if outputPath == "" {
		outputPath = fmt.Sprintf("/tmp/session-stats-%s", stats.SessionID[:8])
	}

	files := &models.OutputFiles{}

	// Save JSON
	jsonPath := outputPath + ".json"
	jsonData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}

	if err := writeFile(jsonPath, jsonData); err != nil {
		return nil, fmt.Errorf("failed to write JSON: %w", err)
	}
	files.JSONPath = jsonPath

	// Generate HTML if requested
	if generateHTML {
		htmlPath := outputPath + ".html"
		htmlContent := generateStatsHTML(stats)

		if err := writeFile(htmlPath, []byte(htmlContent)); err != nil {
			return nil, fmt.Errorf("failed to write HTML: %w", err)
		}
		files.HTMLPath = htmlPath

		// Open browser if requested
		if openBrowser {
			if err := openInBrowser(htmlPath); err == nil {
				files.OpenedBrowser = true
			}
		}
	}

	return files, nil
}

// generateStatsHTML generates HTML visualization of stats.
func generateStatsHTML(stats *models.SessionStats) string {
	// Simple HTML template for stats visualization
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + stats.Project + ` - Session Stats</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
    <style>
        .stat-card { margin-bottom: 1rem; }
        .timeline-item { padding: 0.5rem; border-left: 3px solid #3273dc; margin-left: 1rem; }
        .timeline-item.error { border-left-color: #f14668; }
        .tool-bar { height: 20px; background: #3273dc; margin: 2px 0; }
        pre { background: #f5f5f5; padding: 1rem; overflow-x: auto; font-size: 0.85em; }
        .error-entry-index { font-size: 0.85em; color: #666; margin-left: 0.5rem; }
        .header-project { font-size: 0.6em; color: #7a7a7a; font-weight: normal; }
    </style>
</head>
<body>
<section class="section">
    <div class="container">
        <h1 class="title">
            ` + stats.Project + `
            <span class="header-project">Session Statistics</span>
        </h1>
        <p class="subtitle">
            <span class="tag is-medium is-light">` + stats.SessionID + `</span>
            <span class="tag is-medium is-info is-light">` + stats.Summary.Date + `</span>
        </p>

        <div class="columns">
            <div class="column">
                <div class="box stat-card">
                    <h3 class="title is-5">Summary</h3>
                    <table class="table is-fullwidth">
                        <tr><td>Project</td><td><strong>` + stats.Project + `</strong></td></tr>
                        <tr><td>Date</td><td>` + stats.Summary.Date + `</td></tr>
                        <tr><td>Duration</td><td>` + fmt.Sprintf("%d", stats.Summary.DurationMinutes) + ` minutes</td></tr>
                        <tr><td>Messages</td><td>` + fmt.Sprintf("%d", stats.Summary.MessageCount) + ` (` + fmt.Sprintf("%d", stats.Summary.UserMessages) + ` user, ` + fmt.Sprintf("%d", stats.Summary.AssistantMsgs) + ` assistant)</td></tr>
                        <tr><td>Errors</td><td>` + fmt.Sprintf("%d", stats.Summary.ErrorCount) + `</td></tr>
                    </table>
                </div>
            </div>
            <div class="column">
                <div class="box stat-card">
                    <h3 class="title is-5">Tokens</h3>
                    <table class="table is-fullwidth">
                        <tr><td>Input</td><td>` + fmt.Sprintf("%d", stats.Summary.Tokens.TotalInput) + `</td></tr>
                        <tr><td>Output</td><td>` + fmt.Sprintf("%d", stats.Summary.Tokens.TotalOutput) + `</td></tr>
                        <tr><td>Cache Read</td><td>` + fmt.Sprintf("%d", stats.Summary.Tokens.CacheRead) + `</td></tr>
                        <tr><td>Cache Creation</td><td>` + fmt.Sprintf("%d", stats.Summary.Tokens.CacheCreation) + `</td></tr>
                    </table>
                </div>
            </div>
        </div>

        <div class="box">
            <h3 class="title is-5">Tool Usage</h3>
            <table class="table is-fullwidth">
                <thead>
                    <tr><th>Tool</th><th>Count</th><th>Success</th><th>Failed</th></tr>
                </thead>
                <tbody>`

	for _, tool := range stats.ToolStats.Tools {
		html += fmt.Sprintf(`
                    <tr>
                        <td>%s</td>
                        <td>%d</td>
                        <td class="has-text-success">%d</td>
                        <td class="has-text-danger">%d</td>
                    </tr>`, tool.Name, tool.Count, tool.Success, tool.Failed)
	}

	html += `
                </tbody>
            </table>
        </div>

        <div class="box">
            <h3 class="title is-5">Tool Sequence</h3>
            <div class="tool-sequence" style="display: flex; flex-wrap: wrap; gap: 4px;">`

	for _, entry := range stats.ToolStats.ToolSequence {
		html += fmt.Sprintf(`<span class="tag is-info" title="%s">%s</span>`, entry.ToolUseID, entry.Name)
	}

	html += `
            </div>
        </div>

        <div class="box">
            <h3 class="title is-5">Errors (` + fmt.Sprintf("%d", stats.Errors.TotalErrors) + ` total)</h3>`

	if len(stats.Errors.Errors) == 0 {
		html += `<p class="has-text-success">No errors found</p>`
	} else {
		for _, err := range stats.Errors.Errors {
			toolInfo := ""
			if err.ToolName != "" {
				toolInfo = " - " + err.ToolName
			}
			html += fmt.Sprintf(`
            <article class="message is-danger">
                <div class="message-header">
                    <p>%s%s <span class="error-entry-index">(entry #%d)</span></p>
                    <span>%s</span>
                </div>
                <div class="message-body">
                    <div class="error-message">%s</div>
                    <p class="mt-2"><code class="has-text-grey">UUID: %s</code></p>
                    <p class="is-size-7 has-text-grey">Use get_logs_around_entry(uuid: "%s", offset: 3) to see context</p>
                </div>
            </article>`, err.Type, toolInfo, err.EntryIndex, err.Timestamp, escapeHTML(err.Message), err.UUID, err.UUID)
		}
	}

	html += `
        </div>

        <div class="box">
            <h3 class="title is-5">Raw JSON</h3>
            <pre>` + escapeHTML(mustMarshalIndent(stats)) + `</pre>
        </div>
    </div>
</section>
</body>
</html>`

	return html
}

// escapeHTML escapes HTML special characters.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// mustMarshalIndent marshals to indented JSON, returning empty on error.
func mustMarshalIndent(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// writeFile writes data to a file.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// openInBrowser opens a file in the default browser.
func openInBrowser(path string) error {
	return browser.OpenInBrowser(path)
}

// GetLogsAroundEntryTool implements the get_logs_around_entry tool.
type GetLogsAroundEntryTool struct {
	services *Services
}

func NewGetLogsAroundEntryTool(services *Services) *GetLogsAroundEntryTool {
	return &GetLogsAroundEntryTool{services: services}
}

func (t *GetLogsAroundEntryTool) Name() string {
	return "get_logs_around_entry"
}

func (t *GetLogsAroundEntryTool) Description() string {
	return "Get logs around a specific entry identified by UUID. Use negative offset for entries BEFORE (e.g., -3 for 3 prior entries + target), positive offset for entries AFTER (e.g., +3 for target + 3 following entries). Accepts either a session_id or a direct file_path to a JSONL file."
}

func (t *GetLogsAroundEntryTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID (use this OR file_path)"
			},
			"file_path": {
				"type": "string",
				"description": "Direct path to a JSONL log file (use this OR session_id)"
			},
			"uuid": {
				"type": "string",
				"description": "UUID of the target log entry"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional, only used with session_id)"
			},
			"offset": {
				"type": "integer",
				"description": "Direction and count: negative = entries BEFORE target (e.g., -3), positive = entries AFTER target (e.g., +3). Default: -3"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Include sidechain (agent) conversations",
				"default": true
			},
			"output_path": {
				"type": "string",
				"description": "File path to save the logs as JSON. If provided, creates parent directories automatically."
			}
		},
		"required": ["uuid"]
	}`)
}

func (t *GetLogsAroundEntryTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID := getString(args, "session_id")
	filePath := getString(args, "file_path")

	if sessionID == "" && filePath == "" {
		return nil, fmt.Errorf("either session_id or file_path is required")
	}

	targetUUID := getString(args, "uuid")
	if targetUUID == "" {
		return nil, fmt.Errorf("uuid is required")
	}

	offset := getInt(args, "offset")
	if offset == 0 {
		offset = 3
	}
	includeSidechains := getBool(args, "include_sidechains", true)

	var logs *models.LogsAroundEntry
	var err error

	if filePath != "" {
		logs, err = t.services.Session.GetLogsAroundEntryFromFile(filePath, targetUUID, offset, includeSidechains)
	} else {
		project := getString(args, "project")
		logs, err = t.services.Session.GetLogsAroundEntry(sessionID, targetUUID, project, offset, includeSidechains)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get logs around entry: %w", err)
	}

	if logs == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Save to file if output_path is provided
	outputPath := getString(args, "output_path")
	if outputPath != "" {
		return saveToFile(logs, outputPath)
	}

	return logs, nil
}

// Helper functions for argument extraction
func getString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getInt(args map[string]interface{}, key string) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return 0
}

func getBool(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultVal
}

// RegisterAllTools registers all MCP tools with the server.
func RegisterAllTools(server *Server, services *Services) {
	server.RegisterTool(NewListProjectsTool(services))
	server.RegisterTool(NewListSessionsTool(services))
	server.RegisterTool(NewGetSessionLogsTool(services))
	server.RegisterTool(NewListAgentsTool(services))
	server.RegisterTool(NewGetAgentSessionsTool(services))
	server.RegisterTool(NewSearchLogsTool(services))
	server.RegisterTool(NewGenerateHTMLTool(services))

	// Session stats tools
	server.RegisterTool(NewGetSessionSummaryTool(services))
	server.RegisterTool(NewGetToolUsageStatsTool(services))
	server.RegisterTool(NewGetSessionErrorsTool(services))
	server.RegisterTool(NewGetSessionTimelineTool(services))
	server.RegisterTool(NewGetSessionStatsTool(services))

	// Log exploration tools
	server.RegisterTool(NewGetLogsAroundEntryTool(services))
}

// Ensure all tools implement the Tool interface
var _ Tool = (*ListProjectsTool)(nil)
var _ Tool = (*ListSessionsTool)(nil)
var _ Tool = (*GetSessionLogsTool)(nil)
var _ Tool = (*ListAgentsTool)(nil)
var _ Tool = (*GetAgentSessionsTool)(nil)
var _ Tool = (*SearchLogsTool)(nil)
var _ Tool = (*GenerateHTMLTool)(nil)
var _ Tool = (*GetSessionSummaryTool)(nil)
var _ Tool = (*GetToolUsageStatsTool)(nil)
var _ Tool = (*GetSessionErrorsTool)(nil)
var _ Tool = (*GetSessionTimelineTool)(nil)
var _ Tool = (*GetSessionStatsTool)(nil)
var _ Tool = (*GetLogsAroundEntryTool)(nil)

// Suppress unused variable warning
var _ = []models.Project{}
