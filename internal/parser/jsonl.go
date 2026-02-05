package parser

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"log"
	"math"
	"strings"

	"github.com/brads3290/cclogviewer/internal/constants"
	"github.com/brads3290/cclogviewer/internal/debug"
	"github.com/brads3290/cclogviewer/internal/models"
)

// ReadJSONLFile reads a JSONL file and returns a slice of LogEntry.
// It also automatically loads any subagent files from the {session_id}/subagents/ directory.
func ReadJSONLFile(filename string) ([]models.LogEntry, error) {
	// Read main session file
	entries, err := readSingleJSONLFile(filename)
	if err != nil {
		return nil, err
	}

	// Try to load subagent files
	subagentEntries, err := loadSubagentFiles(filename)
	if err != nil {
		if debug.Enabled {
			log.Printf("Note: Could not load subagent files: %v", err)
		}
		// Continue without subagent entries - this is not a fatal error
	} else if len(subagentEntries) > 0 {
		if debug.Enabled {
			log.Printf("Loaded %d entries from subagent files", len(subagentEntries))
		}
		entries = append(entries, subagentEntries...)
	}

	return entries, nil
}

// readSingleJSONLFile reads a single JSONL file and returns a slice of LogEntry
func readSingleJSONLFile(filename string) ([]models.LogEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []models.LogEntry
	scanner := bufio.NewScanner(file)
	// Set buffer with no maximum size limit
	buf := make([]byte, 0, constants.DefaultScannerBufferSize)
	scanner.Buffer(buf, math.MaxInt)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var entry models.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			if debug.Enabled {
				log.Printf("Error parsing line %d: %v", lineNum, err)
			}
			continue
		}

		// Skip summary messages
		if entry.Type == constants.EntryTypeSummary {
			if debug.Enabled {
				log.Printf("Skipping summary message at line %d", lineNum)
			}
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// loadSubagentFiles loads all subagent files from the {session_id}/subagents/ directory.
// In newer Claude Code versions, subagent/sidechain logs are stored in separate files.
func loadSubagentFiles(mainSessionFile string) ([]models.LogEntry, error) {
	// Extract session ID from filename (e.g., "abc123.jsonl" -> "abc123")
	baseName := filepath.Base(mainSessionFile)
	sessionID := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// Check for subagents directory: {dir}/{session_id}/subagents/
	dir := filepath.Dir(mainSessionFile)
	subagentsDir := filepath.Join(dir, sessionID, "subagents")

	// Check if subagents directory exists
	info, err := os.Stat(subagentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No subagents directory - this is normal for older sessions
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	if debug.Enabled {
		log.Printf("Found subagents directory: %s", subagentsDir)
	}

	// Read all agent-*.jsonl files from the subagents directory
	pattern := filepath.Join(subagentsDir, "agent-*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	if debug.Enabled {
		log.Printf("Found %d subagent files", len(matches))
	}

	var allEntries []models.LogEntry
	for _, agentFile := range matches {
		entries, err := readSingleJSONLFile(agentFile)
		if err != nil {
			if debug.Enabled {
				log.Printf("Error reading subagent file %s: %v", agentFile, err)
			}
			continue
		}
		if debug.Enabled {
			log.Printf("Loaded %d entries from %s", len(entries), filepath.Base(agentFile))
		}
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}
