package service

// Services holds all service dependencies.
type Services struct {
	Project *ProjectService
	Session *SessionService
	Agent   *AgentService
	Search  *SearchService
}

// NewServices creates a new Services instance with all services initialized.
func NewServices(claudeDir string) *Services {
	projectService := NewProjectService(claudeDir)
	sessionService := NewSessionService(projectService)
	agentService := NewAgentService(projectService)
	searchService := NewSearchService(projectService, sessionService)

	return &Services{
		Project: projectService,
		Session: sessionService,
		Agent:   agentService,
		Search:  searchService,
	}
}
