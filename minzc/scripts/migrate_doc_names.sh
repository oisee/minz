#!/bin/bash
# migrate_doc_names.sh - Rename docs from NNN_Topic.md to YYYY-MM-DD-NNN-Topic.md
#
# Usage: ./migrate_doc_names.sh [--dry-run] <docs_dir>

set -e

DRY_RUN=false
if [ "$1" = "--dry-run" ]; then
    DRY_RUN=true
    shift
fi

DOCS_DIR="${1:-docs}"

if [ ! -d "$DOCS_DIR" ]; then
    echo "Error: Directory $DOCS_DIR not found"
    exit 1
fi

echo "Migrating docs in: $DOCS_DIR"
echo "Dry run: $DRY_RUN"
echo ""

# Process each numbered doc
for file in "$DOCS_DIR"/[0-9][0-9][0-9]_*.md; do
    [ -e "$file" ] || continue

    filename=$(basename "$file")

    # Extract number and topic from NNN_Topic.md
    if [[ $filename =~ ^([0-9]{3})_(.+)\.md$ ]]; then
        num="${BASH_REMATCH[1]}"
        topic="${BASH_REMATCH[2]}"

        # Get creation date from git (first commit that added the file)
        git_date=$(git log --follow --format=%as --diff-filter=A -- "$file" 2>/dev/null | tail -1)

        # Fall back to modification time if git history not available
        if [ -z "$git_date" ]; then
            git_date=$(stat -c %y "$file" 2>/dev/null | cut -d' ' -f1)
        fi

        # Fall back to today if still no date
        if [ -z "$git_date" ]; then
            git_date=$(date +%Y-%m-%d)
        fi

        # Create new filename: YYYY-MM-DD-NNN-Topic.md
        new_filename="${git_date}-${num}-${topic}.md"
        new_path="$DOCS_DIR/$new_filename"

        if [ "$DRY_RUN" = true ]; then
            echo "[DRY RUN] $filename -> $new_filename"
        else
            if [ "$file" != "$new_path" ]; then
                git mv "$file" "$new_path" 2>/dev/null || mv "$file" "$new_path"
                echo "Renamed: $filename -> $new_filename"
            fi
        fi
    fi
done

echo ""
echo "Migration complete!"
if [ "$DRY_RUN" = true ]; then
    echo "Run without --dry-run to apply changes."
fi
