package commands

import (
	"flag"
	"fmt"
	"os"
)

// ErrorsCmd implements the errors command.
type ErrorsCmd struct {
	AgentID           string
	Project           string
	IncludeSidechains bool
	Limit             int
	OutputPath        string
}

func (c *ErrorsCmd) Name() string {
	return "errors"
}

func (c *ErrorsCmd) Description() string {
	return "Get errors and blockers from a session"
}

func (c *ErrorsCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.AgentID, "agent-id", "", "Specific subagent ID to analyze")
	fs.StringVar(&c.Project, "project", "", "Project name/path (optional)")
	fs.BoolVar(&c.IncludeSidechains, "include-sidechains", true, "Include sidechain (agent) conversations in analysis")
	fs.IntVar(&c.Limit, "limit", 20, "Maximum number of errors to return")
	fs.StringVar(&c.OutputPath, "output", "", "File path to save the errors as JSON")
}

func (c *ErrorsCmd) Run(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("session ID is required\nUsage: cclogviewer errors <session-id> [flags]")
	}

	sessionID := args[0]
	errors, err := ctx.Services.Session.GetSessionErrors(sessionID, c.AgentID, c.Project, c.IncludeSidechains, c.Limit)
	if err != nil {
		return err
	}

	if errors == nil {
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
		if err := fileOut.WriteJSON(errors); err != nil {
			return fmt.Errorf("failed to write errors: %w", err)
		}

		out.PrintLine("Errors saved to: %s", c.OutputPath)
		return nil
	}

	// Output to stdout
	if ctx.Config.JSONOutput {
		return out.WriteJSON(errors)
	}

	// Human-readable output
	out.PrintLine("Session Errors: %s", errors.SessionID)
	out.PrintLine("Total Errors: %d\n", errors.TotalErrors)

	if len(errors.Errors) == 0 {
		out.PrintLine("No errors found")
		return nil
	}

	for i, e := range errors.Errors {
		out.PrintLine("%d. [%s] %s", i+1, e.Type, e.Timestamp)
		if e.ToolName != "" {
			out.PrintLine("   Tool: %s", e.ToolName)
		}
		out.PrintLine("   Message: %s", Truncate(e.Message, 100))
		out.PrintLine("   UUID: %s", e.UUID)
		out.PrintLine("   Entry Index: %d", e.EntryIndex)
		out.PrintLine("")
	}

	return nil
}
