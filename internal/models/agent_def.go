package models

// AgentDefinition represents a custom agent definition from .md files.
type AgentDefinition struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Tools       []string `json:"tools,omitempty" yaml:"tools"`
	Model       string   `json:"model,omitempty" yaml:"model"`
	Color       string   `json:"color,omitempty" yaml:"color"`
	Scope       string   `json:"scope"` // "global" or "project"
	FilePath    string   `json:"file_path"`
}
