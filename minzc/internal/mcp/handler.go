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

// AIHandler implements the Handler interface for AI colleague tools
type AIHandler struct {
	azureEndpoint string
	azureKey      string
	deployment    string
	multiHandler  *MultiModelHandler
}

// NewAIHandler creates a new AI handler with multi-model support
func NewAIHandler() *AIHandler {
	return &AIHandler{
		azureEndpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
		azureKey:      os.Getenv("AZURE_OPENAI_API_KEY"),
		deployment:    getEnvOrDefault("AZURE_OPENAI_DEPLOYMENT", "gpt-4.1"), // Updated to gpt-4.1
		multiHandler:  NewMultiModelHandler(),
	}
}

// HandleRequest processes tool calls
func (h *AIHandler) HandleRequest(ctx context.Context, toolName string, params json.RawMessage) (interface{}, error) {
	// First try multi-model handler for new tools
	if h.multiHandler != nil {
		if strings.HasPrefix(toolName, "ask_gpt") || strings.HasPrefix(toolName, "ask_o4") || 
		   strings.HasPrefix(toolName, "ask_model") || toolName == "ask_ai_with_context" || 
		   toolName == "brainstorm_semantic_fixes" {
			return h.multiHandler.HandleRequest(ctx, toolName, params)
		}
	}

	// Handle legacy tools
	var result map[string]interface{}
	if err := json.Unmarshal(params, &result); err != nil {
		return nil, err
	}

	arguments := result["arguments"].(map[string]interface{})

	switch toolName {
	case "ask_ai":
		question := arguments["question"].(string)
		context := ""
		if c, ok := arguments["context"].(string); ok {
			context = c
		}
		return h.askAI(ctx, question, context)

	case "analyze_parser":
		parserType := arguments["parser_type"].(string)
		code := arguments["code"].(string)
		errorMsg := ""
		if e, ok := arguments["error"].(string); ok {
			errorMsg = e
		}
		return h.analyzeParser(ctx, parserType, code, errorMsg)

	case "compare_approaches":
		approach1 := arguments["approach1"].(string)
		approach2 := arguments["approach2"].(string)
		criteria := "general"
		if c, ok := arguments["criteria"].(string); ok {
			criteria = c
		}
		return h.compareApproaches(ctx, approach1, approach2, criteria)

	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (h *AIHandler) askAI(ctx context.Context, question, context string) (interface{}, error) {
	prompt := fmt.Sprintf("You are an expert MinZ compiler developer. Question: %s", question)
	if context != "" {
		prompt += fmt.Sprintf("\n\nContext:\n%s", context)
	}

	response, err := h.callAzureOpenAI(ctx, prompt)
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

func (h *AIHandler) analyzeParser(ctx context.Context, parserType, code, errorMsg string) (interface{}, error) {
	prompt := fmt.Sprintf(`Analyze this MinZ parser issue:
Parser Type: %s
Code that fails to parse:
%s

Error (if any): %s

Please suggest:
1. Why this might be failing
2. Potential fixes to the parser
3. Workarounds for the user`, parserType, code, errorMsg)

	response, err := h.callAzureOpenAI(ctx, prompt)
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

func (h *AIHandler) compareApproaches(ctx context.Context, approach1, approach2, criteria string) (interface{}, error) {
	prompt := fmt.Sprintf(`Compare these two implementation approaches for MinZ compiler:

Approach 1:
%s

Approach 2:
%s

Comparison criteria: %s

Please provide a detailed comparison including pros/cons and recommendation.`, approach1, approach2, criteria)

	response, err := h.callAzureOpenAI(ctx, prompt)
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

func (h *AIHandler) callAzureOpenAI(ctx context.Context, prompt string) (string, error) {
	if h.azureEndpoint == "" || h.azureKey == "" {
		return "AI service not configured. Please set AZURE_OPENAI_ENDPOINT and AZURE_OPENAI_API_KEY environment variables.", nil
	}

	// Ensure endpoint has proper protocol
	endpoint := h.azureEndpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2024-02-15-preview",
		strings.TrimSuffix(endpoint, "/"), h.deployment)

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an expert MinZ compiler developer helping with implementation decisions.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"max_tokens":  2000,
		"temperature": 0.7,
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
	req.Header.Set("api-key", h.azureKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call AI: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	choices := result["choices"].([]interface{})
	if len(choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	return message["content"].(string), nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}