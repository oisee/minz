#!/bin/bash
# Ask GPT-4.1 colleague for advice on MinZ compiler issues

# Check if question is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 \"Your question\" [file1] [file2] ..."
    echo "Example: $0 \"Why is ANTLR parser failing?\" pkg/parser/antlr_parser.go"
    exit 1
fi

QUESTION="$1"
shift

# Build the content with question and optional files
CONTENT="Question about MinZ compiler development:\n\n$QUESTION"

# Add file contents if provided
if [ $# -gt 0 ]; then
    CONTENT="$CONTENT\n\n--- Related Files ---"
    for file in "$@"; do
        if [ -f "$file" ]; then
            CONTENT="$CONTENT\n\nFile: $file\n\`\`\`\n$(cat "$file")\n\`\`\`"
        fi
    done
fi

# Prepare JSON payload
JSON_PAYLOAD=$(jq -n \
    --arg content "$CONTENT" \
    '{
        messages: [
            {
                role: "system",
                content: "You are an expert compiler engineer helping with the MinZ language compiler. MinZ compiles to Z80 assembly and uses either ANTLR or tree-sitter for parsing. Provide detailed technical analysis and solutions."
            },
            {
                role: "user", 
                content: $content
            }
        ],
        max_tokens: 2000,
        temperature: 0.3
    }')

# Call Azure OpenAI
echo "🤖 Asking GPT-4.1 colleague..."
echo "Question: $QUESTION"
echo ""

RESPONSE=$(curl -s -X POST "${AZURE_OPENAI_ENDPOINT}openai/deployments/${AZURE_OPENAI_DEPLOYMENT}/chat/completions?api-version=${AZURE_OPENAI_API_VERSION}" \
    -H "Content-Type: application/json" \
    -H "api-key: ${AZURE_OPENAI_API_KEY}" \
    -d "$JSON_PAYLOAD")

# Extract and display the response
echo "$RESPONSE" | jq -r '.choices[0].message.content' 2>/dev/null || echo "Error: Failed to get response"