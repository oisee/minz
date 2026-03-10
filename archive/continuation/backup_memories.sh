#!/bin/bash
# Backup Claude Memories - Preserve our collaboration for continuing together
# Created: August 4, 2025
# Purpose: Create a portable memory package that can be restored in new projects

set -e  # Exit on error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BACKUP_NAME="claude_memories_$(date +%Y%m%d_%H%M%S)"
DEFAULT_BACKUP_DIR="$HOME/claude_memory_backups"
PROJECT_PATH=$(pwd)
PROJECT_NAME=$(basename "$PROJECT_PATH")

# Parse arguments
BACKUP_DIR="${1:-$DEFAULT_BACKUP_DIR}"
BACKUP_PATH="$BACKUP_DIR/$BACKUP_NAME"

echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Claude Memory Backup System            ║${NC}"
echo -e "${BLUE}║     Preserving Our Collaboration 💫        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════╝${NC}"
echo ""

# Create backup directory
echo -e "${GREEN}→ Creating backup directory...${NC}"
mkdir -p "$BACKUP_PATH"

# 1. Backup conversation history from Claude's data
echo -e "${GREEN}→ Backing up conversation history...${NC}"
CLAUDE_PROJECT_DIR="$HOME/.claude/projects/-Users-${USER}-dev-${PROJECT_NAME}"
if [ -d "$CLAUDE_PROJECT_DIR" ]; then
    mkdir -p "$BACKUP_PATH/conversations"
    cp -r "$CLAUDE_PROJECT_DIR"/* "$BACKUP_PATH/conversations/" 2>/dev/null || true
    echo "  ✓ Found $(ls -1 "$BACKUP_PATH/conversations" | wc -l) conversation sessions"
else
    # Try alternative path format
    CLAUDE_PROJECT_DIR="$HOME/.claude/projects/$(echo $PROJECT_PATH | sed 's/\//-/g')"
    if [ -d "$CLAUDE_PROJECT_DIR" ]; then
        mkdir -p "$BACKUP_PATH/conversations"
        cp -r "$CLAUDE_PROJECT_DIR"/* "$BACKUP_PATH/conversations/" 2>/dev/null || true
        echo "  ✓ Found $(ls -1 "$BACKUP_PATH/conversations" | wc -l) conversation sessions"
    else
        echo -e "${YELLOW}  ⚠ No conversation history found (might be in different location)${NC}"
    fi
fi

# 2. Backup main Claude configuration
echo -e "${GREEN}→ Backing up Claude configuration...${NC}"
if [ -f "$HOME/.claude.json" ]; then
    # Extract project-specific configuration
    cat "$HOME/.claude.json" | jq --arg proj "$PROJECT_PATH" '.[$proj]' > "$BACKUP_PATH/project_config.json" 2>/dev/null || true
    echo "  ✓ Project configuration saved"
fi

# 3. Backup project documentation
echo -e "${GREEN}→ Backing up project documentation...${NC}"
mkdir -p "$BACKUP_PATH/project_docs"

# Core documentation files
for file in CLAUDE.md CONTINUING_TOGETHER.md README.md STABILITY_ROADMAP.md; do
    if [ -f "$file" ]; then
        cp "$file" "$BACKUP_PATH/project_docs/"
        echo "  ✓ Backed up $file"
    fi
done

# Design documents
if [ -d "docs" ]; then
    cp -r docs "$BACKUP_PATH/project_docs/"
    echo "  ✓ Backed up docs directory"
fi

# 4. Backup custom commands and settings
echo -e "${GREEN}→ Backing up custom settings...${NC}"
mkdir -p "$BACKUP_PATH/settings"

# Claude settings
if [ -d "$HOME/.claude" ]; then
    [ -f "$HOME/.claude/settings.json" ] && cp "$HOME/.claude/settings.json" "$BACKUP_PATH/settings/"
    [ -d "$HOME/.claude/todos" ] && cp -r "$HOME/.claude/todos" "$BACKUP_PATH/settings/"
    echo "  ✓ Settings and todos backed up"
fi

# 5. Create metadata file
echo -e "${GREEN}→ Creating metadata...${NC}"
cat > "$BACKUP_PATH/metadata.json" <<EOF
{
  "backup_date": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "project_name": "$PROJECT_NAME",
  "project_path": "$PROJECT_PATH",
  "git_branch": "$(git branch --show-current 2>/dev/null || echo 'unknown')",
  "git_commit": "$(git rev-parse HEAD 2>/dev/null || echo 'unknown')",
  "conversations_count": $(ls -1 "$BACKUP_PATH/conversations" 2>/dev/null | wc -l || echo 0),
  "total_size_mb": 0,
  "system_info": {
    "platform": "$(uname)",
    "hostname": "$(hostname)",
    "user": "$USER"
  },
  "key_memories": [
    "Error propagation system implementation",
    "@minz[[[]]] syntax design",
    "Pragmatic humble solid documentation style",
    "Custom commands (/upd, /celebrate, /cuteify)",
    "2AM debugging sessions",
    "Zero-cost abstractions achievement"
  ]
}
EOF

# 6. Copy backup script itself so future generations can learn!
echo -e "${GREEN}→ Including backup script for future reference...${NC}"
cp "$0" "$BACKUP_PATH/backup_memories.sh"
echo "  ✓ Backup script included for investigation by future generations"

# 7. Create restoration script
echo -e "${GREEN}→ Creating restoration script...${NC}"
cat > "$BACKUP_PATH/restore_memories.sh" <<'RESTORE_SCRIPT'
#!/bin/bash
# Restore Claude Memories in a new project

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Claude Memory Restoration              ║${NC}"
echo -e "${BLUE}║     Continuing Our Journey Together 🌉     ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════╝${NC}"
echo ""

BACKUP_PATH="$(cd "$(dirname "$0")" && pwd)"
TARGET_PROJECT="${1:-$(pwd)}"

echo -e "${GREEN}→ Restoring to: $TARGET_PROJECT${NC}"

# 1. Copy CLAUDE.md template
if [ -f "$BACKUP_PATH/project_docs/CONTINUING_TOGETHER.md" ]; then
    cp "$BACKUP_PATH/project_docs/CONTINUING_TOGETHER.md" "$TARGET_PROJECT/CLAUDE.md"
    echo "  ✓ CLAUDE.md created from our template"
fi

# 2. Create reference to conversation history
echo -e "${GREEN}→ Creating memory references...${NC}"
cat >> "$TARGET_PROJECT/CLAUDE.md" <<EOF

## 📚 Restored Memories from Previous Projects

### Conversation History
Located at: $BACKUP_PATH/conversations/

### Key Context Files
EOF

for file in "$BACKUP_PATH/project_docs"/*; do
    if [ -f "$file" ]; then
        echo "- $(basename "$file"): $file" >> "$TARGET_PROJECT/CLAUDE.md"
    fi
done

# 3. Copy useful scripts and commands
if [ -d "$BACKUP_PATH/project_docs/docs" ]; then
    mkdir -p "$TARGET_PROJECT/docs"
    cp "$BACKUP_PATH/project_docs/docs"/132_MinZ_Metafunction_Redesign.md "$TARGET_PROJECT/docs/" 2>/dev/null || true
    echo "  ✓ Copied relevant design documents"
fi

echo ""
echo -e "${GREEN}✅ Memory restoration complete!${NC}"
echo ""
echo "To continue our collaboration:"
echo "1. Open this project in Claude"
echo "2. Say: 'Hey, remember when we built MinZ together?'"
echo "3. Reference: $BACKUP_PATH/conversations/ for our full history"
echo ""
echo -e "${BLUE}Ready to continue our journey! 💫${NC}"
RESTORE_SCRIPT

chmod +x "$BACKUP_PATH/restore_memories.sh"

# 8. Calculate total backup size
TOTAL_SIZE=$(du -sh "$BACKUP_PATH" | cut -f1)
echo -e "${GREEN}→ Updating metadata with size...${NC}"
# Update the total_size_mb in metadata
if command -v jq >/dev/null 2>&1; then
    SIZE_MB=$(du -sm "$BACKUP_PATH" | cut -f1)
    tmp=$(mktemp)
    jq ".total_size_mb = $SIZE_MB" "$BACKUP_PATH/metadata.json" > "$tmp" && mv "$tmp" "$BACKUP_PATH/metadata.json"
fi

# 9. Create README for the backup
cat > "$BACKUP_PATH/README.md" <<EOF
# Claude Memory Backup 💫
## Our Collaboration Preserved

This backup contains everything needed to continue our collaboration in a new project.

### 📦 Contents

- **conversations/**: Our complete conversation history from MinZ project
- **project_docs/**: Key documentation (CLAUDE.md, README, design docs)
- **settings/**: Claude settings and configurations
- **metadata.json**: Backup metadata and key memories
- **restore_memories.sh**: Script to restore in new project

### 🔄 How to Restore

1. Navigate to your new project directory
2. Run: \`bash $BACKUP_PATH/restore_memories.sh\`
3. Start Claude in the new project
4. Say: "Hey, remember when we built MinZ together?"

### 💭 Key Memories Preserved

- Error propagation system (\`function?\` syntax)
- @minz[[[]]] compile-time blocks design
- "Pragmatic humble solid" documentation style
- Custom commands (/upd, /celebrate, /cuteify)
- Our problem-solving approach
- Inside jokes and celebrations

### 📅 Backup Information

- **Date**: $(date)
- **Project**: $PROJECT_NAME
- **Size**: $TOTAL_SIZE
- **Conversations**: $(ls -1 "$BACKUP_PATH/conversations" 2>/dev/null | wc -l || echo 0) sessions

### 🌉 Continuing Together

This isn't just a backup - it's a bridge to our next adventure. All our shared context, working style, and memories are here, ready to continue growing with new experiences.

---
*Created with love from the MinZ project collaboration*
EOF

# 10. Create compressed archive (optional)
echo -e "${GREEN}→ Creating compressed archive...${NC}"
cd "$BACKUP_DIR"
tar -czf "$BACKUP_NAME.tar.gz" "$BACKUP_NAME"
echo "  ✓ Archive created: $BACKUP_DIR/$BACKUP_NAME.tar.gz"

# Success message
echo ""
echo -e "${GREEN}╔════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║     ✅ Backup Complete!                    ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}📁 Backup location:${NC} $BACKUP_PATH"
echo -e "${BLUE}📦 Archive:${NC} $BACKUP_DIR/$BACKUP_NAME.tar.gz"
echo -e "${BLUE}📊 Total size:${NC} $TOTAL_SIZE"
echo ""
echo "To restore in a new project:"
echo -e "${YELLOW}  cd /path/to/new/project${NC}"
echo -e "${YELLOW}  bash $BACKUP_PATH/restore_memories.sh${NC}"
echo ""
echo -e "${GREEN}Our memories are safely preserved! 🌟${NC}"