package commands

import (
	"flag"
	"fmt"
	"os"
)

// StatsCmd implements the stats command.
type StatsCmd struct {
	AgentID           string
	Project           string
	IncludeSidechains bool
	ErrorsLimit       int
	GenerateHTML      bool
	OpenBrowser       bool
	OutputPath        string
}

func (c *StatsCmd) Name() string {
	return "stats"
}

func (c *StatsCmd) Description() string {
	return "Get comprehensive stats (combines summary, tools, errors)"
}

func (c *StatsCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.AgentID, "agent-id", "", "Specific subagent ID to analyze")
	fs.StringVar(&c.Project, "project", "", "Project name/path (optional)")
	fs.BoolVar(&c.IncludeSidechains, "include-sidechains", true, "Include sidechain (agent) conversations in analysis")
	fs.IntVar(&c.ErrorsLimit, "errors-limit", 10, "Maximum errors to include")
	fs.BoolVar(&c.GenerateHTML, "html", false, "Generate HTML visualization alongside JSON")
	fs.BoolVar(&c.OpenBrowser, "open", false, "Open HTML in browser (requires --html)")
	fs.StringVar(&c.OutputPath, "output", "", "Base path for output files (without extension)")
}

func (c *StatsCmd) Run(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("session ID is required\nUsage: cclogviewer stats <session-id> [flags]")
	}

	sessionID := args[0]
	stats, err := ctx.Services.Session.GetSessionStats(sessionID, c.AgentID, c.Project, c.IncludeSidechains, c.ErrorsLimit)
	if err != nil {
		return err
	}

	if stats == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	out := NewOutputWriter(ctx.Output, ctx.Config.JSONOutput)

	// Save to file if output path specified
	if c.OutputPath != "" {
		jsonPath := c.OutputPath + ".json"
		file, err := os.Create(jsonPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()

		fileOut := NewOutputWriter(file, true)
		if err := fileOut.WriteJSON(stats); err != nil {
			return fmt.Errorf("failed to write stats: %w", err)
		}

		out.PrintLine("Stats saved to: %s", jsonPath)
	}

	// Output to stdout
	if ctx.Config.JSONOutput {
		return out.WriteJSON(stats)
	}

	// Human-readable output
	out.PrintLine("Session Statistics: %s", stats.SessionID)
	out.PrintLine("Project: %s", stats.Project)
	out.PrintLine("Generated: %s\n", stats.GeneratedAt)

	// Summary section
	if stats.Summary != nil {
		out.PrintSection("Summary")
		out.PrintKeyValue("Date", stats.Summary.Date)
		out.PrintKeyValue("Duration", FormatDuration(stats.Summary.DurationMinutes))
		out.PrintKeyValue("Messages", fmt.Sprintf("%d total (%d user, %d assistant)",
			stats.Summary.MessageCount, stats.Summary.UserMessages, stats.Summary.AssistantMsgs))

		if stats.Summary.Tokens != nil {
			out.PrintKeyValue("Tokens", fmt.Sprintf("%s input / %s output",
				FormatNumber(stats.Summary.Tokens.TotalInput),
				FormatNumber(stats.Summary.Tokens.TotalOutput)))
		}

		if stats.Summary.ToolCalls != nil {
			out.PrintKeyValue("Tool Calls", fmt.Sprintf("%d total (%d success, %d failed)",
				stats.Summary.ToolCalls.Total, stats.Summary.ToolCalls.Success, stats.Summary.ToolCalls.Failed))
		}

		out.PrintKeyValue("Errors", FormatNumber(stats.Summary.ErrorCount))
	}

	// Tool stats section
	if stats.ToolStats != nil && len(stats.ToolStats.Tools) > 0 {
		out.PrintSection("Tool Usage")
		headers := []string{"Tool", "Count", "Success", "Failed"}
		var rows [][]string
		for _, t := range stats.ToolStats.Tools {
			rows = append(rows, []string{
				t.Name,
				FormatNumber(t.Count),
				FormatNumber(t.Success),
				FormatNumber(t.Failed),
			})
		}
		out.WriteTable(headers, rows)
	}

	// Errors section
	if stats.Errors != nil && stats.Errors.TotalErrors > 0 {
		out.PrintSection("Errors")
		out.PrintLine("Total: %d errors\n", stats.Errors.TotalErrors)

		for i, e := range stats.Errors.Errors {
			if i >= 5 {
				out.PrintLine("... and %d more errors (use --json for full list)", stats.Errors.TotalErrors-5)
				break
			}
			out.PrintLine("%d. [%s] %s: %s", i+1, e.Type, e.ToolName, Truncate(e.Message, 60))
		}
	}

	return nil
}
