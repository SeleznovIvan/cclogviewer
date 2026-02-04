package commands

import (
	"flag"
	"fmt"
	"os"
)

// LogsCmd implements the logs command.
type LogsCmd struct {
	Project           string
	IncludeSidechains bool
	OutputPath        string
}

func (c *LogsCmd) Name() string {
	return "logs"
}

func (c *LogsCmd) Description() string {
	return "Get full processed logs for a session"
}

func (c *LogsCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.Project, "project", "", "Project name/path (optional if session_id is globally unique)")
	fs.BoolVar(&c.IncludeSidechains, "include-sidechains", true, "Include sidechain (agent) conversations")
	fs.StringVar(&c.OutputPath, "output", "", "File path to save the logs as JSON")
}

func (c *LogsCmd) Run(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("session ID is required\nUsage: cclogviewer logs <session-id> [flags]")
	}

	sessionID := args[0]
	logs, err := ctx.Services.Session.GetSessionLogs(sessionID, c.Project, c.IncludeSidechains)
	if err != nil {
		return err
	}

	if logs == nil {
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
		if err := fileOut.WriteJSON(logs); err != nil {
			return fmt.Errorf("failed to write logs: %w", err)
		}

		out.PrintLine("Logs saved to: %s", c.OutputPath)
		return nil
	}

	// Output to stdout
	if ctx.Config.JSONOutput {
		return out.WriteJSON(logs)
	}

	// Human-readable output
	out.PrintLine("Session: %s", logs.SessionID)
	out.PrintLine("Project: %s", logs.Project)
	out.PrintLine("Entries: %d", len(logs.Entries))

	if logs.TokenStats != nil {
		out.PrintLine("\nToken Stats:")
		out.PrintKeyValue("Input", FormatNumber(logs.TokenStats.TotalInput))
		out.PrintKeyValue("Output", FormatNumber(logs.TokenStats.TotalOutput))
		out.PrintKeyValue("Cache Read", FormatNumber(logs.TokenStats.CacheRead))
		out.PrintKeyValue("Cache Creation", FormatNumber(logs.TokenStats.CacheCreation))
	}

	out.PrintLine("\nUse --json flag to see full log content")

	return nil
}
