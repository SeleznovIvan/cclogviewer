package commands

import (
	"flag"
	"fmt"
	"os"
)

// ToolsCmd implements the tools command.
type ToolsCmd struct {
	AgentID           string
	Project           string
	IncludeSidechains bool
	OutputPath        string
}

func (c *ToolsCmd) Name() string {
	return "tools"
}

func (c *ToolsCmd) Description() string {
	return "Get tool usage statistics (counts, success/failure rates)"
}

func (c *ToolsCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.AgentID, "agent-id", "", "Specific subagent ID to analyze")
	fs.StringVar(&c.Project, "project", "", "Project name/path (optional)")
	fs.BoolVar(&c.IncludeSidechains, "include-sidechains", true, "Include sidechain (agent) conversations in analysis")
	fs.StringVar(&c.OutputPath, "output", "", "File path to save the stats as JSON")
}

func (c *ToolsCmd) Run(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("session ID is required\nUsage: cclogviewer tools <session-id> [flags]")
	}

	sessionID := args[0]
	stats, err := ctx.Services.Session.GetToolUsageStats(sessionID, c.AgentID, c.Project, c.IncludeSidechains)
	if err != nil {
		return err
	}

	if stats == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	out := NewOutputWriter(ctx.Output, ctx.Config.JSONOutput)

	// Save to file if output path specified
	if c.OutputPath != "" {
		file, err := os.Create(c.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()

		fileOut := NewOutputWriter(file, true)
		if err := fileOut.WriteJSON(stats); err != nil {
			return fmt.Errorf("failed to write stats: %w", err)
		}

		out.PrintLine("Tool stats saved to: %s", c.OutputPath)
		return nil
	}

	// Output to stdout
	if ctx.Config.JSONOutput {
		return out.WriteJSON(stats)
	}

	// Human-readable output
	out.PrintLine("Tool Usage Statistics: %s\n", stats.SessionID)

	headers := []string{"Tool", "Count", "Success", "Failed"}
	var rows [][]string
	for _, t := range stats.Tools {
		rows = append(rows, []string{
			t.Name,
			FormatNumber(t.Count),
			FormatNumber(t.Success),
			FormatNumber(t.Failed),
		})
	}
	out.WriteTable(headers, rows)

	if stats.Patterns != nil {
		out.PrintLine("\nPatterns:")
		out.PrintKeyValue("Most Used", stats.Patterns.MostUsed)
		out.PrintKeyValue("Most Failed", stats.Patterns.MostFailed)
		out.PrintKeyValue("First Tool", stats.Patterns.FirstTool)
		out.PrintKeyValue("Last Tool", stats.Patterns.LastTool)
	}

	return nil
}
