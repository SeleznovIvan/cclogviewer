// Package main provides the entry point for the cclogviewer MCP server.
// This server exposes Claude Code session logs via the Model Context Protocol,
// allowing Claude Code agents to analyze and query session history.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/brads3290/cclogviewer/internal/mcp"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Parse flags
	showVersion := flag.Bool("version", false, "Show version information")
	claudeDir := flag.String("claude-dir", "", "Path to Claude directory (default: ~/.claude)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cclogviewer-mcp %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if *debug {
		os.Setenv("DEBUG", "1")
		log.SetOutput(os.Stderr)
		log.Println("Debug mode enabled")
	}

	// Create services
	services := mcp.NewServices(*claudeDir)

	// Create and configure server
	server := mcp.NewServer()
	mcp.RegisterAllTools(server, services)

	// Run server
	if err := server.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
