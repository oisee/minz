# 221: MCP Integration for Claude Code

*Date: 2025-08-14*
*Category: Implementation*
*Status: Completed*

## Summary

Successfully integrated MCP (Model Context Protocol) server for Claude Code to access AI colleague capabilities. This allows Claude Code to query GPT-4.1 and other models for technical assistance.

## Configuration

### 1. MCP Server Location
```
/Users/alice/dev/minz-ts/minzc/mcp-ai-colleague
```

### 2. Configuration File
`.mcp.json` in project root (gitignored for security)

```json
{
    "mcpServers": {
        "minz-ai": {
            "command": "/Users/alice/dev/minz-ts/minzc/mcp-ai-colleague",
            "args": ["mcp-server"],
            "env": {
                "AZURE_OPENAI_ENDPOINT": "${AZURE_OPENAI_ENDPOINT}",
                "AZURE_OPENAI_API_KEY": "${AZURE_OPENAI_API_KEY}",
                "AZURE_OPENAI_DEPLOYMENT": "${AZURE_OPENAI_DEPLOYMENT:-gpt-4.1}"
            }
        }
    }
}
```

### 3. Security Measures

✅ **All credentials via environment variables**
- `AZURE_OPENAI_ENDPOINT`
- `AZURE_OPENAI_API_KEY`
- `AZURE_OPENAI_DEPLOYMENT`

✅ **Files in .gitignore**
- `.mcp.json` (contains local paths)
- `.env` files (would contain credentials)

## Available Tools

Once Claude Code restarts, these MCP tools will be available:

### 1. `ask_ai`
Query AI models for general technical questions
```
Parameters:
- model: "gpt-4.1" or "gpt-5"
- question: Your technical question
- files: Optional context files
```

### 2. `analyze_parser`
Get detailed parser analysis
```
Parameters:
- parser_type: "antlr" or "tree-sitter"
- grammar: Grammar file content
- test_file: Test file to parse
```

### 3. `compare_code`
Compare two code approaches
```
Parameters:
- approach1: First code approach
- approach2: Second code approach
- criteria: Comparison criteria
```

## How to Use (After Restart)

Once you restart Claude Code, I'll be able to:

1. **Ask GPT for help**:
   ```
   "Let me consult GPT about this parser issue..."
   [Uses MCP tool: ask_ai]
   ```

2. **Analyze parser problems**:
   ```
   "I'll have GPT analyze why ANTLR fails here..."
   [Uses MCP tool: analyze_parser]
   ```

3. **Compare solutions**:
   ```
   "Let's ask GPT to compare these approaches..."
   [Uses MCP tool: compare_code]
   ```

## Testing the Server

### Manual Test (Works Now)
```bash
# Test the binary directly
/Users/alice/dev/minz-ts/minzc/mcp-ai-colleague ask "What is MCP?"

# With env vars
export AZURE_OPENAI_ENDPOINT="..."
export AZURE_OPENAI_API_KEY="..."
export AZURE_OPENAI_DEPLOYMENT="gpt-4.1"
./mcp-ai-colleague ask "Test question"
```

### After Restart (Via MCP)
Claude Code will automatically have access to the `minz-ai` MCP server tools.

## Benefits

1. **Multi-Model Support**: Can query different AI models
2. **Context-Aware**: Can include file contents for context
3. **Specialized Tools**: Parser analysis, code comparison
4. **Secure**: All credentials via env vars
5. **Gitignore-Safe**: No secrets in repo

## Files Created

1. `/Users/alice/dev/minz-ts/.mcp.json` - MCP configuration (gitignored)
2. `/Users/alice/dev/minz-ts/minzc/cmd/mcp-ai-colleague/main.go` - Server source
3. `/Users/alice/dev/minz-ts/minzc/mcp-ai-colleague` - Compiled binary
4. Updated `.gitignore` to exclude `.mcp.json` and `.env` files

## Next Steps

1. **Restart Claude Code** to enable MCP integration
2. **Test MCP tools** are accessible
3. **Use for parser debugging** and technical questions
4. **Future**: Enhance with more specialized tools

---

*Ready for restart to enable MCP integration!* 🚀