package service

import (
	"bufio"
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

	// When searching across multiple projects, limit sessions per project to avoid
	// excessive file parsing. Use a reasonable limit that balances coverage vs performance.
	perProjectLimit := 0 // 0 means no limit for single project searches
	if len(projectsToSearch) > 1 {
		// Limit sessions per project when searching across all projects
		perProjectLimit = 50
		if limit > 0 && limit < 50 {
			// If user wants fewer results, we can check fewer sessions per project
			perProjectLimit = limit * 5
		}
	}

	for _, project := range projectsToSearch {
		sessions, err := s.ListSessions(project.Name, days, true, perProjectLimit)
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

// GetSessionSummary returns a lightweight summary of a session.
func (s *SessionService) GetSessionSummary(sessionID, agentID, projectName string, includeSidechains bool) (*models.SessionSummary, error) {
	processed, project, err := s.loadProcessedEntries(sessionID, agentID, projectName, includeSidechains)
	if err != nil {
		return nil, err
	}
	if processed == nil {
		return nil, nil
	}

	return s.computeSummary(sessionID, agentID, project, processed), nil
}

// GetToolUsageStats returns tool usage statistics for a session.
func (s *SessionService) GetToolUsageStats(sessionID, agentID, projectName string, includeSidechains bool) (*models.ToolUsageStats, error) {
	processed, _, err := s.loadProcessedEntries(sessionID, agentID, projectName, includeSidechains)
	if err != nil {
		return nil, err
	}
	if processed == nil {
		return nil, nil
	}

	return s.computeToolStats(sessionID, agentID, processed), nil
}

// GetSessionErrors returns errors found in a session.
func (s *SessionService) GetSessionErrors(sessionID, agentID, projectName string, includeSidechains bool, limit int) (*models.SessionErrors, error) {
	processed, _, err := s.loadProcessedEntries(sessionID, agentID, projectName, includeSidechains)
	if err != nil {
		return nil, err
	}
	if processed == nil {
		return nil, nil
	}

	return s.computeErrors(sessionID, agentID, processed, limit), nil
}

// GetSessionTimeline returns a condensed timeline of session events.
func (s *SessionService) GetSessionTimeline(sessionID, agentID, projectName string, includeSidechains bool, limit int) (*models.SessionTimeline, error) {
	processed, _, err := s.loadProcessedEntries(sessionID, agentID, projectName, includeSidechains)
	if err != nil {
		return nil, err
	}
	if processed == nil {
		return nil, nil
	}

	return s.computeTimeline(sessionID, agentID, processed, limit), nil
}

// GetSessionStats returns aggregated session statistics.
func (s *SessionService) GetSessionStats(sessionID, agentID, projectName string, includeSidechains bool, errorsLimit int) (*models.SessionStats, error) {
	processed, project, err := s.loadProcessedEntries(sessionID, agentID, projectName, includeSidechains)
	if err != nil {
		return nil, err
	}
	if processed == nil {
		return nil, nil
	}

	stats := &models.SessionStats{
		SessionID:   sessionID,
		Project:     project,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if agentID != "" {
		stats.AgentID = &agentID
	}

	stats.Summary = s.computeSummary(sessionID, agentID, project, processed)
	stats.ToolStats = s.computeToolStats(sessionID, agentID, processed)
	stats.Errors = s.computeErrors(sessionID, agentID, processed, errorsLimit)

	return stats, nil
}

// loadProcessedEntries loads and processes entries for a session or specific agent.
func (s *SessionService) loadProcessedEntries(sessionID, agentID, projectName string, includeSidechains bool) ([]*models.ProcessedEntry, string, error) {
	// If agentID is specified, load that specific agent file
	if agentID != "" {
		return s.loadAgentEntries(sessionID, agentID, projectName)
	}

	// Otherwise load the main session
	filePath, project, err := s.findSessionFile(sessionID, projectName)
	if err != nil {
		return nil, "", err
	}
	if filePath == "" {
		return nil, "", nil
	}

	entries, err := parser.ReadJSONLFile(filePath)
	if err != nil {
		return nil, "", err
	}

	processed := processor.ProcessEntries(entries)

	// Filter sidechains if not included
	if !includeSidechains {
		var filtered []*models.ProcessedEntry
		for _, e := range processed {
			if !e.IsSidechain {
				filtered = append(filtered, e)
			}
		}
		processed = filtered
	}

	return processed, project, nil
}

// loadAgentEntries loads entries for a specific agent by ID.
func (s *SessionService) loadAgentEntries(sessionID, agentID, projectName string) ([]*models.ProcessedEntry, string, error) {
	// Find the project
	project, err := s.projectService.FindProjectByName(projectName)
	if err != nil {
		return nil, "", err
	}

	projectDir := ""
	projectNameResult := ""

	if project != nil {
		projectDir = s.projectService.GetProjectDir(project.EncodedPath)
		projectNameResult = project.Name
	} else {
		// Search all projects if no specific project
		projects, err := s.projectService.ListProjects("")
		if err != nil {
			return nil, "", err
		}

		for _, p := range projects {
			dir := s.projectService.GetProjectDir(p.EncodedPath)
			if s.findAgentFile(dir, sessionID, agentID) != "" {
				projectDir = dir
				projectNameResult = p.Name
				break
			}
		}
	}

	if projectDir == "" {
		return nil, "", nil
	}

	agentFile := s.findAgentFile(projectDir, sessionID, agentID)
	if agentFile == "" {
		return nil, "", nil
	}

	entries, err := readSingleJSONLFileForService(agentFile)
	if err != nil {
		return nil, "", err
	}

	processed := processor.ProcessEntries(entries)
	return processed, projectNameResult, nil
}

// findAgentFile finds an agent file by ID in the project directory.
func (s *SessionService) findAgentFile(projectDir, sessionID, agentID string) string {
	// Try direct agent file: {project}/agent-{id}.jsonl
	directPath := filepath.Join(projectDir, fmt.Sprintf("agent-%s.jsonl", agentID))
	if _, err := os.Stat(directPath); err == nil {
		return directPath
	}

	// Try subagents directory: {project}/{session_id}/subagents/agent-{id}.jsonl
	if sessionID != "" {
		subagentPath := filepath.Join(projectDir, sessionID, "subagents", fmt.Sprintf("agent-%s.jsonl", agentID))
		if _, err := os.Stat(subagentPath); err == nil {
			return subagentPath
		}
	}

	// Search all session directories for the agent file
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subagentPath := filepath.Join(projectDir, entry.Name(), "subagents", fmt.Sprintf("agent-%s.jsonl", agentID))
		if _, err := os.Stat(subagentPath); err == nil {
			return subagentPath
		}
	}

	return ""
}

// readSingleJSONLFileForService reads a single JSONL file without loading subagents.
func readSingleJSONLFileForService(filename string) ([]models.LogEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []models.LogEntry
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024*10) // 10MB max

	for scanner.Scan() {
		var entry models.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Type == "summary" {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// computeSummary computes a summary from processed entries.
func (s *SessionService) computeSummary(sessionID, agentID, project string, entries []*models.ProcessedEntry) *models.SessionSummary {
	summary := &models.SessionSummary{
		SessionID: sessionID,
		Project:   project,
	}

	if agentID != "" {
		summary.AgentID = &agentID
	}

	var (
		totalInput, totalOutput, cacheRead, cacheCreation int
		totalToolCalls, successCalls, failedCalls         int
		userMessages, assistantMessages                   int
		errorCount                                        int
		toolNames                                         = make(map[string]bool)
		agentTypes                                        = make(map[string]bool)
		minTime, maxTime                                  time.Time
	)

	for _, e := range entries {
		// Count messages
		if e.Role == "user" {
			userMessages++
		} else if e.Role == "assistant" {
			assistantMessages++
		}

		// Count tokens
		totalInput += e.InputTokens
		totalOutput += e.OutputTokens
		cacheRead += e.CacheReadTokens
		cacheCreation += e.CacheCreationTokens

		// Count tool calls
		for _, tc := range e.ToolCalls {
			totalToolCalls++
			toolNames[tc.Name] = true
			if tc.Result != nil && tc.Result.IsError {
				failedCalls++
			} else {
				successCalls++
			}
		}

		// Count errors
		if e.IsError {
			errorCount++
		}

		// Track sidechains/agent types
		if e.IsSidechain && e.AgentID != "" {
			agentTypes[e.AgentID] = true
		}

		// Track timestamps for duration (use RawTimestamp which is RFC3339 format)
		if e.RawTimestamp != "" {
			if t, err := time.Parse(time.RFC3339, e.RawTimestamp); err == nil {
				if minTime.IsZero() || t.Before(minTime) {
					minTime = t
				}
				if maxTime.IsZero() || t.After(maxTime) {
					maxTime = t
				}
			}
		}
	}

	summary.MessageCount = len(entries)
	summary.UserMessages = userMessages
	summary.AssistantMsgs = assistantMessages

	if !minTime.IsZero() {
		summary.Date = minTime.Format("2006-01-02")
		summary.DurationMinutes = int(maxTime.Sub(minTime).Minutes())
	}

	summary.Tokens = &models.TokenStats{
		TotalInput:    totalInput,
		TotalOutput:   totalOutput,
		CacheRead:     cacheRead,
		CacheCreation: cacheCreation,
	}

	summary.ToolCalls = &models.ToolCallStats{
		Total:       totalToolCalls,
		UniqueTools: len(toolNames),
		Success:     successCalls,
		Failed:      failedCalls,
	}

	agentList := make([]string, 0, len(agentTypes))
	for a := range agentTypes {
		agentList = append(agentList, a)
	}
	sort.Strings(agentList)

	summary.Sidechains = &models.SidechainStats{
		Count:      len(agentTypes),
		AgentTypes: agentList,
	}

	summary.HasErrors = errorCount > 0
	summary.ErrorCount = errorCount

	return summary
}

// computeToolStats computes tool usage statistics from processed entries.
func (s *SessionService) computeToolStats(sessionID, agentID string, entries []*models.ProcessedEntry) *models.ToolUsageStats {
	stats := &models.ToolUsageStats{
		SessionID: sessionID,
	}

	if agentID != "" {
		stats.AgentID = &agentID
	}

	toolCounts := make(map[string]*models.ToolUsageStat)
	var toolSequence []models.ToolSequenceEntry
	var firstTool, lastTool string
	maxCount := 0
	maxFailed := 0
	mostUsed := ""
	mostFailed := ""

	for _, e := range entries {
		for _, tc := range e.ToolCalls {
			toolSequence = append(toolSequence, models.ToolSequenceEntry{
				Name:      tc.Name,
				ToolUseID: tc.ID,
			})

			if firstTool == "" {
				firstTool = tc.Name
			}
			lastTool = tc.Name

			if _, exists := toolCounts[tc.Name]; !exists {
				toolCounts[tc.Name] = &models.ToolUsageStat{Name: tc.Name}
			}

			toolCounts[tc.Name].Count++

			// Check if tool call failed
			if tc.Result != nil && tc.Result.IsError {
				toolCounts[tc.Name].Failed++
			} else {
				toolCounts[tc.Name].Success++
			}

			// Track patterns
			if toolCounts[tc.Name].Count > maxCount {
				maxCount = toolCounts[tc.Name].Count
				mostUsed = tc.Name
			}
			if toolCounts[tc.Name].Failed > maxFailed {
				maxFailed = toolCounts[tc.Name].Failed
				mostFailed = tc.Name
			}
		}
	}

	// Convert map to sorted slice
	tools := make([]models.ToolUsageStat, 0, len(toolCounts))
	for _, t := range toolCounts {
		tools = append(tools, *t)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Count > tools[j].Count
	})

	stats.Tools = tools
	stats.ToolSequence = toolSequence
	stats.Patterns = &models.ToolPatterns{
		MostUsed:   mostUsed,
		MostFailed: mostFailed,
		FirstTool:  firstTool,
		LastTool:   lastTool,
	}

	return stats
}

// computeErrors extracts errors from processed entries.
func (s *SessionService) computeErrors(sessionID, agentID string, entries []*models.ProcessedEntry, limit int) *models.SessionErrors {
	result := &models.SessionErrors{
		SessionID:  sessionID,
		Categories: &models.ErrorCategories{},
	}

	if agentID != "" {
		result.AgentID = &agentID
	}

	var errors []models.SessionError

	// Collect all errors with their entry indices and UUIDs
	for i, e := range entries {
		// Check if entry itself is an error
		if e.IsError {
			result.Categories.ToolError++

			err := models.SessionError{
				UUID:       e.UUID,
				Timestamp:  e.Timestamp,
				Type:       "tool_error",
				Message:    truncateString(e.Content, 500),
				EntryIndex: i,
			}

			if e.IsSidechain && e.AgentID != "" {
				err.Sidechain = e.AgentID
			}

			errors = append(errors, err)
		}

		// Check tool call results for errors
		for _, tc := range e.ToolCalls {
			if tc.Result != nil && tc.Result.IsError {
				result.Categories.ToolError++

				err := models.SessionError{
					UUID:       e.UUID, // Use parent entry's UUID
					Timestamp:  tc.Result.Timestamp,
					Type:       "tool_error",
					ToolName:   tc.Name,
					Message:    truncateString(tc.Result.Content, 500),
					EntryIndex: i,
				}

				if e.IsSidechain && e.AgentID != "" {
					err.Sidechain = e.AgentID
				}

				errors = append(errors, err)
			}
		}

		// Look for console errors in content (browser_console_messages results)
		if strings.Contains(strings.ToLower(e.Content), "error") && strings.Contains(e.Content, "console") {
			result.Categories.ConsoleError++
		}
	}

	result.TotalErrors = len(errors)

	// Apply limit
	if limit > 0 && len(errors) > limit {
		errors = errors[:limit]
	}

	result.Errors = errors

	return result
}

// getErrorContextLogs returns context logs surrounding an error at the given entry index.
func (s *SessionService) getErrorContextLogs(entries []*models.ProcessedEntry, errorIndex int, contextSize int) []models.ContextLog {
	var contextLogs []models.ContextLog

	// Get entries before the error
	for offset := -contextSize; offset < 0; offset++ {
		idx := errorIndex + offset
		if idx < 0 {
			continue
		}
		contextLogs = append(contextLogs, s.entryToContextLog(entries[idx], offset))
	}

	// Get entries after the error
	for offset := 1; offset <= contextSize; offset++ {
		idx := errorIndex + offset
		if idx >= len(entries) {
			break
		}
		contextLogs = append(contextLogs, s.entryToContextLog(entries[idx], offset))
	}

	return contextLogs
}

// entryToContextLog converts a ProcessedEntry to a ContextLog.
func (s *SessionService) entryToContextLog(e *models.ProcessedEntry, offset int) models.ContextLog {
	log := models.ContextLog{
		Offset:       offset,
		Timestamp:    e.Timestamp,
		Role:         e.Role,
		Content:      truncateString(e.Content, 5000), // Larger limit for debugging context
		IsToolResult: e.IsToolResult,
		IsError:      e.IsError,
	}

	// If entry has tool calls, include tool details
	if len(e.ToolCalls) > 0 {
		tc := e.ToolCalls[0]
		log.ToolName = tc.Name
		log.ToolUseID = tc.ID
		log.ToolInput = tc.RawInput

		// Include tool result content if available
		if tc.Result != nil {
			log.ToolOutput = truncateString(tc.Result.Content, 5000)
		}

		// If there are multiple tool calls, indicate that
		if len(e.ToolCalls) > 1 {
			log.ToolName = fmt.Sprintf("%s (+%d more)", tc.Name, len(e.ToolCalls)-1)
		}
	}

	return log
}

// computeTimeline creates a condensed timeline from processed entries.
func (s *SessionService) computeTimeline(sessionID, agentID string, entries []*models.ProcessedEntry, limit int) *models.SessionTimeline {
	timeline := &models.SessionTimeline{
		SessionID:    sessionID,
		TotalEntries: len(entries),
	}

	if agentID != "" {
		timeline.AgentID = &agentID
	}

	var items []models.TimelineEntry
	step := 0

	for _, e := range entries {
		step++

		// For tool calls, create separate timeline entries
		if len(e.ToolCalls) > 0 {
			for _, tc := range e.ToolCalls {
				item := models.TimelineEntry{
					Step:      step,
					Timestamp: e.Timestamp,
					Role:      e.Role,
					Type:      "tool_call",
					Tool:      tc.Name,
					ToolUseID: tc.ID,
					Summary:   truncateString(extractToolSummary(tc), 150),
					Tokens:    e.OutputTokens,
				}

				if tc.Result != nil {
					if tc.Result.IsError {
						item.Status = "failed"
					} else {
						item.Status = "success"
					}
				}

				if e.IsSidechain && e.AgentID != "" {
					item.Sidechain = e.AgentID
				}

				items = append(items, item)
				step++
			}
		} else {
			// Regular message
			item := models.TimelineEntry{
				Step:      step,
				Timestamp: e.Timestamp,
				Role:      e.Role,
				Type:      "message",
				Summary:   truncateString(e.Content, 150),
				Tokens:    e.OutputTokens,
			}

			if e.IsSidechain && e.AgentID != "" {
				item.Sidechain = e.AgentID
			}

			items = append(items, item)
		}

		// Apply limit during iteration
		if limit > 0 && len(items) >= limit {
			break
		}
	}

	// Final limit enforcement
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	timeline.ReturnedEntries = len(items)
	timeline.Timeline = items

	return timeline
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractToolSummary extracts a summary from a tool call.
func extractToolSummary(tc models.ToolCall) string {
	// Try to get a meaningful summary from the raw input
	if tc.RawInput != nil {
		switch input := tc.RawInput.(type) {
		case map[string]interface{}:
			// Common fields to extract
			for _, key := range []string{"command", "query", "url", "file_path", "pattern", "prompt"} {
				if val, ok := input[key]; ok {
					if str, ok := val.(string); ok {
						return str
					}
				}
			}
		case string:
			return input
		}
	}

	return tc.Name
}

// GetLogsAroundEntry retrieves logs surrounding a specific entry identified by UUID.
// offset controls direction: negative = entries before target, positive = entries after target.
// Examples: offset=-3 gets 3 entries before + target, offset=+3 gets target + 3 entries after.
func (s *SessionService) GetLogsAroundEntry(sessionID, targetUUID, projectName string, offset int, includeSidechains bool) (*models.LogsAroundEntry, error) {
	processed, project, err := s.loadProcessedEntries(sessionID, "", projectName, includeSidechains)
	if err != nil {
		return nil, err
	}
	if processed == nil {
		return nil, nil
	}

	// Find the target entry by UUID
	targetIndex := -1
	for i, e := range processed {
		if e.UUID == targetUUID {
			targetIndex = i
			break
		}
	}

	if targetIndex == -1 {
		return nil, fmt.Errorf("entry with UUID %s not found", targetUUID)
	}

	// Default offset to -3 (before) if not specified
	if offset == 0 {
		offset = -3
	}

	result := &models.LogsAroundEntry{
		SessionID:   sessionID,
		Project:     project,
		TargetUUID:  targetUUID,
		TargetIndex: targetIndex,
		Offset:      offset,
		TotalCount:  len(processed),
	}

	if offset < 0 {
		// Negative offset: get entries BEFORE the target
		absOffset := -offset
		for i := -absOffset; i < 0; i++ {
			idx := targetIndex + i
			if idx < 0 {
				continue
			}
			result.Entries = append(result.Entries, s.entryToContextLog(processed[idx], i))
		}
		// Include the target entry itself at offset 0
		result.Entries = append(result.Entries, s.entryToContextLog(processed[targetIndex], 0))
	} else {
		// Positive offset: get entries AFTER the target
		// Include the target entry itself at offset 0
		result.Entries = append(result.Entries, s.entryToContextLog(processed[targetIndex], 0))
		for i := 1; i <= offset; i++ {
			idx := targetIndex + i
			if idx >= len(processed) {
				break
			}
			result.Entries = append(result.Entries, s.entryToContextLog(processed[idx], i))
		}
	}

	return result, nil
}

// fileLabel returns a short label derived from the file path for use in response models.
func fileLabel(filePath string) string {
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// loadProcessedEntriesFromFile loads and processes entries directly from a JSONL file path.
func (s *SessionService) loadProcessedEntriesFromFile(filePath string, includeSidechains bool) ([]*models.ProcessedEntry, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	entries, err := parser.ReadJSONLFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	processed := processor.ProcessEntries(entries)

	if !includeSidechains {
		var filtered []*models.ProcessedEntry
		for _, e := range processed {
			if !e.IsSidechain {
				filtered = append(filtered, e)
			}
		}
		processed = filtered
	}

	return processed, nil
}

// GetSessionLogsFromFile retrieves full processed logs from a JSONL file path.
func (s *SessionService) GetSessionLogsFromFile(filePath string, includeSidechains bool) (*models.SessionLogs, error) {
	entries, err := parser.ReadJSONLFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	processed := processor.ProcessEntries(entries)

	label := fileLabel(filePath)
	logs := &models.SessionLogs{
		SessionID: label,
		Project:   filePath,
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

		for _, tc := range entry.ToolCalls {
			logEntry.ToolCalls = append(logEntry.ToolCalls, models.SessionToolCall{
				Name:  tc.Name,
				Input: tc.RawInput,
			})
		}

		logs.Entries = append(logs.Entries, logEntry)

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

// GetSessionSummaryFromFile returns a lightweight summary from a JSONL file path.
func (s *SessionService) GetSessionSummaryFromFile(filePath string, includeSidechains bool) (*models.SessionSummary, error) {
	processed, err := s.loadProcessedEntriesFromFile(filePath, includeSidechains)
	if err != nil {
		return nil, err
	}

	label := fileLabel(filePath)
	return s.computeSummary(label, "", filePath, processed), nil
}

// GetToolUsageStatsFromFile returns tool usage statistics from a JSONL file path.
func (s *SessionService) GetToolUsageStatsFromFile(filePath string, includeSidechains bool) (*models.ToolUsageStats, error) {
	processed, err := s.loadProcessedEntriesFromFile(filePath, includeSidechains)
	if err != nil {
		return nil, err
	}

	label := fileLabel(filePath)
	return s.computeToolStats(label, "", processed), nil
}

// GetSessionErrorsFromFile returns errors found in a JSONL file.
func (s *SessionService) GetSessionErrorsFromFile(filePath string, includeSidechains bool, limit int) (*models.SessionErrors, error) {
	processed, err := s.loadProcessedEntriesFromFile(filePath, includeSidechains)
	if err != nil {
		return nil, err
	}

	label := fileLabel(filePath)
	return s.computeErrors(label, "", processed, limit), nil
}

// GetSessionTimelineFromFile returns a condensed timeline from a JSONL file.
func (s *SessionService) GetSessionTimelineFromFile(filePath string, includeSidechains bool, limit int) (*models.SessionTimeline, error) {
	processed, err := s.loadProcessedEntriesFromFile(filePath, includeSidechains)
	if err != nil {
		return nil, err
	}

	label := fileLabel(filePath)
	return s.computeTimeline(label, "", processed, limit), nil
}

// GetSessionStatsFromFile returns aggregated statistics from a JSONL file.
func (s *SessionService) GetSessionStatsFromFile(filePath string, includeSidechains bool, errorsLimit int) (*models.SessionStats, error) {
	processed, err := s.loadProcessedEntriesFromFile(filePath, includeSidechains)
	if err != nil {
		return nil, err
	}

	label := fileLabel(filePath)
	stats := &models.SessionStats{
		SessionID:   label,
		Project:     filePath,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	stats.Summary = s.computeSummary(label, "", filePath, processed)
	stats.ToolStats = s.computeToolStats(label, "", processed)
	stats.Errors = s.computeErrors(label, "", processed, errorsLimit)

	return stats, nil
}

// GetLogsAroundEntryFromFile returns logs around a specific entry from a JSONL file.
func (s *SessionService) GetLogsAroundEntryFromFile(filePath, targetUUID string, offset int, includeSidechains bool) (*models.LogsAroundEntry, error) {
	processed, err := s.loadProcessedEntriesFromFile(filePath, includeSidechains)
	if err != nil {
		return nil, err
	}

	// Find the target entry by UUID
	targetIndex := -1
	for i, e := range processed {
		if e.UUID == targetUUID {
			targetIndex = i
			break
		}
	}

	if targetIndex == -1 {
		return nil, fmt.Errorf("entry with UUID %s not found", targetUUID)
	}

	if offset == 0 {
		offset = -3
	}

	label := fileLabel(filePath)
	result := &models.LogsAroundEntry{
		SessionID:   label,
		Project:     filePath,
		TargetUUID:  targetUUID,
		TargetIndex: targetIndex,
		Offset:      offset,
		TotalCount:  len(processed),
	}

	if offset < 0 {
		absOffset := -offset
		for i := -absOffset; i < 0; i++ {
			idx := targetIndex + i
			if idx < 0 {
				continue
			}
			result.Entries = append(result.Entries, s.entryToContextLog(processed[idx], i))
		}
		result.Entries = append(result.Entries, s.entryToContextLog(processed[targetIndex], 0))
	} else {
		result.Entries = append(result.Entries, s.entryToContextLog(processed[targetIndex], 0))
		for i := 1; i <= offset; i++ {
			idx := targetIndex + i
			if idx >= len(processed) {
				break
			}
			result.Entries = append(result.Entries, s.entryToContextLog(processed[idx], i))
		}
	}

	return result, nil
}
