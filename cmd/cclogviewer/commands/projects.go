package commands

import (
	"flag"
)

// ProjectsCmd implements the projects command.
type ProjectsCmd struct {
	SortBy string
}

func (c *ProjectsCmd) Name() string {
	return "projects"
}

func (c *ProjectsCmd) Description() string {
	return "List all Claude Code projects with session counts"
}

func (c *ProjectsCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.SortBy, "sort-by", "last_modified", "Sort by: last_modified, name, session_count")
}

func (c *ProjectsCmd) Run(ctx *Context, args []string) error {
	projects, err := ctx.Services.Project.ListProjects(c.SortBy)
	if err != nil {
		return err
	}

	out := NewOutputWriter(ctx.Output, ctx.Config.JSONOutput)

	if ctx.Config.JSONOutput {
		return out.WriteJSON(map[string]interface{}{
			"projects": projects,
			"total":    len(projects),
		})
	}

	// Human-readable output
	if len(projects) == 0 {
		out.PrintLine("No projects found")
		return nil
	}

	headers := []string{"Name", "Path", "Sessions", "Last Modified"}
	var rows [][]string
	for _, p := range projects {
		rows = append(rows, []string{
			p.Name,
			Truncate(p.Path, 50),
			FormatNumber(p.SessionCount),
			FormatTime(p.LastModified),
		})
	}
	out.WriteTable(headers, rows)

	return nil
}
