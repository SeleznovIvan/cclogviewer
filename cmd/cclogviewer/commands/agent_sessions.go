package commands

import (
	"flag"
	"fmt"
)

// AgentSessionsCmd implements the agent-sessions command.
type AgentSessionsCmd struct {
	Project string
	Days    int
	Limit   int
}

func (c *AgentSessionsCmd) Name() string {
	return "agent-sessions"
}

func (c *AgentSessionsCmd) Description() string {
	return "Find sessions where a specific agent type was used"
}

func (c *AgentSessionsCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.Project, "project", "", "Limit search to a specific project")
	fs.IntVar(&c.Days, "days", 0, "Only search sessions from the last N days")
	fs.IntVar(&c.Limit, "limit", 20, "Maximum sessions to return")
}

func (c *AgentSessionsCmd) Run(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("agent type is required\nUsage: cclogviewer agent-sessions <type> [flags]")
	}

	agentType := args[0]
	sessions, err := ctx.Services.Session.FindSessionsByAgentType(agentType, c.Project, c.Days, c.Limit)
	if err != nil {
		return err
	}

	out := NewOutputWriter(ctx.Output, ctx.Config.JSONOutput)

	if ctx.Config.JSONOutput {
		return out.WriteJSON(map[string]interface{}{
			"agent_type": agentType,
			"sessions":   sessions,
			"count":      len(sessions),
		})
	}

	// Human-readable output
	if len(sessions) == 0 {
		out.PrintLine("No sessions found using agent type: %s", agentType)
		return nil
	}

	out.PrintLine("Sessions using agent type: %s\n", agentType)

	headers := []string{"Session ID", "Project", "Timestamp", "Usage Count"}
	var rows [][]string
	for _, s := range sessions {
		rows = append(rows, []string{
			Truncate(s.SessionID, 36),
			s.Project,
			FormatTime(s.Timestamp),
			FormatNumber(s.UsageCount),
		})
	}
	out.WriteTable(headers, rows)

	return nil
}
