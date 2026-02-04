package commands

import (
	"flag"
)

// AgentsCmd implements the agents command.
type AgentsCmd struct {
	Project       string
	IncludeGlobal bool
}

func (c *AgentsCmd) Name() string {
	return "agents"
}

func (c *AgentsCmd) Description() string {
	return "List available agent definitions (global and project-specific)"
}

func (c *AgentsCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.Project, "project", "", "Project path to include project-specific agents")
	fs.BoolVar(&c.IncludeGlobal, "include-global", true, "Include global agents from ~/.claude/agents/")
}

func (c *AgentsCmd) Run(ctx *Context, args []string) error {
	// If project name given, resolve to path
	projectPath := c.Project
	if projectPath != "" {
		project, err := ctx.Services.Project.FindProjectByName(projectPath)
		if err == nil && project != nil {
			projectPath = project.Path
		}
	}

	agents, err := ctx.Services.Agent.ListAgents(projectPath, c.IncludeGlobal)
	if err != nil {
		return err
	}

	out := NewOutputWriter(ctx.Output, ctx.Config.JSONOutput)

	if ctx.Config.JSONOutput {
		return out.WriteJSON(map[string]interface{}{
			"agents": agents,
			"count":  len(agents),
		})
	}

	// Human-readable output
	if len(agents) == 0 {
		out.PrintLine("No agents found")
		return nil
	}

	headers := []string{"Name", "Scope", "Description"}
	var rows [][]string
	for _, a := range agents {
		rows = append(rows, []string{
			a.Name,
			a.Scope,
			Truncate(a.Description, 50),
		})
	}
	out.WriteTable(headers, rows)

	return nil
}
