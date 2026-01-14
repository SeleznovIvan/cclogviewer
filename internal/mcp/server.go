package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

const (
	// Protocol version
	ProtocolVersion = "2024-11-05"
	ServerName      = "cclogviewer-mcp"
	ServerVersion   = "1.0.0"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      interface{}      `json:"id,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *JSONRPCError    `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Tool represents an MCP tool that can be called.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Execute(args map[string]interface{}) (interface{}, error)
}

// ToolInfo represents tool metadata for listing.
type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Server is the MCP server implementation.
type Server struct {
	tools    map[string]Tool
	mu       sync.RWMutex
	input    io.Reader
	output   io.Writer
	debug    bool
}

// NewServer creates a new MCP server.
func NewServer() *Server {
	return &Server{
		tools:  make(map[string]Tool),
		input:  os.Stdin,
		output: os.Stdout,
		debug:  os.Getenv("DEBUG") != "",
	}
}

// RegisterTool registers a tool with the server.
func (s *Server) RegisterTool(tool Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name()] = tool
}

// Run starts the MCP server main loop.
func (s *Server) Run() error {
	reader := bufio.NewReader(s.input)
	encoder := json.NewEncoder(s.output)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading input: %w", err)
		}

		if len(line) == 0 || (len(line) == 1 && line[0] == '\n') {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := s.errorResponse(nil, ParseError, "Parse error", err.Error())
			encoder.Encode(resp)
			continue
		}

		if s.debug {
			log.Printf("Received request: %s", string(line))
		}

		resp := s.handleRequest(&req)
		if resp != nil {
			if s.debug {
				respBytes, _ := json.Marshal(resp)
				log.Printf("Sending response: %s", string(respBytes))
			}
			encoder.Encode(resp)
		}
	}
}

func (s *Server) handleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	if req.JSONRPC != "2.0" {
		return s.errorResponse(req.ID, InvalidRequest, "Invalid Request", "jsonrpc must be 2.0")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return s.successResponse(req.ID, map[string]interface{}{})
	default:
		return s.errorResponse(req.ID, MethodNotFound, "Method not found", req.Method)
	}
}

func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	result := map[string]interface{}{
		"protocolVersion": ProtocolVersion,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    ServerName,
			"version": ServerVersion,
		},
	}
	return s.successResponse(req.ID, result)
}

func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]ToolInfo, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, ToolInfo{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}

	return s.successResponse(req.ID, map[string]interface{}{
		"tools": tools,
	})
}

func (s *Server) handleToolsCall(req *JSONRPCRequest) *JSONRPCResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, InvalidParams, "Invalid params", err.Error())
	}

	s.mu.RLock()
	tool, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		return s.errorResponse(req.ID, InvalidParams, "Tool not found", params.Name)
	}

	result, err := tool.Execute(params.Arguments)
	if err != nil {
		return s.errorResponse(req.ID, InternalError, "Tool execution failed", err.Error())
	}

	// Format result as MCP content
	content := []map[string]interface{}{
		{
			"type": "text",
			"text": formatResult(result),
		},
	}

	return s.successResponse(req.ID, map[string]interface{}{
		"content": content,
	})
}

func formatResult(result interface{}) string {
	bytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return string(bytes)
}

func (s *Server) successResponse(id interface{}, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func (s *Server) errorResponse(id interface{}, code int, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}
