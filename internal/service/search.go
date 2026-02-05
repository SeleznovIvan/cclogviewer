package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brads3290/cclogviewer/internal/models"
	"github.com/brads3290/cclogviewer/internal/parser"
)

// SearchService handles log searching.
type SearchService struct {
	projectService *ProjectService
	sessionService *SessionService
}

// NewSearchService creates a new SearchService.
func NewSearchService(projectService *ProjectService, sessionService *SessionService) *SearchService {
	return &SearchService{
		projectService: projectService,
		sessionService: sessionService,
	}
}

// SearchCriteria defines search parameters.
type SearchCriteria struct {
	Query             string
	ToolName          string
	Role              string
	Project           string
	Days              int
	IncludeSidechains bool
	Limit             int
}

// SearchResult represents a single search result.
type SearchResult struct {
	SessionID      string    `json:"session_id"`
	Project        string    `json:"project"`
	EntryUUID      string    `json:"entry_uuid"`
	Timestamp      time.Time `json:"timestamp"`
	Role           string    `json:"role"`
	ContentSnippet string    `json:"content_snippet"`
	ToolName       string    `json:"tool_name,omitempty"`
	IsSidechain    bool      `json:"is_sidechain,omitempty"`
}

// SearchResults represents search results.
type SearchResults struct {
	Results      []SearchResult `json:"results"`
	TotalMatches int            `json:"total_matches"`
}

// Search searches across sessions by various criteria.
func (s *SearchService) Search(criteria SearchCriteria) (*SearchResults, error) {
	var projectsToSearch []models.Project

	if criteria.Project != "" {
		project, err := s.projectService.FindProjectByName(criteria.Project)
		if err != nil {
			return nil, err
		}
		if project != nil {
			projectsToSearch = append(projectsToSearch, *project)
		}
	} else {
		projects, err := s.projectService.ListProjects("")
		if err != nil {
			return nil, err
		}
		projectsToSearch = projects
	}

	var results []SearchResult
	limit := criteria.Limit
	if limit <= 0 {
		limit = 50
	}

	for _, project := range projectsToSearch {
		if len(results) >= limit {
			break
		}

		sessions, err := s.sessionService.ListSessions(project.Name, criteria.Days, false, 0)
		if err != nil {
			continue
		}

		for _, session := range sessions {
			if len(results) >= limit {
				break
			}

			sessionResults, err := s.searchInSession(session.FilePath, session.SessionID, project.Name, criteria)
			if err != nil {
				continue
			}

			for _, r := range sessionResults {
				if len(results) >= limit {
					break
				}
				results = append(results, r)
			}
		}
	}

	return &SearchResults{
		Results:      results,
		TotalMatches: len(results),
	}, nil
}

// searchInSession searches within a single session.
func (s *SearchService) searchInSession(filePath, sessionID, project string, criteria SearchCriteria) ([]SearchResult, error) {
	entries, err := parser.ReadJSONLFile(filePath)
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	for _, entry := range entries {
		if !criteria.IncludeSidechains && entry.IsSidechain {
			continue
		}

		// Parse message
		var msg map[string]interface{}
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		role, _ := msg["role"].(string)

		// Filter by role
		if criteria.Role != "" && role != criteria.Role {
			continue
		}

		// Extract content
		content := extractContent(msg)

		// Check tool name filter
		toolName := ""
		if criteria.ToolName != "" {
			toolName = findToolName(msg, criteria.ToolName)
			if toolName == "" {
				continue
			}
		}

		// Check query match
		if criteria.Query != "" {
			if !strings.Contains(strings.ToLower(content), strings.ToLower(criteria.Query)) {
				continue
			}
		}

		timestamp, _ := time.Parse(time.RFC3339, entry.Timestamp)

		results = append(results, SearchResult{
			SessionID:      sessionID,
			Project:        project,
			EntryUUID:      entry.UUID,
			Timestamp:      timestamp,
			Role:           role,
			ContentSnippet: truncate(content, 200),
			ToolName:       toolName,
			IsSidechain:    entry.IsSidechain,
		})
	}

	return results, nil
}

// extractContent extracts text content from a message.
func extractContent(msg map[string]interface{}) string {
	content, ok := msg["content"]
	if !ok {
		return ""
	}

	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, item := range c {
			if m, ok := item.(map[string]interface{}); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		return strings.Join(parts, " ")
	}

	return ""
}

// findToolName checks if the message contains a specific tool.
func findToolName(msg map[string]interface{}, targetTool string) string {
	content, ok := msg["content"].([]interface{})
	if !ok {
		return ""
	}

	for _, item := range content {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if m["type"] != "tool_use" {
			continue
		}

		name, _ := m["name"].(string)
		if strings.EqualFold(name, targetTool) {
			return name
		}
	}

	return ""
}

// truncate truncates a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
