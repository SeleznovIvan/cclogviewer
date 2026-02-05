package service

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/brads3290/cclogviewer/internal/models"
)

// ProjectService handles project discovery and management.
type ProjectService struct {
	claudeDir string
}

// NewProjectService creates a new ProjectService.
func NewProjectService(claudeDir string) *ProjectService {
	if claudeDir == "" {
		home, _ := os.UserHomeDir()
		claudeDir = filepath.Join(home, ".claude")
	}
	return &ProjectService{claudeDir: claudeDir}
}

// ListProjects returns all Claude Code projects with metadata.
func (s *ProjectService) ListProjects(sortBy string) ([]models.Project, error) {
	projectsDir := filepath.Join(s.claudeDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	var projects []models.Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		encodedPath := entry.Name()
		decodedPath := decodeProjectPath(encodedPath)
		projectName := filepath.Base(decodedPath)

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Count session files
		sessionCount := countSessionFiles(filepath.Join(projectsDir, encodedPath))

		projects = append(projects, models.Project{
			Name:         projectName,
			Path:         decodedPath,
			EncodedPath:  encodedPath,
			SessionCount: sessionCount,
			LastModified: info.ModTime(),
		})
	}

	// Sort projects
	sortProjects(projects, sortBy)

	return projects, nil
}

// FindProjectByName finds a project by name or partial path match.
func (s *ProjectService) FindProjectByName(name string) (*models.Project, error) {
	projects, err := s.ListProjects("")
	if err != nil {
		return nil, err
	}

	// Exact name match first
	for _, p := range projects {
		if p.Name == name {
			return &p, nil
		}
	}

	// Partial path match
	nameLower := strings.ToLower(name)
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Path), nameLower) ||
			strings.Contains(strings.ToLower(p.Name), nameLower) {
			return &p, nil
		}
	}

	return nil, nil
}

// GetProjectDir returns the project directory path.
func (s *ProjectService) GetProjectDir(encodedPath string) string {
	return filepath.Join(s.claudeDir, "projects", encodedPath)
}

// GetClaudeDir returns the Claude directory path.
func (s *ProjectService) GetClaudeDir() string {
	return s.claudeDir
}

// decodeProjectPath converts encoded path to actual path.
// "-Users-name-Projects-foo" -> "/Users/name/Projects/foo"
func decodeProjectPath(encoded string) string {
	if encoded == "" {
		return ""
	}

	// Handle the leading dash which represents root /
	if strings.HasPrefix(encoded, "-") {
		encoded = encoded[1:]
	}

	// Replace dashes with slashes
	// But we need to handle paths that might have actual dashes
	// Claude Code uses a specific encoding pattern
	return "/" + strings.ReplaceAll(encoded, "-", "/")
}

// encodeProjectPath converts actual path to encoded path.
// "/Users/name/Projects/foo" -> "-Users-name-Projects-foo"
func encodeProjectPath(path string) string {
	// Remove leading slash and replace slashes with dashes
	path = strings.TrimPrefix(path, "/")
	return "-" + strings.ReplaceAll(path, "/", "-")
}

// countSessionFiles counts main session files in a project directory.
func countSessionFiles(projectDir string) int {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return 0
	}

	count := 0
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.jsonl$`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Count only main session files (UUID.jsonl), not agent-*.jsonl
		if uuidPattern.MatchString(entry.Name()) {
			count++
		}
	}

	return count
}

// sortProjects sorts projects by the specified field.
func sortProjects(projects []models.Project, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(projects, func(i, j int) bool {
			return projects[i].Name < projects[j].Name
		})
	case "session_count":
		sort.Slice(projects, func(i, j int) bool {
			return projects[i].SessionCount > projects[j].SessionCount
		})
	default: // "last_modified" or empty
		sort.Slice(projects, func(i, j int) bool {
			return projects[i].LastModified.After(projects[j].LastModified)
		})
	}
}
