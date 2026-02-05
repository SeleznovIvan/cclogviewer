// Package commands provides the CLI command infrastructure for cclogviewer.
package commands

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/brads3290/cclogviewer/internal/service"
)

// Command represents a CLI subcommand.
type Command interface {
	// Name returns the command name (e.g., "projects", "sessions").
	Name() string
	// Description returns a short description for help text.
	Description() string
	// Setup configures command-specific flags.
	Setup(fs *flag.FlagSet)
	// Run executes the command with the given context and arguments.
	Run(ctx *Context, args []string) error
}

// Config holds global CLI configuration.
type Config struct {
	// ClaudeDir is the path to the Claude directory (default: ~/.claude).
	ClaudeDir string
	// JSONOutput indicates whether to output in JSON format.
	JSONOutput bool
	// Debug enables debug logging.
	Debug bool
}

// Context provides the execution context for commands.
type Context struct {
	// Config contains global configuration.
	Config *Config
	// Services provides access to all services.
	Services *service.Services
	// Output is the writer for command output (default: os.Stdout).
	Output io.Writer
	// ErrOutput is the writer for error output (default: os.Stderr).
	ErrOutput io.Writer
}

// NewContext creates a new Context with the given config.
func NewContext(config *Config) *Context {
	return &Context{
		Config:    config,
		Services:  service.NewServices(config.ClaudeDir),
		Output:    os.Stdout,
		ErrOutput: os.Stderr,
	}
}

// Registry holds all registered commands.
type Registry struct {
	commands map[string]Command
	order    []string
}

// NewRegistry creates a new command registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	name := cmd.Name()
	if _, exists := r.commands[name]; !exists {
		r.order = append(r.order, name)
	}
	r.commands[name] = cmd
}

// Get returns a command by name.
func (r *Registry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Commands returns all registered commands in registration order.
func (r *Registry) Commands() []Command {
	result := make([]Command, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.commands[name])
	}
	return result
}

// PrintHelp prints the help text for all commands.
func (r *Registry) PrintHelp(w io.Writer) {
	fmt.Fprintln(w, "cclogviewer - Claude Code Log Viewer")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "    cclogviewer <command> [flags] [arguments]")
	fmt.Fprintln(w, "    cclogviewer -input <file.jsonl> [flags]    (legacy mode)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "COMMANDS:")
	for _, cmd := range r.Commands() {
		fmt.Fprintf(w, "    %-14s %s\n", cmd.Name(), cmd.Description())
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "GLOBAL FLAGS:")
	fmt.Fprintln(w, "    --json         Output results in JSON format (default: human-readable)")
	fmt.Fprintln(w, "    --claude-dir   Path to Claude directory (default: ~/.claude)")
	fmt.Fprintln(w, "    --debug        Enable debug logging")
	fmt.Fprintln(w, "    --help, -h     Show help for command")
	fmt.Fprintln(w, "    --version, -v  Show version information")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "    # List all projects")
	fmt.Fprintln(w, "    cclogviewer projects")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "    # List recent sessions for a project")
	fmt.Fprintln(w, "    cclogviewer sessions my-project --days 7 --limit 10")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "    # Get session summary")
	fmt.Fprintln(w, "    cclogviewer summary abc123-def456 --json")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "    # Search for tool usage across sessions")
	fmt.Fprintln(w, "    cclogviewer search --tool Bash --project my-project --days 30")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "    # Generate HTML from session")
	fmt.Fprintln(w, "    cclogviewer html --session abc123-def456 --open")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "    # Legacy mode (backward compatible)")
	fmt.Fprintln(w, "    cclogviewer -input session.jsonl -output report.html -open")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Use \"cclogviewer <command> --help\" for detailed help on any command.")
}

// DefaultRegistry is the global command registry with all commands pre-registered.
var DefaultRegistry = NewRegistry()

// RegisterAll registers all CLI commands with the given registry.
func RegisterAll(r *Registry) {
	r.Register(&ProjectsCmd{})
	r.Register(&SessionsCmd{})
	r.Register(&AgentsCmd{})
	r.Register(&AgentSessionsCmd{})
	r.Register(&SearchCmd{})
	r.Register(&LogsCmd{})
	r.Register(&SummaryCmd{})
	r.Register(&ToolsCmd{})
	r.Register(&ErrorsCmd{})
	r.Register(&TimelineCmd{})
	r.Register(&StatsCmd{})
	r.Register(&ContextCmd{})
	r.Register(&HTMLCmd{})
}

func init() {
	RegisterAll(DefaultRegistry)
}
