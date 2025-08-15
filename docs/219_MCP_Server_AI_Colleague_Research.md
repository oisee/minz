# 219: MCP Server for AI Colleague Integration Research

*Date: 2025-08-14*
*Category: Architecture Research*
*Status: Research Complete*

## Executive Summary

After researching MCP (Model Context Protocol) implementations, we have multiple options for creating an AI colleague tool that can query different models (GPT-4.1, GPT-5, Claude, etc.). MCP provides a standardized way to integrate AI models with external tools and data sources.

## What is MCP?

Model Context Protocol (MCP) is an open standard created by Anthropic (November 2024) that:
- Provides a unified protocol for AI-tool integration
- Enables AI models to access external data and execute actions
- Supports multiple AI providers (OpenAI, Anthropic, Google, Azure)
- Eliminates the need for custom integrations per model

## Architecture Options

### Option 1: Use Existing MCP Server (Recommended for Quick Start)

#### metoro-io/mcp-golang ⭐ RECOMMENDED
```go
// Minimal setup - just a few lines of code
import "github.com/metoro-io/mcp-golang"

// Define your AI colleague tool
type AIColleagueTool struct {
    AzureEndpoint string
    APIKey        string
}

func (t *AIColleagueTool) AskGPT(question string, files []string) (string, error) {
    // Implementation here
}
```

**Pros:**
- Minimal boilerplate
- Type-safe with automatic schema generation
- Well-designed API
- Production-ready

**Cons:**
- Unofficial implementation
- May need customization for Azure OpenAI

#### mark3labs/mcp-go
```go
import "github.com/mark3labs/mcp-go"

// More comprehensive but slightly more complex setup
```

**Pros:**
- Handles protocol details automatically
- Good documentation
- Active development

**Cons:**
- More complex than metoro-io
- Still in progress for some features

### Option 2: Create Custom MCP Server in Golang

```go
// Custom MCP server for AI colleague queries
package main

import (
    "github.com/metoro-io/mcp-golang"
    "github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
)

type AIColleagueServer struct {
    clients map[string]*azopenai.Client
}

// Tools exposed via MCP
func (s *AIColleagueServer) Tools() []mcp.Tool {
    return []mcp.Tool{
        {
            Name: "ask_gpt",
            Description: "Ask GPT-4.1 or GPT-5 for analysis",
            Parameters: mcp.ToolParams{
                "model": "gpt-4.1 or gpt-5",
                "question": "The question to ask",
                "context": "Optional context files",
            },
        },
        {
            Name: "analyze_code",
            Description: "Get AI analysis of code",
            Parameters: mcp.ToolParams{
                "code": "Code to analyze",
                "language": "Programming language",
            },
        },
    }
}
```

### Option 3: Direct Integration (Current Approach)

Keep the current `ask_gpt_colleague.sh` script and enhance it:

```bash
# Current simple approach - works well!
/Users/alice/dev/minz-ts/minzc/scripts/ask_gpt_colleague.sh
```

**Pros:**
- Already working
- Simple and direct
- No dependencies

**Cons:**
- Not standardized
- Limited to bash scripting
- No multi-model support

## Implementation Recommendation

### Phase 1: Enhance Current Script (Immediate)
1. Keep `ask_gpt_colleague.sh` for now
2. Add support for multiple models
3. Add better error handling

### Phase 2: MCP Server (Next Sprint)
1. Use `metoro-io/mcp-golang` for quick implementation
2. Create MCP server with these tools:
   - `ask_ai` - Query any AI model
   - `analyze_parser` - Specific parser analysis
   - `compare_implementations` - Compare code approaches
   - `suggest_fixes` - Get fix suggestions

### Phase 3: Full Integration (Future)
1. Integrate with MinZ compiler directly
2. Add MCP client to MinZ CLI
3. Enable `mz --ask-ai "question"` command

## Example MCP Server Implementation

```go
// minzc/cmd/mcp-server/main.go
package main

import (
    "github.com/metoro-io/mcp-golang/server"
    "github.com/metoro-io/mcp-golang/transport/stdio"
)

type MinZAIColleague struct {
    azureKey      string
    azureEndpoint string
}

func (m *MinZAIColleague) AskGPT(ctx context.Context, args AskGPTArgs) (*Answer, error) {
    // Call Azure OpenAI API
    // Return structured response
}

func main() {
    colleague := &MinZAIColleague{
        azureKey:      os.Getenv("AZURE_OPENAI_API_KEY"),
        azureEndpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
    }
    
    server := server.NewMCPServer(
        "MinZ AI Colleague",
        "1.0.0",
        server.WithTools(colleague),
    )
    
    transport := stdio.NewTransport()
    server.Serve(transport)
}
```

## Benefits of MCP Approach

1. **Standardization**: Works with Claude Desktop, VS Code, and other MCP clients
2. **Multi-Model**: Easy to add GPT-4, GPT-5, Claude, Gemini support
3. **Tool Discovery**: Clients can discover available AI tools
4. **Type Safety**: Automatic validation of requests/responses
5. **Future-Proof**: MCP is becoming industry standard

## Current Azure OpenAI Integration

Azure fully supports MCP with:
- Azure AI Agent Service (managed MCP servers)
- Azure OpenAI Service integration
- Multi-server orchestration
- Built-in security and scaling

## Decision Matrix

| Criteria | Current Script | metoro-io/mcp-golang | Custom MCP | Azure AI Agent |
|----------|---------------|---------------------|------------|----------------|
| Setup Time | ✅ 0 (done) | 1 day | 3 days | 1 week |
| Maintenance | Medium | Low | High | Very Low |
| Features | Basic | Good | Excellent | Excellent |
| Cost | Free | Free | Free | Paid |
| Multi-Model | No | Yes | Yes | Yes |
| Standardized | No | Yes | Yes | Yes |

## Conclusion

**Immediate Action**: Continue using the current `ask_gpt_colleague.sh` script - it works well!

**Next Sprint**: Implement a simple MCP server using `metoro-io/mcp-golang` to:
- Standardize the AI colleague interface
- Enable multi-model support (GPT-4.1, GPT-5, Claude)
- Integrate with existing MCP clients
- Future-proof the architecture

The MCP approach will make the AI colleague tool more powerful and reusable across different contexts, while maintaining the simplicity we need for the MinZ compiler project.

---

*Next Step: Continue with v0.14.0 release using current script, plan MCP implementation for v0.15.0*