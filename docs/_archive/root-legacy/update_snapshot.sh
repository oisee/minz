#!/bin/bash
# Update compiler snapshot with current metrics

SNAPSHOT_FILE="COMPILER_SNAPSHOT.md"
TIMESTAMP=$(date +"%Y-%m-%d")

echo "📊 Updating MinZ Compiler Snapshot..."

# Function to count success rate
count_success_rate() {
    local dir=$1
    local total=0
    local success=0
    
    for file in "$dir"/*.minz; do
        if [ -f "$file" ]; then
            ((total++))
            if ./minzc/minzc "$file" -o /tmp/test.a80 >/dev/null 2>&1; then
                ((success++))
            fi
        fi
    done
    
    if [ $total -gt 0 ]; then
        echo "$success/$total"
    else
        echo "0/0"
    fi
}

# Update the timestamp
sed -i.bak "s/\*\*Last Updated:\*\* .*/\*\*Last Updated:\*\* $TIMESTAMP/" "$SNAPSHOT_FILE"

# Count grammar keywords
KEYWORDS=$(grep -o '\b\(const\|fun\|let\|mut\|if\|else\|while\|for\|break\|continue\|return\|struct\|enum\|interface\|impl\|import\|export\|pub\|true\|false\|nil\|as\|in\|match\|@asm\|@abi\|@lua\|@macro\)\b' grammar.js | sort | uniq | wc -l)
echo "✓ Keywords in grammar: $KEYWORDS"

# Test compilation success rates
echo "🧪 Testing compilation success rates..."
cd minzc && make build >/dev/null 2>&1 && cd ..

# Test core features
CORE_SUCCESS=$(count_success_rate "examples")
echo "✓ Core language success: $CORE_SUCCESS"

# Test with optimizations
OPT_SUCCESS=0
OPT_TOTAL=0
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        ((OPT_TOTAL++))
        if ./minzc/minzc "$file" -O --enable-smc -o /tmp/test.a80 >/dev/null 2>&1; then
            ((OPT_SUCCESS++))
        fi
    fi
done
echo "✓ Optimization success: $OPT_SUCCESS/$OPT_TOTAL"

# Count test files
PARSER_TESTS=$(find test -name "*.txt" 2>/dev/null | wc -l)
E2E_TESTS=$(find examples -name "*.minz" | wc -l)
echo "✓ Parser tests: $PARSER_TESTS"
echo "✓ E2E tests: $E2E_TESTS"

# Detect optimization patterns in generated assembly
echo "🔍 Analyzing optimization patterns..."
TSMC_COUNT=0
XOR_OPT_COUNT=0
DJNZ_COUNT=0

for file in minzc/comprehensive_test_results/*_optimized.a80 2>/dev/null; do
    if [ -f "$file" ]; then
        # Count TRUE SMC patterns
        if grep -q "immOP:" "$file"; then
            ((TSMC_COUNT++))
        fi
        # Count XOR optimizations
        if grep -q "XOR A" "$file"; then
            ((XOR_OPT_COUNT++))
        fi
        # Count DJNZ patterns
        if grep -q "DJNZ" "$file"; then
            ((DJNZ_COUNT++))
        fi
    fi
done

echo "✓ TRUE SMC functions: $TSMC_COUNT"
echo "✓ XOR optimizations: $XOR_OPT_COUNT"
echo "✓ DJNZ loops: $DJNZ_COUNT"

# Run issue detection
echo "🐛 Running issue detection..."
if [ -f scripts/detect_issues.go ]; then
    go run scripts/detect_issues.go minzc/comprehensive_test_results/ > /tmp/issues_report.md 2>/dev/null
    CRITICAL_ISSUES=$(grep -c "Critical Issues" /tmp/issues_report.md 2>/dev/null || echo 0)
    WARNING_ISSUES=$(grep -c "Warnings" /tmp/issues_report.md 2>/dev/null || echo 0)
    echo "✓ Critical issues: $CRITICAL_ISSUES"
    echo "✓ Warnings: $WARNING_ISSUES"
fi

# Generate metrics summary
cat > /tmp/metrics_summary.md << EOF
## 📊 Current Metrics (Auto-Updated)

| Metric | Value | Change |
|--------|-------|--------|
| Compilation Success | ${CORE_SUCCESS} | - |
| Optimization Success | ${OPT_SUCCESS}/${OPT_TOTAL} | - |
| TRUE SMC Functions | ${TSMC_COUNT} | - |
| XOR Optimizations | ${XOR_OPT_COUNT} | - |
| Parser Tests | ${PARSER_TESTS} | - |
| E2E Tests | ${E2E_TESTS} | - |

*Last automated update: ${TIMESTAMP}*
EOF

echo ""
echo "✅ Snapshot update complete!"
echo ""
echo "📋 Summary:"
cat /tmp/metrics_summary.md

# Cleanup
rm -f /tmp/test.a80 /tmp/issues_report.md "$SNAPSHOT_FILE.bak"