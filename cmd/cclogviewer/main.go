package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/brads3290/cclogviewer/cmd/cclogviewer/commands"
	"github.com/brads3290/cclogviewer/internal/browser"
	"github.com/brads3290/cclogviewer/internal/constants"
	debugpkg "github.com/brads3290/cclogviewer/internal/debug"
	"github.com/brads3290/cclogviewer/internal/models"
	"github.com/brads3290/cclogviewer/internal/parser"
	"github.com/brads3290/cclogviewer/internal/processor"
	"github.com/brads3290/cclogviewer/internal/renderer"
)

var (
	// Version can be set by ldflags during build
	Version = constants.DefaultVersion
	// BuildTime can be set by ldflags during build
	BuildTime = ""
)

func main() {
	// Check for help/version flags first (before legacy mode detection)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h", "help":
			commands.DefaultRegistry.PrintHelp(os.Stdout)
			os.Exit(0)
		case "--version", "-v", "version":
			printVersion()
			os.Exit(0)
		}
	}

	// If no arguments, print help
	if len(os.Args) < 2 {
		commands.DefaultRegistry.PrintHelp(os.Stdout)
		os.Exit(0)
	}

	// Check if running in legacy mode (first arg starts with - and is a legacy flag)
	if len(os.Args) > 1 && isLegacyFlag(os.Args[1]) {
		runLegacyMode()
		return
	}

	// Run subcommand mode
	if err := runSubcommandMode(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// isLegacyFlag checks if the argument is a legacy mode flag.
func isLegacyFlag(arg string) bool {
	legacyFlags := []string{"-input", "-output", "-open", "-debug", "-contextsize"}
	for _, f := range legacyFlags {
		if arg == f || strings.HasPrefix(arg, f+"=") {
			return true
		}
	}
	return false
}

// runSubcommandMode handles the new subcommand-based CLI.
func runSubcommandMode() error {
	cmdName := os.Args[1]

	// Look up command
	cmd, ok := commands.DefaultRegistry.Get(cmdName)
	if !ok {
		return fmt.Errorf("unknown command: %s\nRun 'cclogviewer --help' for usage", cmdName)
	}

	// Parse global flags and command flags
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)

	// Global flags
	var config commands.Config
	var homeDir string
	if hd, err := os.UserHomeDir(); err == nil {
		homeDir = filepath.Join(hd, ".claude")
	}

	fs.StringVar(&config.ClaudeDir, "claude-dir", homeDir, "Path to Claude directory")
	fs.BoolVar(&config.JSONOutput, "json", false, "Output in JSON format")
	fs.BoolVar(&config.Debug, "debug", false, "Enable debug logging")

	// Command-specific flags
	cmd.Setup(fs)

	// Custom usage
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "cclogviewer %s - %s\n\n", cmd.Name(), cmd.Description())
		fmt.Fprintf(os.Stderr, "Usage: cclogviewer %s [flags] [arguments]\n\n", cmd.Name())
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
	}

	// Parse flags (skip program name and command name)
	if err := fs.Parse(os.Args[2:]); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Enable debug mode
	if config.Debug {
		debugpkg.Enabled = true
	}

	// Create context
	ctx := commands.NewContext(&config)

	// Run command with remaining arguments
	return cmd.Run(ctx, fs.Args())
}

// runLegacyMode handles the original -input/-output flag-based CLI.
func runLegacyMode() {
	var inputFile, outputFile string
	var openBrowser, showVersion, showContextSize bool
	flag.StringVar(&inputFile, "input", "", "Input JSONL file path")
	flag.StringVar(&outputFile, "output", "", "Output HTML file path (optional)")
	flag.BoolVar(&openBrowser, "open", false, "Open the generated HTML file in browser")
	flag.BoolVar(&debugpkg.Enabled, "debug", false, "Enable debug logging")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showContextSize, "contextsize", false, "Print the conversation size from the last assistant message")
	flag.Parse()

	if showVersion {
		printVersion()
		os.Exit(0)
	}

	if inputFile == "" {
		fmt.Fprintln(os.Stderr, "Please provide an input file using -input flag")
		fmt.Fprintln(os.Stderr, "\nUsage:")
		fmt.Fprintln(os.Stderr, "  Legacy mode:    cclogviewer -input <file.jsonl> [flags]")
		fmt.Fprintln(os.Stderr, "  Subcommand mode: cclogviewer <command> [flags] [arguments]")
		fmt.Fprintln(os.Stderr, "\nRun 'cclogviewer --help' for full usage information")
		os.Exit(1)
	}

	// If no output file specified, create a temp file and auto-open it
	autoOpen := false
	if outputFile == "" {
		// Generate unique filename based on input file and timestamp
		baseName := filepath.Base(inputFile)
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
		timestamp := time.Now().Format(constants.TempFileTimestampFormat)
		outputFile = filepath.Join(os.TempDir(), fmt.Sprintf(constants.TempFileNameFormat, baseName, timestamp))
		autoOpen = true
	}

	entries, err := parser.ReadJSONLFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	processed := processor.ProcessEntries(entries)

	// If -contextsize flag is set, print the conversation size and exit
	if showContextSize {
		// Find the last assistant message
		var lastAssistantTokens int
		var foundAssistant bool

		// Traverse all processed entries to find the last assistant message
		var findLastAssistant func([]*models.ProcessedEntry)
		findLastAssistant = func(entries []*models.ProcessedEntry) {
			for _, entry := range entries {
				if entry.Role == constants.RoleAssistant && !entry.IsSidechain {
					lastAssistantTokens = entry.TotalTokens
					foundAssistant = true
				}
				// Check tool calls for nested entries
				for _, toolCall := range entry.ToolCalls {
					if toolCall.Result != nil && toolCall.Result.Role == constants.RoleAssistant {
						lastAssistantTokens = toolCall.Result.TotalTokens
						foundAssistant = true
					}
					// Check Task entries
					findLastAssistant(toolCall.TaskEntries)
				}
			}
		}

		findLastAssistant(processed)

		if foundAssistant {
			fmt.Println(lastAssistantTokens)
		} else {
			fmt.Println(0)
		}
		os.Exit(0)
	}

	err = renderer.GenerateHTML(processed, outputFile, debugpkg.Enabled)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating HTML: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated %s\n", outputFile)

	// Open browser if -open flag was set OR if output was auto-generated
	if openBrowser || autoOpen {
		if err := browser.OpenInBrowser(outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not open browser: %v\n", err)
		}
	}
}

// printVersion prints version information.
func printVersion() {
	version := Version
	if version == "" {
		// Try to get version from build info
		if info, ok := debug.ReadBuildInfo(); ok {
			version = info.Main.Version
			if version == constants.DevelopmentVersionString {
				version = constants.DevVersionString
			}
		} else {
			version = constants.UnknownVersionString
		}
	}

	fmt.Printf("cclogviewer version %s", version)
	if BuildTime != "" {
		fmt.Printf(" (built %s)", BuildTime)
	}
	fmt.Println()
}
