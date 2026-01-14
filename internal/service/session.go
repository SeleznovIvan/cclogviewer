package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/brads3290/cclogviewer/internal/browser"
	"github.com/brads3290/cclogviewer/internal/constants"
	"github.com/brads3290/cclogviewer/internal/models"
	"github.com/brads3290/cclogviewer/internal/parser"
	"github.com/brads3290/cclogviewer/internal/processor"
	"github.com/brads3290/cclogviewer/internal/renderer"
)

// SessionService handles session listing and retrieval.
type SessionService struct {
	projectService *ProjectService
}

// NewSessionService creates a new SessionService.
func NewSessionService(projectService *ProjectService) *SessionService {
	return &SessionService{projectService: projectService}
}

// ListSessions returns sessions for a project with optional filtering.
func (s *SessionService) ListSessions(projectName string, days int, includeAgentTypes bool, limit int) ([]models.SessionInfo, error) {
	project, err := s.projectService.FindProjectByName(projectName)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, nil
	}

	projectDir := s.projectService.GetProjectDir(project.EncodedPath)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	// Calculate cutoff time
	var cutoff time.Time
	if days > 0 {
		cutoff = time.Now().AddDate(0, 0, -days)
	}

	uuidPattern := regexp.MustCompile(`^([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.jsonl$`)

	var sessions []models.SessionInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := uuidPattern.FindStringSubmatch(entry.Name())
		if len(matches) != 2 {
			continue
		}

		sessionID := matches[1]
		filePath := filepath.Join(projectDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Filter by time
		if days > 0 && info.ModTime().Before(cutoff) {
			continue
		}

		sessionInfo, err := s.getSessionInfo(filePath, sessionID, project.Name, includeAgentTypes)
		if err != nil {
			continue
		}

		sessions = append(sessions, *sessionInfo)
	}

	// Sort by start time descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	// Apply limit
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

// GetSessionLogs retrieves full processed logs for a session.
func (s *SessionService) GetSessionLogs(sessionID, projectName string, includeSidechains bool) (*models.SessionLogs, error) {
	filePath, project, err := s.findSessionFile(sessionID, projectName)
	if err != nil {
		return nil, err
	}
	if filePath == "" {
		return nil, nil
	}

	// Use existing parser
	entries, err := parser.ReadJSONLFile(filePath)
	if err != nil {
		return nil, err
	}

	// Use existing processor
	processed := processor.ProcessEntries(entries)

	// Convert to session logs format
	logs := &models.SessionLogs{
		SessionID: sessionID,
		Project:   project,
		Entries:   make([]models.SessionLogEntry, 0),
	}

	var totalInput, totalOutput, cacheRead, cacheCreation int

	for _, entry := range processed {
		if !includeSidechains && entry.IsSidechain {
			continue
		}

		logEntry := models.SessionLogEntry{
			UUID:        entry.UUID,
			Timestamp:   entry.Timestamp,
			Role:        entry.Role,
			Content:     entry.Content,
			IsSidechain: entry.IsSidechain,
			AgentID:     entry.AgentID,
		}

		// Add tool calls
		for _, tc := range entry.ToolCalls {
			logEntry.ToolCalls = append(logEntry.ToolCalls, models.SessionToolCall{
				Name:  tc.Name,
				Input: tc.RawInput,
			})
		}

		logs.Entries = append(logs.Entries, logEntry)

		// Accumulate token stats
		totalInput += entry.InputTokens
		totalOutput += entry.OutputTokens
		cacheRead += entry.CacheReadTokens
		cacheCreation += entry.CacheCreationTokens
	}

	logs.TokenStats = &models.SessionTokenStats{
		TotalInput:    totalInput,
		TotalOutput:   totalOutput,
		CacheRead:     cacheRead,
		CacheCreation: cacheCreation,
	}

	return logs, nil
}

// HTMLGenerationResult contains the result of HTML generation.
type HTMLGenerationResult struct {
	OutputPath    string `json:"output_path"`
	SessionID     string `json:"session_id"`
	Project       string `json:"project"`
	OpenedBrowser bool   `json:"opened_browser"`
}

// GenerateSessionHTML generates an HTML file from a session's logs.
// If outputPath is empty, a temporary file is created and auto-opened in the browser.
// If openBrowser is true, the HTML file is opened in the default browser.
func (s *SessionService) GenerateSessionHTML(sessionID, projectName, outputPath string, openBrowser bool) (*HTMLGenerationResult, error) {
	// Find the session file
	filePath, project, err := s.findSessionFile(sessionID, projectName)
	if err != nil {
		return nil, err
	}
	if filePath == "" {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Parse the JSONL file
	entries, err := parser.ReadJSONLFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Process entries
	processed := processor.ProcessEntries(entries)

	// Determine output path
	autoOpen := false
	if outputPath == "" {
		// Generate unique filename based on session ID and timestamp
		timestamp := time.Now().Format(constants.TempFileTimestampFormat)
		outputPath = filepath.Join(os.TempDir(), fmt.Sprintf(constants.TempFileNameFormat, sessionID[:8], timestamp))
		autoOpen = true
	}

	// Generate HTML
	err = renderer.GenerateHTML(processed, outputPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HTML: %w", err)
	}

	result := &HTMLGenerationResult{
		OutputPath:    outputPath,
		SessionID:     sessionID,
		Project:       project,
		OpenedBrowser: false,
	}

	// Open browser if requested or if output was auto-generated
	if openBrowser || autoOpen {
		if err := browser.OpenInBrowser(outputPath); err != nil {
			// Don't fail, just note that browser wasn't opened
			result.OpenedBrowser = false
		} else {
			result.OpenedBrowser = true
		}
	}

	return result, nil
}

// GenerateHTMLFromFile generates an HTML file from a JSONL file path directly.
// If outputPath is empty, a temporary file is created and auto-opened in the browser.
// If openBrowser is true, the HTML file is opened in the default browser.
func (s *SessionService) GenerateHTMLFromFile(inputPath, outputPath string, openBrowser bool) (*HTMLGenerationResult, error) {
	// Verify the file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", inputPath)
	}

	// Parse the JSONL file
	entries, err := parser.ReadJSONLFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Process entries
	processed := processor.ProcessEntries(entries)

	// Determine output path
	autoOpen := false
	if outputPath == "" {
		// Generate unique filename based on input file name and timestamp
		baseName := filepath.Base(inputPath)
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
		// Truncate base name if too long
		if len(baseName) > 8 {
			baseName = baseName[:8]
		}
		timestamp := time.Now().Format(constants.TempFileTimestampFormat)
		outputPath = filepath.Join(os.TempDir(), fmt.Sprintf(constants.TempFileNameFormat, baseName, timestamp))
		autoOpen = true
	}

	// Generate HTML
	err = renderer.GenerateHTML(processed, outputPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HTML: %w", err)
	}

	result := &HTMLGenerationResult{
		OutputPath:    outputPath,
		SessionID:     "",
		Project:       "",
		OpenedBrowser: false,
	}

	// Open browser if requested or if output was auto-generated
	if openBrowser || autoOpen {
		if err := browser.OpenInBrowser(outputPath); err != nil {
			// Don't fail, just note that browser wasn't opened
			result.OpenedBrowser = false
		} else {
			result.OpenedBrowser = true
		}
	}

	return result, nil
}

// FindSessionsByAgentType finds sessions that used a specific agent type.
func (s *SessionService) FindSessionsByAgentType(agentType, projectName string, days int, limit int) ([]AgentUsageInfo, error) {
	var projectsToSearch []models.Project

	if projectName != "" {
		project, err := s.projectService.FindProjectByName(projectName)
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

	var results []AgentUsageInfo

	for _, project := range projectsToSearch {
		sessions, err := s.ListSessions(project.Name, days, true, 0)
		if err != nil {
			continue
		}

		for _, session := range sessions {
			for _, agentUsed := range session.AgentTypesUsed {
				if strings.EqualFold(agentUsed, agentType) {
					results = append(results, AgentUsageInfo{
						SessionID:  session.SessionID,
						Project:    project.Name,
						Timestamp:  session.StartTime,
						UsageCount: 1, // TODO: count actual usages
					})
					break
				}
			}
		}

		if limit > 0 && len(results) >= limit {
			results = results[:limit]
			break
		}
	}

	return results, nil
}

// AgentUsageInfo represents agent usage in a session.
type AgentUsageInfo struct {
	SessionID  string    `json:"session_id"`
	Project    string    `json:"project"`
	Timestamp  time.Time `json:"timestamp"`
	UsageCount int       `json:"usage_count"`
	Prompts    []string  `json:"prompts,omitempty"`
}

// getSessionInfo extracts metadata from a session file.
func (s *SessionService) getSessionInfo(filePath, sessionID, projectName string, includeAgentTypes bool) (*models.SessionInfo, error) {
	entries, err := parser.ReadJSONLFile(filePath)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, nil
	}

	info := &models.SessionInfo{
		SessionID:    sessionID,
		Project:      projectName,
		MessageCount: len(entries),
		FilePath:     filePath,
	}

	// Find min/max timestamps and collect metadata
	for _, entry := range entries {
		if entry.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
				// Track earliest timestamp as start time
				if info.StartTime.IsZero() || t.Before(info.StartTime) {
					info.StartTime = t
				}
				// Track latest timestamp as end time
				if info.EndTime.IsZero() || t.After(info.EndTime) {
					info.EndTime = t
				}
			}
		}
		// Get CWD and GitBranch from first entry that has them
		if info.CWD == "" && entry.CWD != "" {
			info.CWD = entry.CWD
		}
		if info.GitBranch == "" && entry.GitBranch != "" {
			info.GitBranch = entry.GitBranch
		}
	}

	// Get first user message
	for _, entry := range entries {
		if entry.Type == "user" || isUserMessage(entry) {
			info.FirstUserMessage = extractFirstUserMessage(entry)
			break
		}
	}

	// Extract agent types if requested
	if includeAgentTypes {
		info.AgentTypesUsed = extractAgentTypes(entries)
	}

	return info, nil
}

// findSessionFile finds the session file path.
func (s *SessionService) findSessionFile(sessionID, projectName string) (string, string, error) {
	var projectsToSearch []models.Project

	if projectName != "" {
		project, err := s.projectService.FindProjectByName(projectName)
		if err != nil {
			return "", "", err
		}
		if project != nil {
			projectsToSearch = append(projectsToSearch, *project)
		}
	} else {
		projects, err := s.projectService.ListProjects("")
		if err != nil {
			return "", "", err
		}
		projectsToSearch = projects
	}

	for _, project := range projectsToSearch {
		projectDir := s.projectService.GetProjectDir(project.EncodedPath)
		filePath := filepath.Join(projectDir, sessionID+".jsonl")
		if _, err := os.Stat(filePath); err == nil {
			return filePath, project.Name, nil
		}
	}

	return "", "", nil
}

// isUserMessage checks if an entry is a user message.
func isUserMessage(entry models.LogEntry) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return false
	}
	return msg["role"] == "user"
}

// extractFirstUserMessage extracts the first user message content.
func extractFirstUserMessage(entry models.LogEntry) string {
	var msg map[string]interface{}
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return ""
	}

	content, ok := msg["content"]
	if !ok {
		return ""
	}

	switch c := content.(type) {
	case string:
		if len(c) > 200 {
			return c[:200] + "..."
		}
		return c
	case []interface{}:
		for _, item := range c {
			if m, ok := item.(map[string]interface{}); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						if len(text) > 200 {
							return text[:200] + "..."
						}
						return text
					}
				}
			}
		}
	}

	return ""
}

// extractAgentTypes extracts subagent_type values from Task tool calls.
func extractAgentTypes(entries []models.LogEntry) []string {
	types := make(map[string]bool)

	for _, entry := range entries {
		var msg map[string]interface{}
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		content, ok := msg["content"].([]interface{})
		if !ok {
			continue
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
			if name != "Task" {
				continue
			}

			input, ok := m["input"].(map[string]interface{})
			if !ok {
				continue
			}

			if subagentType, ok := input["subagent_type"].(string); ok && subagentType != "" {
				types[subagentType] = true
			}
		}
	}

	result := make([]string, 0, len(types))
	for t := range types {
		result = append(result, t)
	}
	sort.Strings(result)

	return result
}
