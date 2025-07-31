#!/bin/bash

# AI Testing Revolution - Installation Script
# This script installs the AI-driven development practices for Claude CLI

echo "🚀 Installing AI Testing Revolution Practices..."

# Check if Claude CLI directory exists
CLAUDE_DIR="$HOME/.claude"
if [ ! -d "$CLAUDE_DIR" ]; then
    echo "Creating Claude CLI directory..."
    mkdir -p "$CLAUDE_DIR"
fi

# Create commands directory
COMMANDS_DIR="$CLAUDE_DIR/commands"
mkdir -p "$COMMANDS_DIR"

# Check if we're in the right directory
if [ ! -f ".claude/commands/ai-testing-revolution.md" ]; then
    echo "❌ Error: Please run this script from the MinZ project root directory"
    exit 1
fi

# Copy command files
echo "📁 Installing custom commands..."
cp .claude/commands/*.md "$COMMANDS_DIR/"

# List installed commands
echo ""
echo "✅ Installed commands:"
ls -1 "$COMMANDS_DIR"/*.md | xargs -n1 basename | sed 's/\.md$//' | sed 's/^/   \//

# Create a project template if requested
if [ "$1" == "--init-project" ]; then
    echo ""
    echo "📝 Initializing project with AI practices..."
    
    # Create .claude directory in current project
    mkdir -p .claude
    
    # Create a basic CLAUDE.md if it doesn't exist
    if [ ! -f "CLAUDE.md" ]; then
        cat > CLAUDE.md << 'EOF'
# CLAUDE.md

This file provides guidance to Claude Code when working with this project.

## AI-Driven Development Practices

For rapid development using AI agent orchestration, see:
@CLAUDE_BEST_PRACTICES.md

## Available Commands

- `/ai-testing-revolution` - Build complete testing infrastructure
- `/parallel-development` - Execute multiple tasks simultaneously
- `/performance-verification` - Verify optimization claims

EOF
        echo "✅ Created CLAUDE.md"
    fi
    
    # Copy best practices file
    cp CLAUDE_BEST_PRACTICES.md .
    echo "✅ Copied CLAUDE_BEST_PRACTICES.md"
fi

echo ""
echo "🎉 Installation complete!"
echo ""
echo "📚 Next steps:"
echo "1. Use /ai-testing-revolution to build testing infrastructure"
echo "2. Use /parallel-development for rapid feature development"
echo "3. Use /performance-verification to prove optimizations"
echo ""
echo "💡 To initialize a new project with these practices:"
echo "   ./install_ai_practices.sh --init-project"
echo ""
echo "Happy coding at AI speed! 🚀"