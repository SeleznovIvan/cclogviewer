package commands

import (
	"flag"
	"fmt"
	"os"
)

// ContextCmd implements the context command.
type ContextCmd struct {
	Project           string
	Offset            int
	IncludeSidechains bool
	OutputPath        string
}

func (c *ContextCmd) Name() string {
	return "context"
}

func (c *ContextCmd) Description() string {
	return "Get logs around a specific entry by UUID"
}

func (c *ContextCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.Project, "project", "", "Project name/path (optional)")
	fs.IntVar(&c.Offset, "offset", -3, "Direction and count: negative = before target, positive = after target")
	fs.BoolVar(&c.IncludeSidechains, "include-sidechains", true, "Include sidechain (agent) conversations")
	fs.StringVar(&c.OutputPath, "output", "", "File path to save the logs as JSON")
}

func (c *ContextCmd) Run(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("session ID and UUID are required\nUsage: cclogviewer context <session-id> <uuid> [flags]")
	}

	sessionID := args[0]
	targetUUID := args[1]

	logs, err := ctx.Services.Session.GetLogsAroundEntry(sessionID, targetUUID, c.Project, c.Offset, c.IncludeSidechains)
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
	out.PrintLine("Context around entry: %s", logs.TargetUUID)
	out.PrintLine("Session: %s", logs.SessionID)
	out.PrintLine("Project: %s", logs.Project)
	out.PrintLine("Target Index: %d / %d", logs.TargetIndex, logs.TotalCount)
	out.PrintLine("Offset: %d\n", logs.Offset)

	for _, e := range logs.Entries {
		marker := " "
		if e.Offset == 0 {
			marker = ">"
		}

		roleStr := e.Role
		if e.ToolName != "" {
			roleStr = fmt.Sprintf("%s (%s)", e.Role, e.ToolName)
		}

		out.PrintLine("%s [%+d] %s %s", marker, e.Offset, e.Timestamp, roleStr)
		out.PrintLine("      %s", Truncate(e.Content, 80))
		if e.IsError {
			out.PrintLine("      [ERROR]")
		}
		out.PrintLine("")
	}

	return nil
}
