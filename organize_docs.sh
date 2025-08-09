#!/bin/bash

# Documentation Organization Script
# Automatically numbers and moves documents from inbox/ to docs/

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}📚 MinZ Documentation Organizer${NC}"
echo "================================"

# Find the highest numbered document in docs/
find_next_number() {
    local max_num=0
    
    # Find all numbered docs (format: NNN_*.md)
    for file in docs/[0-9][0-9][0-9]_*.md; do
        if [ -f "$file" ]; then
            # Extract the number from filename
            num=$(basename "$file" | cut -d'_' -f1 | sed 's/^0*//')
            if [ "$num" -gt "$max_num" ]; then
                max_num=$num
            fi
        fi
    done
    
    # Return next number
    echo $((max_num + 1))
}

# Format number with leading zeros (001, 002, etc.)
format_number() {
    printf "%03d" $1
}

# Sanitize filename (remove special chars, spaces to underscores)
sanitize_name() {
    echo "$1" | sed 's/\.md$//' | sed 's/[^a-zA-Z0-9_-]/_/g' | sed 's/__*/_/g' | sed 's/^_//' | sed 's/_$//'
}

# Main processing
main() {
    # Create directories if they don't exist
    mkdir -p inbox
    mkdir -p docs
    
    # Check if there are any files to process
    shopt -s nullglob
    files=(inbox/*.md)
    
    if [ ${#files[@]} -eq 0 ]; then
        echo -e "${YELLOW}No markdown files found in inbox/${NC}"
        echo "Place your .md files in the inbox/ folder and run this script again."
        exit 0
    fi
    
    echo -e "${GREEN}Found ${#files[@]} file(s) to process${NC}"
    echo
    
    # Get the starting number
    next_num=$(find_next_number)
    
    # Process each file
    for file in "${files[@]}"; do
        # Get original filename without path
        original_name=$(basename "$file")
        
        # Skip .gitkeep
        if [ "$original_name" = ".gitkeep" ]; then
            continue
        fi
        
        # Sanitize the name
        clean_name=$(sanitize_name "$original_name")
        
        # Format the number
        formatted_num=$(format_number $next_num)
        
        # Create new filename
        new_name="${formatted_num}_${clean_name}.md"
        new_path="docs/$new_name"
        
        # Move the file
        mv "$file" "$new_path"
        
        echo -e "${GREEN}✓${NC} $original_name"
        echo -e "  → ${BLUE}$new_name${NC}"
        echo
        
        # Increment counter
        next_num=$((next_num + 1))
    done
    
    echo -e "${GREEN}✅ Documentation organized successfully!${NC}"
    
    # Show summary
    echo
    echo "Summary:"
    echo "--------"
    echo "Next available number: $(format_number $next_num)"
    echo "Total docs in docs/: $(ls -1 docs/*.md 2>/dev/null | wc -l)"
}

# Run main function
main "$@"