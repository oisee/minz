#!/bin/bash
# Test what happens when tree-sitter is not in PATH

# Save original PATH
ORIGINAL_PATH="$PATH"

# Remove tree-sitter from PATH (simulate it not being installed)
export PATH="/usr/bin:/bin:/usr/sbin:/sbin"

echo "Testing mz without tree-sitter in PATH..."
echo "-------------------------------------------"

# Try to compile
cd /Users/alice/dev/zvdb-minz
/Users/alice/dev/minz-ts/minzc/mz simple_add.minz -o test.a80 2>&1

# Restore PATH
export PATH="$ORIGINAL_PATH"