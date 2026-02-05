package commands

import (
	"flag"
	"fmt"
)

// SessionsCmd implements the sessions command.
type SessionsCmd struct {
	Days              int
	Limit             int
	IncludeAgentTypes bool
}

func (c *SessionsCmd) Name() string {
	return "sessions"
}

func (c *SessionsCmd) Description() string {
	return "List sessions for a project with optional filtering"
}

func (c *SessionsCmd) Setup(fs *flag.FlagSet) {
	fs.IntVar(&c.Days, "days", 0, "Only include sessions from the last N days")
	fs.IntVar(&c.Limit, "limit", 50, "Maximum sessions to return")
	fs.BoolVar(&c.IncludeAgentTypes, "include-agent-types", false, "Include subagent types used in each session")
}

func (c *SessionsCmd) Run(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("project name is required\nUsage: cclogviewer sessions <project> [flags]")
	}

	project := args[0]
	sessions, err := ctx.Services.Session.ListSessions(project, c.Days, c.IncludeAgentTypes, c.Limit)
	if err != nil {
		return err
	}

	out := NewOutputWriter(ctx.Output, ctx.Config.JSONOutput)

	if ctx.Config.JSONOutput {
		return out.WriteJSON(map[string]interface{}{
			"project":  project,
			"sessions": sessions,
			"count":    len(sessions),
		})
	}

	// Human-readable output
	if len(sessions) == 0 {
		out.PrintLine("No sessions found for project: %s", project)
		return nil
	}

	out.PrintLine("Sessions for project: %s\n", project)

	headers := []string{"Session ID", "Start Time", "Messages", "First Message"}
	if c.IncludeAgentTypes {
		headers = append(headers, "Agent Types")
	}

	var rows [][]string
	for _, s := range sessions {
		row := []string{
			Truncate(s.SessionID, 36),
			FormatTime(s.StartTime),
			FormatNumber(s.MessageCount),
			Truncate(s.FirstUserMessage, 40),
		}
		if c.IncludeAgentTypes {
			agents := ""
			if len(s.AgentTypesUsed) > 0 {
				agents = fmt.Sprintf("%v", s.AgentTypesUsed)
			}
			row = append(row, Truncate(agents, 30))
		}
		rows = append(rows, row)
	}
	out.WriteTable(headers, rows)

	return nil
}
