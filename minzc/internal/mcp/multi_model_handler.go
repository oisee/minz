package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Model represents an AI model configuration
type Model struct {
	Name       string
	Endpoint   string
	APIKey     string
	Deployment string
	APIVersion string
}

// MultiModelHandler handles multiple AI models
type MultiModelHandler struct {
	models map[string]*Model
}

// NewMultiModelHandler creates a handler with multiple models configured
func NewMultiModelHandler() *MultiModelHandler {
	h := &MultiModelHandler{
		models: make(map[string]*Model),
	}

	// Get base configuration from environment
	baseEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	apiKey := os.Getenv("AZURE_OPENAI_API_KEY")

	if baseEndpoint == "" || apiKey == "" {
		// Return empty handler if not configured
		return h
	}

	// Configure available models based on your provided URLs
	// Get API key from environment variable for security
	modelApiKey := os.Getenv("AZURE_OPENAI_API_KEY")
	if modelApiKey == "" {
		// Fall back to a default or return empty handler
		return h
	}
	
	h.models["gpt4"] = &Model{
		Name:       "GPT-4.1",
		Endpoint:   "https://imqt.openai.azure.com",
		APIKey:     modelApiKey,
		Deployment: "gpt-4.1",
		APIVersion: "2025-01-01-preview",
	}

	h.models["gpt5"] = &Model{
		Name:       "gpt-5",
		Endpoint:   "https://imqt.openai.azure.com",
		APIKey:     modelApiKey,
		Deployment: "gpt-5",
		APIVersion: "2025-04-01-preview",
	}

	h.models["o4_mini"] = &Model{
		Name:       "o4-mini",
		Endpoint:   "https://imqt.openai.azure.com",
		APIKey:     modelApiKey,
		Deployment: "o4-mini",
		APIVersion: "2025-01-01-preview",
	}

	h.models["model_router"] = &Model{
		Name:       "model-router",
		Endpoint:   "https://imqt.openai.azure.com",
		APIKey:     modelApiKey,
		Deployment: "model-router",
		APIVersion: "2025-01-01-preview",
	}

	return h
}

// HandleRequest processes tool calls for all models
func (h *MultiModelHandler) HandleRequest(ctx context.Context, toolName string, params json.RawMessage) (interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(params, &result); err != nil {
		return nil, err
	}

	arguments := result["arguments"].(map[string]interface{})

	// Handle model-specific ask tools
	switch toolName {
	case "ask_gpt4", "ask_gpt5", "ask_o4_mini", "ask_model_router":
		modelName := strings.TrimPrefix(toolName, "ask_")
		return h.askModel(ctx, modelName, arguments)

	case "ask_ai_with_context":
		// Enhanced version with source file grounding
		return h.askWithSourceContext(ctx, arguments)

	case "brainstorm_semantic_fixes":
		// Specialized for semantic analyzer issues
		return h.brainstormSemanticFixes(ctx, arguments)

	default:
		// Fall back to original handler for backward compatibility
		return h.handleLegacyTool(ctx, toolName, arguments)
	}
}

func (h *MultiModelHandler) askModel(ctx context.Context, modelName string, args map[string]interface{}) (interface{}, error) {
	model, exists := h.models[modelName]
	if !exists {
		return nil, fmt.Errorf("model %s not configured", modelName)
	}

	question := args["question"].(string)
	context := ""
	if c, ok := args["context"].(string); ok {
		context = c
	}

	prompt := fmt.Sprintf("You are an expert MinZ compiler developer. Question: %s", question)
	if context != "" {
		prompt += fmt.Sprintf("\n\nContext:\n%s", context)
	}

	response, err := h.callModel(ctx, model, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("[%s]\n%s", model.Name, response),
			},
		},
	}, nil
}

func (h *MultiModelHandler) askWithSourceContext(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	question := args["question"].(string)
	files := []string{}
	if f, ok := args["files"].([]interface{}); ok {
		for _, file := range f {
			files = append(files, file.(string))
		}
	}

	modelName := "gpt4" // Default
	if m, ok := args["model"].(string); ok {
		modelName = m
	}

	model, exists := h.models[modelName]
	if !exists {
		return nil, fmt.Errorf("model %s not configured", modelName)
	}

	// Build context with actual source files
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Question: " + question + "\n\n")

	for _, file := range files {
		content, err := h.readSourceFile(file)
		if err != nil {
			contextBuilder.WriteString(fmt.Sprintf("\n[Error reading %s: %v]\n", file, err))
			continue
		}
		contextBuilder.WriteString(fmt.Sprintf("\n=== Source: %s ===\n%s\n", file, content))
	}

	response, err := h.callModel(ctx, model, contextBuilder.String())
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": response,
			},
		},
	}, nil
}

func (h *MultiModelHandler) brainstormSemanticFixes(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Read semantic analyzer source - use GPT-4.1's 1M context capability
	analyzerPath := "/Users/alice/dev/minz-ts/minzc/pkg/semantic/analyzer.go"
	analyzerContent, err := h.readFullFile(analyzerPath)
	if err != nil {
		analyzerContent = "[Could not read analyzer.go]"
	}

	// Read AST definitions
	astPath := "/Users/alice/dev/minz-ts/minzc/pkg/ast/ast.go"
	astContent, err := h.readFullFile(astPath)
	if err != nil {
		astContent = "[Could not read ast.go]"
	}

	// Read parser for context
	parserPath := "/Users/alice/dev/minz-ts/minzc/pkg/parser/parser.go"
	parserContent, err := h.readFullFile(parserPath)
	if err != nil {
		parserContent = "[Could not read parser.go]"
	}

	// Prepare comprehensive prompt for GPT-4.1 with full context
	gpt4Prompt := fmt.Sprintf(`You are an expert compiler engineer analyzing MinZ semantic analyzer issues.

CURRENT STATUS:
- Parser success rate: 65%% (58/88 examples compile)
- Main blockers are in semantic analyzer, not parser
- Recent fix: If expressions with block statements now working

KEY REMAINING ISSUES:
1. Recursive functions - functions can't reference themselves during declaration
2. Lambda parameter types - fn(T) -> R syntax partially supported
3. Module imports - full path syntax not working (e.g., zx.screen.set_border())
4. Pattern matching - partially implemented

FULL SOURCE CODE CONTEXT:

=== Complete Semantic Analyzer (analyzer.go) ===
%s

=== Complete AST Definitions (ast.go) ===
%s

=== Parser Implementation (parser.go) ===
%s

Please provide:
1. Root cause analysis of why recursive functions fail
2. Specific code changes with exact line numbers
3. Implementation order (easiest to hardest)
4. Test cases to verify each fix
5. Estimated impact on success rate

Focus on fixes that will have maximum impact on compilation success rate.`, 
		analyzerContent, astContent, parserContent)

	// Use GPT-4.1 for comprehensive analysis with 1M context
	gpt4Model := h.models["gpt4"]
	gpt4Response := ""
	if gpt4Model != nil {
		if resp, err := h.callModel(ctx, gpt4Model, gpt4Prompt); err == nil {
			gpt4Response = resp
		}
	}

	// Ask other models with focused questions
	responses := make(map[string]string)
	
	// GPT-5 for innovative solutions
	if gpt5Model := h.models["gpt5"]; gpt5Model != nil {
		gpt5Prompt := `Given that MinZ is a Z80 compiler with modern features, what innovative approach would you suggest for implementing recursive function support? Consider that the symbol table needs two-pass registration.`
		if resp, err := h.callModel(ctx, gpt5Model, gpt5Prompt); err == nil {
			responses["gpt5"] = resp
		}
	}

	// O4-Mini for deep analysis
	if o4Model := h.models["o4_mini"]; o4Model != nil {
		o4Prompt := `Step by step, analyze why recursive functions fail in the semantic analyzer. The issue is that functions can't reference themselves during their own declaration. What's the minimal fix needed?`
		if resp, err := h.callModel(ctx, o4Model, o4Prompt); err == nil {
			responses["o4_mini"] = resp
		}
	}

	// Combine responses
	var combined strings.Builder
	combined.WriteString("=== Multi-Model Analysis Results ===\n\n")
	
	if gpt4Response != "" {
		combined.WriteString("--- GPT-4.1 Comprehensive Analysis (with full context) ---\n")
		combined.WriteString(gpt4Response)
		combined.WriteString("\n\n")
	}
	
	for modelName, response := range responses {
		combined.WriteString(fmt.Sprintf("--- %s Focused Analysis ---\n%s\n\n", modelName, response))
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": combined.String(),
			},
		},
	}, nil
}

// readFullFile reads entire file without truncation for GPT-4.1's 1M context
func (h *MultiModelHandler) readFullFile(path string) (string, error) {
	if !strings.Contains(path, "minz") {
		return "", fmt.Errorf("can only read MinZ project files")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// For GPT-4.1 with 1M context, we can include much more
	const maxSizeFor1M = 500000 // ~500KB should be safe for 1M tokens
	if len(content) > maxSizeFor1M {
		content = content[:maxSizeFor1M]
		content = append(content, []byte("\n... [truncated for context limit]")...)
	}

	return string(content), nil
}

func (h *MultiModelHandler) readSourceFile(path string) (string, error) {
	// Ensure we're reading from the MinZ project
	if !strings.Contains(path, "minz") {
		return "", fmt.Errorf("can only read MinZ project files")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Limit size to avoid huge contexts
	const maxSize = 10000
	if len(content) > maxSize {
		content = content[:maxSize]
		content = append(content, []byte("\n... [truncated for context limit]")...)
	}

	return string(content), nil
}

func (h *MultiModelHandler) callModel(ctx context.Context, model *Model, prompt string) (string, error) {
	if model.Endpoint == "" || model.APIKey == "" {
		return "Model not configured", nil
	}

	endpoint := model.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		strings.TrimSuffix(endpoint, "/"), model.Deployment, model.APIVersion)

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an expert MinZ compiler developer with deep knowledge of semantic analysis, type systems, and compiler architecture.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	
	// Use correct parameter based on model type
	// GPT-5 and O4 models use max_completion_tokens instead of max_tokens
	// They also don't support custom temperature values (only default of 1)
	if model.Name == "gpt-5" || model.Name == "o4-mini" {
		payload["max_completion_tokens"] = 3000
		// Don't set temperature - these models only support default (1)
	} else {
		payload["max_tokens"] = 3000
		payload["temperature"] = 0.7
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", model.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call %s: %w", model.Name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned status %d: %s", model.Name, resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					return content, nil
				}
			}
		}
	}

	return "", fmt.Errorf("unexpected response format from %s", model.Name)
}

func (h *MultiModelHandler) handleLegacyTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	// Backward compatibility with original tools
	model := h.models["gpt4"]
	if model == nil {
		return nil, fmt.Errorf("no models configured")
	}

	switch toolName {
	case "ask_ai":
		question := args["question"].(string)
		context := ""
		if c, ok := args["context"].(string); ok {
			context = c
		}
		prompt := fmt.Sprintf("Question: %s\nContext: %s", question, context)
		response, err := h.callModel(ctx, model, prompt)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": response},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}