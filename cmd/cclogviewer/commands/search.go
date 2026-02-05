package commands

import (
	"flag"

	"github.com/brads3290/cclogviewer/internal/service"
)

// SearchCmd implements the search command.
type SearchCmd struct {
	Query             string
	ToolName          string
	Role              string
	Project           string
	Days              int
	IncludeSidechains bool
	Limit             int
}

func (c *SearchCmd) Name() string {
	return "search"
}

func (c *SearchCmd) Description() string {
	return "Search across sessions by content, tool usage, or role"
}

func (c *SearchCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.Query, "query", "", "Text to search for in log content")
	fs.StringVar(&c.ToolName, "tool", "", "Filter by tool name (e.g., 'Bash', 'Edit')")
	fs.StringVar(&c.Role, "role", "", "Filter by message role (user, assistant)")
	fs.StringVar(&c.Project, "project", "", "Limit search to a specific project")
	fs.IntVar(&c.Days, "days", 0, "Only search sessions from the last N days")
	fs.BoolVar(&c.IncludeSidechains, "include-sidechains", true, "Search in sidechain conversations too")
	fs.IntVar(&c.Limit, "limit", 50, "Maximum results to return")
}

func (c *SearchCmd) Run(ctx *Context, args []string) error {
	criteria := service.SearchCriteria{
		Query:             c.Query,
		ToolName:          c.ToolName,
		Role:              c.Role,
		Project:           c.Project,
		Days:              c.Days,
		IncludeSidechains: c.IncludeSidechains,
		Limit:             c.Limit,
	}

	results, err := ctx.Services.Search.Search(criteria)
	if err != nil {
		return err
	}

	out := NewOutputWriter(ctx.Output, ctx.Config.JSONOutput)

	if ctx.Config.JSONOutput {
		return out.WriteJSON(results)
	}

	// Human-readable output
	if len(results.Results) == 0 {
		out.PrintLine("No results found")
		return nil
	}

	out.PrintLine("Found %d results:\n", results.TotalMatches)

	headers := []string{"Session ID", "Project", "Role", "Tool", "Content"}
	var rows [][]string
	for _, r := range results.Results {
		rows = append(rows, []string{
			Truncate(r.SessionID, 20),
			Truncate(r.Project, 15),
			r.Role,
			r.ToolName,
			Truncate(r.ContentSnippet, 50),
		})
	}
	out.WriteTable(headers, rows)

	return nil
}
