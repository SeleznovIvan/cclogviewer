package service

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/brads3290/cclogviewer/internal/models"
	"gopkg.in/yaml.v3"
)

// AgentService handles agent definition discovery.
type AgentService struct {
	projectService *ProjectService
}

// NewAgentService creates a new AgentService.
func NewAgentService(projectService *ProjectService) *AgentService {
	return &AgentService{projectService: projectService}
}

// ListAgents returns available agent definitions.
func (s *AgentService) ListAgents(projectPath string, includeGlobal bool) ([]models.AgentDefinition, error) {
	var agents []models.AgentDefinition

	// Global agents from ~/.claude/agents/
	if includeGlobal {
		globalDir := filepath.Join(s.projectService.GetClaudeDir(), "agents")
		globalAgents, err := s.loadAgentsFromDir(globalDir, "global")
		if err == nil {
			agents = append(agents, globalAgents...)
		}
	}

	// Project agents from <project>/.claude/agents/
	if projectPath != "" {
		projectAgentsDir := filepath.Join(projectPath, ".claude", "agents")
		projectAgents, err := s.loadAgentsFromDir(projectAgentsDir, "project")
		if err == nil {
			agents = append(agents, projectAgents...)
		}
	}

	return agents, nil
}

// loadAgentsFromDir loads agent definitions from a directory.
func (s *AgentService) loadAgentsFromDir(dir string, scope string) ([]models.AgentDefinition, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var agents []models.AgentDefinition
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		agent, err := parseAgentFile(filePath)
		if err != nil {
			continue
		}

		agent.Scope = scope
		agent.FilePath = filePath
		agents = append(agents, *agent)
	}

	return agents, nil
}

// parseAgentFile parses an agent definition from a .md file.
func parseAgentFile(path string) (*models.AgentDefinition, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Extract YAML frontmatter
	frontmatter, err := extractFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var agent models.AgentDefinition
	if err := yaml.Unmarshal([]byte(frontmatter), &agent); err != nil {
		return nil, err
	}

	// Handle tools field which might be comma-separated string or array
	// The YAML parser should handle arrays, but strings need special handling
	if agent.Name == "" {
		// Use filename as name
		agent.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	return &agent, nil
}

// extractFrontmatter extracts YAML frontmatter from markdown content.
func extractFrontmatter(content string) (string, error) {
	lines := strings.Split(content, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", nil
	}

	var frontmatterLines []string
	inFrontmatter := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if i == 0 && trimmed == "---" {
			inFrontmatter = true
			continue
		}

		if inFrontmatter && trimmed == "---" {
			break
		}

		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		}
	}

	return strings.Join(frontmatterLines, "\n"), nil
}
