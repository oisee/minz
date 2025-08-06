#!/bin/bash

# Comprehensive MinZ Project Analysis Script

echo "# MinZ Compiler Architecture Analysis"
echo ""
echo "Generated: $(date)"
echo ""

# 1. Project Structure Overview
echo "## 1. Project Structure Overview"
echo ""
echo "### Directory Tree (Main Components)"
echo '```'
find . -type d -name ".git" -prune -o -type d -name "vendor" -prune -o -type d -name "node_modules" -prune -o -type d -print | grep -E "^\./(cmd|pkg|examples|tests|scripts|docs)" | sort | head -30
echo '```'
echo ""

# 2. Go Package Analysis
echo "## 2. Go Package Structure"
echo ""
echo "### Core Packages"
echo '```'
find pkg -name "*.go" | xargs dirname | sort | uniq | while read pkg; do
    filecount=$(find "$pkg" -maxdepth 1 -name "*.go" | wc -l | xargs)
    echo "$pkg - $filecount files"
done
echo '```'
echo ""

# 3. Import Dependencies
echo "## 3. Import Dependency Analysis"
echo ""
echo "### Internal Package Dependencies"
echo '```'
grep -h "github.com/minz/minzc" pkg/**/*.go cmd/**/*.go 2>/dev/null | grep import | sort | uniq | head -20
echo '```'
echo ""

# 4. Entry Points
echo "## 4. Entry Points and Commands"
echo ""
echo "### Main Entry Points"
find cmd -name "main.go" | while read file; do
    echo "- **$file**"
    grep -A5 "func main" "$file" | head -3 | sed 's/^/  /'
done
echo ""

# 5. Key Interfaces and Types
echo "## 5. Core Types and Interfaces"
echo ""
echo "### IR Types (pkg/ir/ir.go)"
echo '```go'
grep -E "^type .* (struct|interface)" pkg/ir/ir.go | head -10
echo '```'
echo ""

echo "### AST Types (pkg/ast/ast.go)"
echo '```go'
grep -E "^type .* (struct|interface)" pkg/ast/ast.go | head -10
echo '```'
echo ""

# 6. Compilation Pipeline
echo "## 6. Compilation Pipeline"
echo ""
echo "Based on code analysis, the compilation flow is:"
echo '```'
echo "1. Source (.minz) → Parser (tree-sitter) → AST"
echo "2. AST → Semantic Analysis → IR + Type Checking"
echo "3. IR → Optimization Passes → Optimized IR"
echo "4. Optimized IR → Code Generation → Target Output"
echo '```'
echo ""

# 7. Backend Analysis
echo "## 7. Backend Support"
echo ""
echo "### Available Backends (pkg/codegen/)"
echo '```'
ls pkg/codegen/*backend*.go 2>/dev/null | sed 's/.*\///' | sed 's/_backend.go//' | sort
echo '```'
echo ""

# 8. Build System Analysis
echo "## 8. Build System and Scripts"
echo ""
echo "### Key Scripts"
echo "- **Makefile** targets:"
grep -E "^[a-z-]+:" Makefile | head -10 | sed 's/:.*/ /'
echo ""
echo "### Shell Scripts"
find scripts -name "*.sh" -o -name "*.py" | sort | while read script; do
    echo "- **$script**"
    head -3 "$script" | grep -E "^#" | head -1 | sed 's/^#/  /'
done
echo ""

# 9. Test Infrastructure
echo "## 9. Test Infrastructure"
echo ""
echo "### Test Files"
echo '```'
find . -name "*_test.go" | wc -l | xargs echo "Go test files:"
find tests -name "*.minz" 2>/dev/null | wc -l | xargs echo "MinZ test programs:"
find examples -name "*.minz" | wc -l | xargs echo "Example programs:"
echo '```'
echo ""

# 10. Dead Code Detection
echo "## 10. Potential Dead Code"
echo ""
echo "### Unused Go Files (not imported by any other file)"
echo '```'
for file in $(find pkg -name "*.go" | grep -v _test.go); do
    basename=$(basename "$file" .go)
    if ! grep -q "\".*$basename\"" pkg/**/*.go cmd/**/*.go 2>/dev/null; then
        grep -l "package" "$file" >/dev/null 2>&1 && echo "$file"
    fi
done | head -10
echo '```'
echo ""

# 11. Function Statistics
echo "## 11. Function Statistics"
echo ""
echo "### Exported Functions per Package"
echo '```'
for pkg in pkg/*/; do
    if [ -d "$pkg" ]; then
        exported=$(grep -h "^func [A-Z]" "$pkg"*.go 2>/dev/null | wc -l | xargs)
        total=$(grep -h "^func " "$pkg"*.go 2>/dev/null | wc -l | xargs)
        echo "$(basename $pkg): $exported exported / $total total"
    fi
done
echo '```'

# 12. External Dependencies
echo ""
echo "## 12. External Dependencies"
echo '```'
grep -E "^\s*(github\.com|golang\.org|gopkg\.in)" go.mod | grep -v "minz/minzc" | head -10
echo '```'