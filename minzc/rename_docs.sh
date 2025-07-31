#!/bin/bash

# Get all markdown files with their creation dates, excluding README and CLAUDE
# Sort by date and rename sequentially

cd docs

# Create temporary file with dates and filenames
temp_file="/tmp/doc_rename_list.txt"
rm -f "$temp_file"

for file in *.md; do
  if [[ "$file" != "README.md" && "$file" != "CLAUDE.md" ]]; then
    # Get the earliest date from git log (creation date)
    creation_date=$(git log --follow --format=%ai --reverse "$file" 2>/dev/null | head -1)
    if [ -n "$creation_date" ]; then
      echo "$creation_date|$file" >> "$temp_file"
    fi
  fi
done

# Sort by date and process renames
sort "$temp_file" | awk -F'|' '
BEGIN { counter = 1 }
{
  old_name = $2
  # Skip if already correctly numbered
  if (old_name ~ /^compiler-enhancements-success-report\.md$/) {
    # Keep this special file as-is
    print "Keeping: " old_name
  } else {
    # Extract the title part after the number
    match(old_name, /^[0-9]+_(.+)$/, arr)
    if (arr[1] != "") {
      title = arr[1]
    } else {
      # No number prefix, use whole name without .md
      match(old_name, /^(.+)\.md$/, arr2)
      title = arr2[1] ".md"
    }
    
    # Create new name with proper number
    new_name = sprintf("%03d_%s", counter, title)
    
    if (old_name != new_name) {
      print "git mv \"" old_name "\" \"" new_name "\""
      system("git mv \"" old_name "\" \"" new_name "\"")
    }
    counter++
  }
}'

# Clean up
rm -f "$temp_file"

echo "Done! Files renamed in chronological order."