#!/bin/bash
# Clean E2E Benchmark without verbose output

echo "═══════════════════════════════════════════════════════════════════"
echo "       MinZ E2E Compilation Pipeline Benchmark v1.0"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

# Suppress optimization output
export MINZ_QUIET=1

# Initialize counters
total=0
ast_ok=0
mir_ok=0
z80_ok=0
c_ok=0
crystal_ok=0

# Test all examples
echo "Testing examples/*.minz files..."
echo ""

for file in examples/*.minz; do
    if [ -f "$file" ]; then
        name=$(basename "$file" .minz)
        printf "%-30s " "$name:"
        
        total=$((total + 1))
        
        # Test AST
        if minzc/mz "$file" --dump-ast > /tmp/test.ast 2>/dev/null; then
            ast_ok=$((ast_ok + 1))
            printf "AST✓ "
        else
            printf "AST✗ "
        fi
        
        # Test MIR
        if minzc/mz "$file" --dump-mir > /tmp/test.mir 2>/dev/null; then
            mir_ok=$((mir_ok + 1))
            printf "MIR✓ "
        else
            printf "MIR✗ "
        fi
        
        # Test Z80
        if minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
            z80_ok=$((z80_ok + 1))
            printf "Z80✓ "
        else
            printf "Z80✗ "
        fi
        
        # Test C
        if minzc/mz "$file" -b c -o /tmp/test.c 2>/dev/null; then
            c_ok=$((c_ok + 1))
            printf "C✓ "
        else
            printf "C✗ "
        fi
        
        # Test Crystal
        if minzc/mz "$file" -b crystal -o /tmp/test.cr 2>/dev/null; then
            crystal_ok=$((crystal_ok + 1))
            printf "CR✓"
        else
            printf "CR✗"
        fi
        
        echo ""
    fi
done

# Calculate percentages
if [ $total -gt 0 ]; then
    ast_pct=$((ast_ok * 100 / total))
    mir_pct=$((mir_ok * 100 / total))
    z80_pct=$((z80_ok * 100 / total))
    c_pct=$((c_ok * 100 / total))
    crystal_pct=$((crystal_ok * 100 / total))
else
    ast_pct=0
    mir_pct=0
    z80_pct=0
    c_pct=0
    crystal_pct=0
fi

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "                      BENCHMARK RESULTS"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "📊 Success Rates (out of $total files):"
echo ""
printf "  %-20s %3d/%-3d  %3d%%  " "AST Generation:" $ast_ok $total $ast_pct
for i in $(seq 1 $((ast_pct/2))); do printf "█"; done
echo ""
printf "  %-20s %3d/%-3d  %3d%%  " "MIR Generation:" $mir_ok $total $mir_pct
for i in $(seq 1 $((mir_pct/2))); do printf "█"; done
echo ""
printf "  %-20s %3d/%-3d  %3d%%  " "Z80 Backend:" $z80_ok $total $z80_pct
for i in $(seq 1 $((z80_pct/2))); do printf "█"; done
echo ""
printf "  %-20s %3d/%-3d  %3d%%  " "C Backend:" $c_ok $total $c_pct
for i in $(seq 1 $((c_pct/2))); do printf "█"; done
echo ""
printf "  %-20s %3d/%-3d  %3d%%  " "Crystal Backend:" $crystal_ok $total $crystal_pct
for i in $(seq 1 $((crystal_pct/2))); do printf "█"; done
echo ""
echo ""

# Overall score
overall=$(( (ast_pct + mir_pct + z80_pct + c_pct + crystal_pct) / 5 ))
echo "🏆 Overall Health Score: ${overall}%"
if [ $overall -ge 80 ]; then
    echo "   ✅ Excellent - Production Ready!"
elif [ $overall -ge 60 ]; then
    echo "   ⚠️  Good - Minor Issues"
elif [ $overall -ge 40 ]; then
    echo "   ⚠️  Fair - Needs Work"
else
    echo "   ❌ Poor - Major Issues"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════════"