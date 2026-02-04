package commands

import (
	"flag"
	"fmt"
	"os"
)

// SummaryCmd implements the summary command.
type SummaryCmd struct {
	AgentID           string
	Project           string
	IncludeSidechains bool
	OutputPath        string
}

func (c *SummaryCmd) Name() string {
	return "summary"
}

func (c *SummaryCmd) Description() string {
	return "Get lightweight session summary (tokens, messages, errors)"
}

func (c *SummaryCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.AgentID, "agent-id", "", "Specific subagent ID to analyze")
	fs.StringVar(&c.Project, "project", "", "Project name/path (optional)")
	fs.BoolVar(&c.IncludeSidechains, "include-sidechains", true, "Include sidechain (agent) conversations in analysis")
	fs.StringVar(&c.OutputPath, "output", "", "File path to save the summary as JSON")
}

func (c *SummaryCmd) Run(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("session ID is required\nUsage: cclogviewer summary <session-id> [flags]")
	}

	sessionID := args[0]
	summary, err := ctx.Services.Session.GetSessionSummary(sessionID, c.AgentID, c.Project, c.IncludeSidechains)
	if err != nil {
		return err
	}

	if summary == nil {
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
		if err := fileOut.WriteJSON(summary); err != nil {
			return fmt.Errorf("failed to write summary: %w", err)
		}

		out.PrintLine("Summary saved to: %s", c.OutputPath)
		return nil
	}

	// Output to stdout
	if ctx.Config.JSONOutput {
		return out.WriteJSON(summary)
	}

	// Human-readable output
	out.PrintLine("Session Summary: %s", summary.SessionID)
	out.PrintLine("Project: %s", summary.Project)
	out.PrintLine("Date: %s", summary.Date)
	out.PrintLine("Duration: %s", FormatDuration(summary.DurationMinutes))
	out.PrintLine("")

	out.PrintLine("Messages: %d total (%d user, %d assistant)",
		summary.MessageCount, summary.UserMessages, summary.AssistantMsgs)

	if summary.Tokens != nil {
		out.PrintLine("Tokens: %s input / %s output",
			FormatNumber(summary.Tokens.TotalInput),
			FormatNumber(summary.Tokens.TotalOutput))
		if summary.Tokens.CacheRead > 0 || summary.Tokens.CacheCreation > 0 {
			out.PrintLine("Cache: %s read / %s creation",
				FormatNumber(summary.Tokens.CacheRead),
				FormatNumber(summary.Tokens.CacheCreation))
		}
	}

	if summary.ToolCalls != nil {
		out.PrintLine("Tool Calls: %d total (%d success, %d failed)",
			summary.ToolCalls.Total, summary.ToolCalls.Success, summary.ToolCalls.Failed)
	}

	out.PrintLine("Errors: %d found", summary.ErrorCount)

	if summary.Sidechains != nil && summary.Sidechains.Count > 0 {
		out.PrintLine("Sidechains: %d (%v)", summary.Sidechains.Count, summary.Sidechains.AgentTypes)
	}

	return nil
}
