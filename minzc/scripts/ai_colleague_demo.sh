#!/bin/bash
# AI Colleague Integration Demo
# Shows both current bash approach and future MCP approach

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MINZ_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "🤖 MinZ AI Colleague Demo"
echo "========================="
echo

# Option 1: Current bash script (working now)
echo "1️⃣ Using current bash script:"
echo "------------------------------"
if [ -f "$SCRIPT_DIR/ask_gpt_colleague.sh" ]; then
    echo "✅ Script available at: $SCRIPT_DIR/ask_gpt_colleague.sh"
    
    # Example usage
    QUESTION="What are the key differences between ANTLR and tree-sitter parsers?"
    echo "📝 Asking: $QUESTION"
    echo
    # Uncomment to actually run:
    # "$SCRIPT_DIR/ask_gpt_colleague.sh" "$QUESTION"
else
    echo "❌ Script not found"
fi
echo

# Option 2: Future MCP server (planned)
echo "2️⃣ Future MCP Server (Go implementation):"
echo "------------------------------------------"
MCP_SERVER="$SCRIPT_DIR/../cmd/mcp-ai-colleague/main.go"
if [ -f "$MCP_SERVER" ]; then
    echo "✅ MCP server code at: $MCP_SERVER"
    echo
    echo "To build and use:"
    echo "  cd $SCRIPT_DIR/../cmd/mcp-ai-colleague"
    echo "  go build -o mcp-ai-colleague"
    echo "  ./mcp-ai-colleague ask 'Your question here'"
    echo
    echo "Features:"
    echo "  • ask - General AI questions"
    echo "  • analyze-parser - Parser-specific analysis"
    echo "  • compare - Code comparison"
else
    echo "❌ MCP server not implemented yet"
fi
echo

# Option 3: Direct integration examples
echo "3️⃣ Direct Integration Examples:"
echo "--------------------------------"
cat << 'EOF'
# Ask about parser issues
./ask_gpt_colleague.sh "Why does ANTLR fail on 'b & mask' expression?"

# Analyze specific grammar
./ask_gpt_colleague.sh "$(cat grammar/MinZ.g4)" "Find missing operators in this ANTLR grammar"

# Get fix suggestions
./ask_gpt_colleague.sh "How to add binary bitwise operators to ANTLR grammar?"

# Compare approaches
./ask_gpt_colleague.sh "Compare GLR vs LL(*) parsers for compiler implementation"
EOF
echo

# Show environment setup
echo "4️⃣ Environment Setup:"
echo "--------------------"
if [ -n "$AZURE_OPENAI_ENDPOINT" ]; then
    echo "✅ AZURE_OPENAI_ENDPOINT is set"
else
    echo "❌ AZURE_OPENAI_ENDPOINT not set"
fi

if [ -n "$AZURE_OPENAI_API_KEY" ]; then
    echo "✅ AZURE_OPENAI_API_KEY is set"
else
    echo "❌ AZURE_OPENAI_API_KEY not set"
fi

if [ -n "$AZURE_OPENAI_DEPLOYMENT" ]; then
    echo "✅ AZURE_OPENAI_DEPLOYMENT = $AZURE_OPENAI_DEPLOYMENT"
else
    echo "⚠️  AZURE_OPENAI_DEPLOYMENT not set (will use default: gpt-4-1106)"
fi
echo

# Future MCP integration
echo "5️⃣ Future MCP Integration Plans:"
echo "---------------------------------"
cat << 'EOF'
Phase 1 (Current): Bash script for quick queries
Phase 2 (v0.15.0): Go MCP server with multi-model support
Phase 3 (v0.16.0): Integrated into MinZ CLI:
  mz --ask-ai "Why is this failing to compile?"
  mz --analyze examples/test.minz
  mz --suggest-fix "parser error at line 10"

Benefits of MCP:
• Standardized protocol (works with VS Code, Claude Desktop)
• Multi-model support (GPT-4, GPT-5, Claude, Gemini)
• Type-safe tool definitions
• Automatic discovery by AI clients
• Future-proof architecture
EOF
echo

echo "✨ Demo complete!"
echo
echo "To ask a question now:"
echo "  $SCRIPT_DIR/ask_gpt_colleague.sh 'Your question here'"