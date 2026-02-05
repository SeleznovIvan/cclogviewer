package models

import "time"

// Project represents a Claude Code project with its log directory.
type Project struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	EncodedPath  string    `json:"encoded_path"`
	SessionCount int       `json:"session_count,omitempty"`
	LastModified time.Time `json:"last_modified"`
}
