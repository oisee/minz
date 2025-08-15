// MCP Server implementation for MinZ AI Colleague
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Message represents a JSON-RPC message
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Server implements the MCP server
type Server struct {
	name    string
	version string
	handler Handler
	reader  *bufio.Reader
	writer  io.Writer
}

// Handler processes MCP requests
type Handler interface {
	HandleRequest(ctx context.Context, method string, params json.RawMessage) (interface{}, error)
}

// NewServer creates a new MCP server
func NewServer(name, version string, handler Handler) *Server {
	return &Server{
		name:    name,
		version: version,
		handler: handler,
		reader:  bufio.NewReader(os.Stdin),
		writer:  os.Stdout,
	}
}

// Start begins processing messages from stdio
func (s *Server) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := s.readMessage()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				continue
			}

			if msg.Method != "" {
				response := s.handleMessage(ctx, msg)
				if err := s.writeMessage(response); err != nil {
					// Continue processing
				}
			}
		}
	}
}

func (s *Server) readMessage() (*Message, error) {
	line, err := s.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

func (s *Server) writeMessage(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(s.writer, "%s\n", data)
	return err
}

func (s *Server) handleMessage(ctx context.Context, msg *Message) *Message {
	response := &Message{
		JSONRPC: "2.0",
		ID:      msg.ID,
	}

	// Handle standard MCP methods
	switch msg.Method {
	case "initialize":
		response.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    s.name,
				"version": s.version,
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"list": true,
				},
			},
		}
	case "tools/list":
		response.Result = map[string]interface{}{
			"tools": s.getTools(),
		}
	case "tools/call":
		var params map[string]interface{}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			response.Error = &Error{
				Code:    -32602,
				Message: "Invalid params",
			}
		} else {
			result, err := s.handler.HandleRequest(ctx, params["name"].(string), msg.Params)
			if err != nil {
				response.Error = &Error{
					Code:    -32603,
					Message: err.Error(),
				}
			} else {
				response.Result = result
			}
		}
	default:
		response.Error = &Error{
			Code:    -32601,
			Message: "Method not found",
		}
	}

	return response
}

func (s *Server) getTools() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "ask_ai",
			"description": "Ask an AI colleague for advice on MinZ compiler development",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "Question to ask the AI",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Optional context (code, errors, etc)",
					},
				},
				"required": []string{"question"},
			},
		},
		{
			"name":        "analyze_parser",
			"description": "Analyze parser issues and suggest fixes",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"parser_type": map[string]interface{}{
						"type":        "string",
						"description": "Parser type: antlr or tree-sitter",
						"enum":        []string{"antlr", "tree-sitter"},
					},
					"code": map[string]interface{}{
						"type":        "string",
						"description": "MinZ code that fails to parse",
					},
					"error": map[string]interface{}{
						"type":        "string",
						"description": "Error message from parser",
					},
				},
				"required": []string{"parser_type", "code"},
			},
		},
		{
			"name":        "compare_approaches",
			"description": "Compare different implementation approaches",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"approach1": map[string]interface{}{
						"type":        "string",
						"description": "First approach description or code",
					},
					"approach2": map[string]interface{}{
						"type":        "string",
						"description": "Second approach description or code",
					},
					"criteria": map[string]interface{}{
						"type":        "string",
						"description": "Comparison criteria (performance, maintainability, etc)",
					},
				},
				"required": []string{"approach1", "approach2"},
			},
		},
	}
}