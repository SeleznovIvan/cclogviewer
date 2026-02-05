package commands

import (
	"flag"
	"fmt"
	"os"
)

// TimelineCmd implements the timeline command.
type TimelineCmd struct {
	AgentID           string
	Project           string
	IncludeSidechains bool
	Limit             int
	OutputPath        string
}

func (c *TimelineCmd) Name() string {
	return "timeline"
}

func (c *TimelineCmd) Description() string {
	return "Get condensed timeline of session events"
}

func (c *TimelineCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.AgentID, "agent-id", "", "Specific subagent ID to analyze")
	fs.StringVar(&c.Project, "project", "", "Project name/path (optional)")
	fs.BoolVar(&c.IncludeSidechains, "include-sidechains", true, "Include sidechain (agent) conversations in analysis")
	fs.IntVar(&c.Limit, "limit", 100, "Maximum number of timeline entries to return")
	fs.StringVar(&c.OutputPath, "output", "", "File path to save the timeline as JSON")
}

func (c *TimelineCmd) Run(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("session ID is required\nUsage: cclogviewer timeline <session-id> [flags]")
	}

	sessionID := args[0]
	timeline, err := ctx.Services.Session.GetSessionTimeline(sessionID, c.AgentID, c.Project, c.IncludeSidechains, c.Limit)
	if err != nil {
		return err
	}

	if timeline == nil {
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
		if err := fileOut.WriteJSON(timeline); err != nil {
			return fmt.Errorf("failed to write timeline: %w", err)
		}

		out.PrintLine("Timeline saved to: %s", c.OutputPath)
		return nil
	}

	// Output to stdout
	if ctx.Config.JSONOutput {
		return out.WriteJSON(timeline)
	}

	// Human-readable output
	out.PrintLine("Session Timeline: %s", timeline.SessionID)
	out.PrintLine("Total Entries: %d (showing %d)\n", timeline.TotalEntries, timeline.ReturnedEntries)

	headers := []string{"Step", "Time", "Role", "Type", "Tool/Summary", "Status"}
	var rows [][]string
	for _, e := range timeline.Timeline {
		summary := e.Summary
		if e.Tool != "" {
			summary = e.Tool + ": " + summary
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", e.Step),
			e.Timestamp,
			e.Role,
			e.Type,
			Truncate(summary, 40),
			e.Status,
		})
	}
	out.WriteTable(headers, rows)

	return nil
}
