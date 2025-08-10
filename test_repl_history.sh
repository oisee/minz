#!/bin/bash

echo "Testing MinZ REPL Command History with Arrow Keys"
echo "=================================================="
echo ""
echo "The REPL now supports:"
echo "  • ↑ (Up Arrow)    - Navigate to previous command in history"
echo "  • ↓ (Down Arrow)  - Navigate to next command in history"
echo "  • ← (Left Arrow)  - Move cursor left"
echo "  • → (Right Arrow) - Move cursor right"
echo "  • Backspace       - Delete character before cursor"
echo "  • Ctrl+C          - Cancel current line"
echo "  • Ctrl+D          - Exit REPL (on empty line)"
echo ""
echo "Starting the REPL - try it out!"
echo ""

./mzr