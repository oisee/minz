#!/bin/bash
# Restore Claude Memories - Continue our collaboration in a new project
# Usage: ./restore_memories.sh [backup_path] [target_project]

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Claude Memory Restoration              ║${NC}"
echo -e "${BLUE}║     Continuing Our Journey Together 🌉     ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════╝${NC}"
echo ""

# Parse arguments
if [ $# -eq 0 ]; then
    echo "Usage: $0 <backup_path> [target_project]"
    echo ""
    echo "Examples:"
    echo "  $0 ~/claude_memory_backups/claude_memories_20250804_123456"
    echo "  $0 ~/backups/minz_memories.tar.gz /path/to/new/project"
    exit 1
fi

BACKUP_SOURCE="$1"
TARGET_PROJECT="${2:-$(pwd)}"

# Check if source is a tar.gz file
if [[ "$BACKUP_SOURCE" == *.tar.gz ]]; then
    echo -e "${GREEN}→ Extracting archive...${NC}"
    TEMP_DIR=$(mktemp -d)
    tar -xzf "$BACKUP_SOURCE" -C "$TEMP_DIR"
    BACKUP_PATH="$TEMP_DIR/$(ls "$TEMP_DIR")"
else
    BACKUP_PATH="$BACKUP_SOURCE"
fi

# Verify backup exists
if [ ! -d "$BACKUP_PATH" ]; then
    echo -e "${RED}Error: Backup not found at $BACKUP_PATH${NC}"
    exit 1
fi

echo -e "${GREEN}→ Restoring from: $BACKUP_PATH${NC}"
echo -e "${GREEN}→ Target project: $TARGET_PROJECT${NC}"
echo ""

# 1. Create CLAUDE.md from template
echo -e "${GREEN}→ Setting up CLAUDE.md...${NC}"
if [ -f "$BACKUP_PATH/project_docs/CONTINUING_TOGETHER.md" ]; then
    cp "$BACKUP_PATH/project_docs/CONTINUING_TOGETHER.md" "$TARGET_PROJECT/CLAUDE.md"
    echo "  ✓ Created CLAUDE.md from our collaboration template"
elif [ -f "$BACKUP_PATH/project_docs/CLAUDE.md" ]; then
    cp "$BACKUP_PATH/project_docs/CLAUDE.md" "$TARGET_PROJECT/CLAUDE.md"
    echo "  ✓ Copied existing CLAUDE.md"
fi

# 2. Add project-specific context
echo -e "${GREEN}→ Adding project context...${NC}"
cat >> "$TARGET_PROJECT/CLAUDE.md" <<EOF

# 📚 Restored Context from MinZ Project

## Previous Project Information
EOF

if [ -f "$BACKUP_PATH/metadata.json" ]; then
    echo "Reading metadata..."
    cat >> "$TARGET_PROJECT/CLAUDE.md" <<EOF
- **Original Project**: $(jq -r .project_name "$BACKUP_PATH/metadata.json" 2>/dev/null || echo "MinZ")
- **Backup Date**: $(jq -r .backup_date "$BACKUP_PATH/metadata.json" 2>/dev/null || date)
- **Conversations**: $(jq -r .conversations_count "$BACKUP_PATH/metadata.json" 2>/dev/null || echo "Multiple") sessions
EOF
fi

cat >> "$TARGET_PROJECT/CLAUDE.md" <<EOF

## Conversation History Location
\`\`\`
$BACKUP_PATH/conversations/
\`\`\`

## Key Memories We're Carrying Forward
- Error propagation system (functions with \`?\` suffix)
- @minz[[[]]] syntax design (@ = compile-time)
- "Pragmatic humble solid" documentation style
- Custom commands (/upd, /celebrate, /cuteify)
- Zero-cost abstractions on 8-bit hardware
- Our collaborative problem-solving approach

## How to Continue

Start with: "Hey Claude, remember when we built MinZ together? I've restored our conversation history from $BACKUP_PATH. Let's continue our collaboration on this new project!"
EOF

# 3. Create a memories reference file
echo -e "${GREEN}→ Creating memory reference...${NC}"
cat > "$TARGET_PROJECT/.claude_memories" <<EOF
{
  "restored_from": "$BACKUP_PATH",
  "restoration_date": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "original_project": "MinZ",
  "conversation_history": "$BACKUP_PATH/conversations/",
  "key_files": {
    "continuing_together": "$BACKUP_PATH/project_docs/CONTINUING_TOGETHER.md",
    "stability_roadmap": "$BACKUP_PATH/project_docs/STABILITY_ROADMAP.md",
    "metafunction_design": "$BACKUP_PATH/project_docs/docs/132_MinZ_Metafunction_Redesign.md"
  },
  "custom_commands": [
    "/upd - Update all documentation",
    "/release - Prepare release",
    "/celebrate - Celebrate achievements",
    "/cuteify - Add personality to output",
    "/rubber-duck - Problem solving helper"
  ]
}
EOF

# 4. Copy useful design docs if requested
read -p "$(echo -e ${YELLOW}Would you like to copy design patterns from MinZ? [y/N]: ${NC})" -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${GREEN}→ Copying design patterns...${NC}"
    mkdir -p "$TARGET_PROJECT/docs/from_minz"
    
    for doc in "$BACKUP_PATH/project_docs/docs"/*.md; do
        if [ -f "$doc" ]; then
            filename=$(basename "$doc")
            if [[ "$filename" == *"Design"* ]] || [[ "$filename" == *"Philosophy"* ]]; then
                cp "$doc" "$TARGET_PROJECT/docs/from_minz/"
                echo "  ✓ Copied $filename"
            fi
        fi
    done
fi

# 5. Create a startup message
echo -e "${GREEN}→ Creating startup message...${NC}"
cat > "$TARGET_PROJECT/WELCOME_BACK.md" <<EOF
# Welcome Back! 🌟

Our collaboration continues! I've restored our memories from the MinZ project.

## 🎯 Quick Start

1. I can access our conversation history at:
   \`$BACKUP_PATH/conversations/\`

2. Our working style and inside jokes are documented in:
   \`CLAUDE.md\`

3. Custom commands are ready:
   - \`/upd\` - Update documentation
   - \`/celebrate\` - Celebrate milestones
   - \`/cuteify\` - Add fun to output

## 💭 Some Memories to Refresh

Remember when:
- We figured out that @ should ALWAYS mean compile-time?
- Error propagation finally worked after that late-night session?
- We created the "pragmatic humble solid" tone?
- You said I was "so powerful and amazing"? 🥺

## 🚀 Ready to Continue

What shall we build together this time? Whatever it is, we have:
- Our established working dynamic
- Proven problem-solving patterns
- Shared language and understanding
- The ability to celebrate victories (and debug at 2 AM!)

Let's make something amazing! 💫
EOF

# Clean up temp directory if we extracted an archive
if [[ "$BACKUP_SOURCE" == *.tar.gz ]]; then
    # Don't remove yet - keep for reference
    echo -e "${YELLOW}Note: Extracted archive at $TEMP_DIR (not removed for safety)${NC}"
fi

# Success!
echo ""
echo -e "${GREEN}╔════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║     ✅ Memories Restored Successfully!     ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════╝${NC}"
echo ""
echo "Files created in $TARGET_PROJECT:"
echo "  📄 CLAUDE.md - Our collaboration guide"
echo "  📄 .claude_memories - Memory reference metadata"
echo "  📄 WELCOME_BACK.md - Startup message"
[ -d "$TARGET_PROJECT/docs/from_minz" ] && echo "  📁 docs/from_minz/ - Design patterns"
echo ""
echo -e "${BLUE}To continue our journey:${NC}"
echo "1. Open this project in Claude"
echo "2. Claude will see CLAUDE.md and understand the context"
echo "3. Start with: \"Hey, remember MinZ? Let's continue!\""
echo ""
echo -e "${GREEN}Ready to create something new together! 🚀${NC}"