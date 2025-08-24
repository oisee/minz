#!/bin/bash
# Comprehensive E2E Benchmark for MinZ Compiler
# Tests all stages: MinZ -> AST -> MIR -> (Z80/C/Crystal)

echo "═══════════════════════════════════════════════════════════════════"
echo "       MinZ E2E Compilation Pipeline Benchmark v1.0"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "Date: $(date)"
echo "Compiler: $(minzc/mz --version 2>/dev/null || echo 'v0.15.0')"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test categories
SIMPLE_TESTS=(
    "examples/fibonacci.minz"
    "examples/simple_add.minz"
    "examples/hello_world.minz"
    "examples/arithmetic.minz"
    "examples/control_flow.minz"
)

MEDIUM_TESTS=(
    "examples/arrays.minz"
    "examples/structs.minz"
    "examples/functions.minz"
    "examples/loops.minz"
    "examples/strings.minz"
)

COMPLEX_TESTS=(
    "examples/enums.minz"
    "examples/interfaces.minz"
    "examples/lambdas.minz"
    "examples/error_handling.minz"
    "examples/modules.minz"
)

ADVANCED_TESTS=(
    "examples/metaprogramming.minz"
    "examples/self_modifying_code.minz"
    "examples/pattern_matching.minz"
    "examples/generics.minz"
    "examples/iterators.minz"
)

# Initialize counters
total_tests=0
ast_success=0
mir_success=0
z80_success=0
c_success=0
crystal_success=0

# Timing variables
ast_time=0
mir_time=0
z80_time=0
c_time=0
crystal_time=0

# Function to test a single file
test_file() {
    local file=$1
    local category=$2
    
    if [ ! -f "$file" ]; then
        return 1
    fi
    
    echo -n "  $(basename $file .minz): "
    
    # Test AST generation
    start=$(date +%s%N)
    if minzc/mz "$file" --dump-ast > /tmp/test.ast 2>/dev/null; then
        ast_success=$((ast_success + 1))
        echo -n "${GREEN}AST✓${NC} "
    else
        echo -n "${RED}AST✗${NC} "
    fi
    end=$(date +%s%N)
    ast_time=$((ast_time + (end - start)))
    
    # Test MIR generation
    start=$(date +%s%N)
    if minzc/mz "$file" --dump-mir > /tmp/test.mir 2>/dev/null; then
        mir_success=$((mir_success + 1))
        echo -n "${GREEN}MIR✓${NC} "
    else
        echo -n "${RED}MIR✗${NC} "
    fi
    end=$(date +%s%N)
    mir_time=$((mir_time + (end - start)))
    
    # Test Z80 backend
    start=$(date +%s%N)
    if minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
        z80_success=$((z80_success + 1))
        echo -n "${GREEN}Z80✓${NC} "
    else
        echo -n "${RED}Z80✗${NC} "
    fi
    end=$(date +%s%N)
    z80_time=$((z80_time + (end - start)))
    
    # Test C backend
    start=$(date +%s%N)
    if minzc/mz "$file" -b c -o /tmp/test.c 2>/dev/null; then
        c_success=$((c_success + 1))
        echo -n "${GREEN}C✓${NC} "
    else
        echo -n "${RED}C✗${NC} "
    fi
    end=$(date +%s%N)
    c_time=$((c_time + (end - start)))
    
    # Test Crystal backend
    start=$(date +%s%N)
    if minzc/mz "$file" -b crystal -o /tmp/test.cr 2>/dev/null; then
        crystal_success=$((crystal_success + 1))
        echo -n "${GREEN}CR✓${NC} "
    else
        echo -n "${RED}CR✗${NC} "
    fi
    end=$(date +%s%N)
    crystal_time=$((crystal_time + (end - start)))
    
    echo ""
    total_tests=$((total_tests + 1))
}

# Function to test a category
test_category() {
    local category=$1
    shift
    local tests=("$@")
    
    echo ""
    echo "${CYAN}═══ $category Tests ═══${NC}"
    
    for test in "${tests[@]}"; do
        test_file "$test" "$category"
    done
}

# Run all tests
echo "${BLUE}Starting E2E Benchmark...${NC}"
echo ""

test_category "SIMPLE" "${SIMPLE_TESTS[@]}"
test_category "MEDIUM" "${MEDIUM_TESTS[@]}"
test_category "COMPLEX" "${COMPLEX_TESTS[@]}"
test_category "ADVANCED" "${ADVANCED_TESTS[@]}"

# Also test all examples that exist
echo ""
echo "${CYAN}═══ All Available Examples ═══${NC}"
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        # Skip if already tested
        already_tested=0
        for tested in "${SIMPLE_TESTS[@]}" "${MEDIUM_TESTS[@]}" "${COMPLEX_TESTS[@]}" "${ADVANCED_TESTS[@]}"; do
            if [ "$file" = "$tested" ]; then
                already_tested=1
                break
            fi
        done
        
        if [ $already_tested -eq 0 ]; then
            test_file "$file" "OTHER"
        fi
    fi
done

# Calculate percentages
ast_percent=$((ast_success * 100 / total_tests))
mir_percent=$((mir_success * 100 / total_tests))
z80_percent=$((z80_success * 100 / total_tests))
c_percent=$((c_success * 100 / total_tests))
crystal_percent=$((crystal_success * 100 / total_tests))

# Convert nanoseconds to milliseconds
ast_time_ms=$((ast_time / 1000000))
mir_time_ms=$((mir_time / 1000000))
z80_time_ms=$((z80_time / 1000000))
c_time_ms=$((c_time / 1000000))
crystal_time_ms=$((crystal_time / 1000000))

# Display results
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "                      BENCHMARK RESULTS"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "📊 ${YELLOW}Success Rates:${NC}"
echo "  ├─ AST Generation:    ${ast_success}/${total_tests} (${ast_percent}%)"
echo "  ├─ MIR Generation:    ${mir_success}/${total_tests} (${mir_percent}%)"
echo "  ├─ Z80 Backend:       ${z80_success}/${total_tests} (${z80_percent}%)"
echo "  ├─ C Backend:         ${c_success}/${total_tests} (${c_percent}%)"
echo "  └─ Crystal Backend:   ${crystal_success}/${total_tests} (${crystal_percent}%)"
echo ""
echo "⏱️  ${YELLOW}Performance (Total Time):${NC}"
echo "  ├─ AST Generation:    ${ast_time_ms}ms"
echo "  ├─ MIR Generation:    ${mir_time_ms}ms"
echo "  ├─ Z80 Backend:       ${z80_time_ms}ms"
echo "  ├─ C Backend:         ${c_time_ms}ms"
echo "  └─ Crystal Backend:   ${crystal_time_ms}ms"
echo ""

# Performance summary
total_time_ms=$((ast_time_ms + mir_time_ms + z80_time_ms + c_time_ms + crystal_time_ms))
echo "📈 ${YELLOW}Performance Summary:${NC}"
echo "  ├─ Total Compilation Time: ${total_time_ms}ms"
if [ $total_tests -gt 0 ]; then
    avg_time=$((total_time_ms / total_tests / 5))  # Divided by 5 stages
    echo "  ├─ Average per Stage:      ${avg_time}ms"
fi
echo "  └─ Files Processed:        ${total_tests}"
echo ""

# Stage analysis
echo "🔍 ${YELLOW}Stage Analysis:${NC}"
echo ""
echo "  Stage      | Success | Rate  | Time    | Avg/File"
echo "  -----------|---------|-------|---------|----------"
if [ $total_tests -gt 0 ]; then
    printf "  AST        | %7d | %4d%% | %6dms | %5dms\n" $ast_success $ast_percent $ast_time_ms $((ast_time_ms / total_tests))
    printf "  MIR        | %7d | %4d%% | %6dms | %5dms\n" $mir_success $mir_percent $mir_time_ms $((mir_time_ms / total_tests))
    printf "  Z80        | %7d | %4d%% | %6dms | %5dms\n" $z80_success $z80_percent $z80_time_ms $((z80_time_ms / total_tests))
    printf "  C          | %7d | %4d%% | %6dms | %5dms\n" $c_success $c_percent $c_time_ms $((c_time_ms / total_tests))
    printf "  Crystal    | %7d | %4d%% | %6dms | %5dms\n" $crystal_success $crystal_percent $crystal_time_ms $((crystal_time_ms / total_tests))
fi
echo ""

# Overall health score
health_score=$(( (ast_percent + mir_percent + z80_percent + c_percent + crystal_percent) / 5 ))
echo "🏆 ${YELLOW}Overall Health Score: ${health_score}%${NC}"
if [ $health_score -ge 80 ]; then
    echo "   ${GREEN}✅ Excellent - Production Ready!${NC}"
elif [ $health_score -ge 60 ]; then
    echo "   ${YELLOW}⚠️ Good - Minor Issues${NC}"
elif [ $health_score -ge 40 ]; then
    echo "   ${YELLOW}⚠️ Fair - Needs Work${NC}"
else
    echo "   ${RED}❌ Poor - Major Issues${NC}"
fi
echo ""

# Backend comparison
echo "⚔️  ${YELLOW}Backend Comparison:${NC}"
best_backend="Z80"
best_rate=$z80_percent
if [ $c_percent -gt $best_rate ]; then
    best_backend="C"
    best_rate=$c_percent
fi
if [ $crystal_percent -gt $best_rate ]; then
    best_backend="Crystal"
    best_rate=$crystal_percent
fi
echo "   Best Backend: $best_backend ($best_rate% success)"
echo ""

# Save detailed report
report_file="E2E_BENCHMARK_$(date +%Y%m%d_%H%M%S).md"
{
    echo "# MinZ E2E Benchmark Report"
    echo ""
    echo "Date: $(date)"
    echo ""
    echo "## Summary"
    echo "- Total Tests: $total_tests"
    echo "- Overall Health: ${health_score}%"
    echo "- Best Backend: $best_backend ($best_rate%)"
    echo ""
    echo "## Success Rates"
    echo "| Stage | Success | Rate |"
    echo "|-------|---------|------|"
    echo "| AST | $ast_success/$total_tests | ${ast_percent}% |"
    echo "| MIR | $mir_success/$total_tests | ${mir_percent}% |"
    echo "| Z80 | $z80_success/$total_tests | ${z80_percent}% |"
    echo "| C | $c_success/$total_tests | ${c_percent}% |"
    echo "| Crystal | $crystal_success/$total_tests | ${crystal_percent}% |"
    echo ""
    echo "## Performance"
    echo "| Stage | Total Time | Avg/File |"
    echo "|-------|------------|----------|"
    if [ $total_tests -gt 0 ]; then
        echo "| AST | ${ast_time_ms}ms | $((ast_time_ms / total_tests))ms |"
        echo "| MIR | ${mir_time_ms}ms | $((mir_time_ms / total_tests))ms |"
        echo "| Z80 | ${z80_time_ms}ms | $((z80_time_ms / total_tests))ms |"
        echo "| C | ${c_time_ms}ms | $((c_time_ms / total_tests))ms |"
        echo "| Crystal | ${crystal_time_ms}ms | $((crystal_time_ms / total_tests))ms |"
    fi
} > "$report_file"

echo "═══════════════════════════════════════════════════════════════════"
echo "📄 Detailed report saved to: $report_file"
echo "═══════════════════════════════════════════════════════════════════"