package mcp

// getExtendedTools returns all available tools including multi-model support
func (s *Server) getExtendedTools() []map[string]interface{} {
	baseTools := []map[string]interface{}{
		// Original tools for backward compatibility
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
					"model": map[string]interface{}{
						"type":        "string",
						"description": "Optional: Model to use (gpt4, gpt5, o4_mini, model_router). Defaults to standard GPT-4 if not specified.",
						"enum":        []string{"gpt4", "gpt5", "o4_mini", "model_router"},
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
		
		// New multi-model tools with capability descriptions
		{
			"name":        "ask_gpt4",
			"description": "Ask GPT-4.1 - CONTEXT: 1M tokens, STRENGTHS: Large file analysis, full codebase understanding, comprehensive documentation review. Use for: analyzing entire semantic analyzer, processing multiple large source files, understanding complex architectural relationships",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "Question to ask GPT-4.1",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Can include entire source files (up to 1M tokens - ~2M chars)",
					},
				},
				"required": []string{"question"},
			},
		},
		{
			"name":        "ask_gpt5",
			"description": "Ask gpt-5 - CONTEXT: Standard, STRENGTHS: Most advanced reasoning, complex problem solving, novel architecture design, breakthrough insights. Use for: hardest compiler problems, innovative solutions, architectural redesigns, complex type system issues",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "Question to ask GPT-5",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Focused context for deep reasoning",
					},
				},
				"required": []string{"question"},
			},
		},
		{
			"name":        "ask_o4_mini",
			"description": "Ask o4-mini - CONTEXT: Standard, STRENGTHS: Deep thinking, step-by-step reasoning, careful analysis, thorough verification. Use for: debugging complex semantic issues, verifying correctness, detailed code review, finding subtle bugs",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "Question requiring careful analysis",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Code or problem requiring deep thinking",
					},
				},
				"required": []string{"question"},
			},
		},
		{
			"name":        "ask_model_router",
			"description": "Ask model-router - CONTEXT: Variable, STRENGTHS: Automatically selects optimal model based on query type. Use for: general questions when unsure which model is best, mixed workloads",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "Question for automatic routing",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Optional context",
					},
				},
				"required": []string{"question"},
			},
		},
		{
			"name":        "ask_ai_with_context",
			"description": "Ask AI with actual source file grounding for accurate analysis",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "Question about the code",
					},
					"files": map[string]interface{}{
						"type":        "array",
						"description": "List of source file paths to include as context",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"model": map[string]interface{}{
						"type":        "string",
						"description": "Model to use: gpt4, gpt5, o4_mini, model_router",
						"enum":        []string{"gpt4", "gpt5", "o4_mini", "model_router"},
					},
				},
				"required": []string{"question", "files"},
			},
		},
		{
			"name":        "brainstorm_semantic_fixes",
			"description": "Get multi-model perspectives on semantic analyzer improvements",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"issues": map[string]interface{}{
						"type":        "string",
						"description": "Description of semantic analyzer issues to fix",
					},
				},
				"required": []string{},
			},
		},
	}
	
	return baseTools
}