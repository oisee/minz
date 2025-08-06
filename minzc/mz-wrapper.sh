#!/bin/bash
# MinZ compiler wrapper - works from anywhere

# Find where this script is installed
SCRIPT_DIR="$(dirname "$(realpath "$0")")"

# Find the MinZ root directory (where grammar.js lives)
# Try common locations
if [ -f "$SCRIPT_DIR/../../grammar.js" ]; then
    # Installed via make install-local from minzc
    MINZ_DIR="$(realpath "$SCRIPT_DIR/../..")"
elif [ -f "$SCRIPT_DIR/../grammar.js" ]; then
    # In minzc directory
    MINZ_DIR="$(realpath "$SCRIPT_DIR/..")"
elif [ -f "$HOME/dev/minz-ts/grammar.js" ]; then
    # Development location
    MINZ_DIR="$HOME/dev/minz-ts"
else
    echo "Error: Cannot find MinZ grammar files!" >&2
    echo "Please ensure MinZ is properly installed." >&2
    exit 1
fi

# Find the mz binary
if [ -x "$SCRIPT_DIR/mz" ]; then
    MZ_BINARY="$SCRIPT_DIR/mz"
elif [ -x "$MINZ_DIR/minzc/mz" ]; then
    MZ_BINARY="$MINZ_DIR/minzc/mz"
else
    echo "Error: Cannot find mz compiler binary!" >&2
    exit 1
fi

# Run mz from the MinZ directory so tree-sitter can find grammar
cd "$MINZ_DIR" && exec "$MZ_BINARY" "$@"