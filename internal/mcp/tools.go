package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/brads3290/cclogviewer/internal/models"
	"github.com/brads3290/cclogviewer/internal/service"
)

// Services holds all service dependencies for tools.
type Services struct {
	Project *service.ProjectService
	Session *service.SessionService
	Agent   *service.AgentService
	Search  *service.SearchService
}

// NewServices creates a new Services instance.
func NewServices(claudeDir string) *Services {
	projectService := service.NewProjectService(claudeDir)
	sessionService := service.NewSessionService(projectService)
	agentService := service.NewAgentService(projectService)
	searchService := service.NewSearchService(projectService, sessionService)

	return &Services{
		Project: projectService,
		Session: sessionService,
		Agent:   agentService,
		Search:  searchService,
	}
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
	return "Get full processed logs for a specific session"
}

func (t *GetSessionLogsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"session_id": {
				"type": "string",
				"description": "Session UUID"
			},
			"project": {
				"type": "string",
				"description": "Project name/path (optional if session_id is globally unique)"
			},
			"include_sidechains": {
				"type": "boolean",
				"description": "Include sidechain (agent) conversations",
				"default": true
			}
		},
		"required": ["session_id"]
	}`)
}

func (t *GetSessionLogsTool) Execute(args map[string]interface{}) (interface{}, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	project, _ := args["project"].(string)

	includeSidechains := true
	if b, ok := args["include_sidechains"].(bool); ok {
		includeSidechains = b
	}

	logs, err := t.services.Session.GetSessionLogs(sessionID, project, includeSidechains)
	if err != nil {
		return nil, fmt.Errorf("failed to get session logs: %w", err)
	}

	if logs == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
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
}

// Ensure all tools implement the Tool interface
var _ Tool = (*ListProjectsTool)(nil)
var _ Tool = (*ListSessionsTool)(nil)
var _ Tool = (*GetSessionLogsTool)(nil)
var _ Tool = (*ListAgentsTool)(nil)
var _ Tool = (*GetAgentSessionsTool)(nil)
var _ Tool = (*SearchLogsTool)(nil)
var _ Tool = (*GenerateHTMLTool)(nil)

// Suppress unused variable warning
var _ = []models.Project{}
