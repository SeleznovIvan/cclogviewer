package commands

import (
	"flag"
	"fmt"

	"github.com/brads3290/cclogviewer/internal/service"
)

// HTMLCmd implements the html command.
type HTMLCmd struct {
	SessionID   string
	FilePath    string
	Project     string
	OutputPath  string
	OpenBrowser bool
}

func (c *HTMLCmd) Name() string {
	return "html"
}

func (c *HTMLCmd) Description() string {
	return "Generate interactive HTML from session or file"
}

func (c *HTMLCmd) Setup(fs *flag.FlagSet) {
	fs.StringVar(&c.SessionID, "session", "", "Session UUID to generate HTML for")
	fs.StringVar(&c.FilePath, "file", "", "Direct path to a JSONL log file")
	fs.StringVar(&c.Project, "project", "", "Project name/path (only used with --session)")
	fs.StringVar(&c.OutputPath, "output", "", "Output HTML file path (creates temp file if not specified)")
	fs.BoolVar(&c.OpenBrowser, "open", false, "Open the generated HTML file in browser")
}

func (c *HTMLCmd) Run(ctx *Context, args []string) error {
	// Check for positional argument as file path
	if len(args) > 0 && c.FilePath == "" && c.SessionID == "" {
		c.FilePath = args[0]
	}

	if c.SessionID == "" && c.FilePath == "" {
		return fmt.Errorf("either --session or --file (or a file path argument) is required\nUsage: cclogviewer html [--session <id> | --file <path> | <path>] [flags]")
	}

	out := NewOutputWriter(ctx.Output, ctx.Config.JSONOutput)

	var result interface{}
	var err error

	if c.FilePath != "" {
		// Generate from file
		result, err = ctx.Services.Session.GenerateHTMLFromFile(c.FilePath, c.OutputPath, c.OpenBrowser)
	} else {
		// Generate from session
		result, err = ctx.Services.Session.GenerateSessionHTML(c.SessionID, c.Project, c.OutputPath, c.OpenBrowser)
	}

	if err != nil {
		return err
	}

	if ctx.Config.JSONOutput {
		return out.WriteJSON(result)
	}

	// Human-readable output
	if htmlResult, ok := result.(*service.HTMLGenerationResult); ok {
		out.PrintLine("HTML generated: %s", htmlResult.OutputPath)
		if htmlResult.OpenedBrowser {
			out.PrintLine("Opened in browser")
		}
	}

	return nil
}
