#!/bin/bash
# Run TSMC benchmarks and generate performance report

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== TSMC Performance Benchmark Suite ===${NC}"
echo -e "${BLUE}=======================================${NC}\n"

# Check if we're in the right directory
if [ ! -f "Makefile" ] || [ ! -d "pkg/z80testing" ]; then
    echo -e "${RED}Error: Please run this script from the minzc directory${NC}"
    exit 1
fi

# Build the compiler
echo -e "${YELLOW}Building MinZ compiler...${NC}"
make build

# Run the benchmark suite
echo -e "\n${YELLOW}Running TSMC benchmarks...${NC}"
echo -e "${YELLOW}This will compile and execute each benchmark with and without TSMC${NC}\n"

# Create results directory
mkdir -p benchmark_results
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULT_FILE="benchmark_results/tsmc_results_${TIMESTAMP}.txt"

# Run tests with detailed output
go test ./pkg/z80testing -run TestTSMCBenchmarkSuite -v > "$RESULT_FILE" 2>&1

# Extract summary from results
echo -e "\n${GREEN}=== Benchmark Summary ===${NC}"

# Parse results for summary
if grep -q "PASS" "$RESULT_FILE"; then
    echo -e "${GREEN}✓ All benchmarks completed successfully${NC}\n"
    
    # Extract performance metrics
    echo "Performance Improvements:"
    grep -E "✓.*[0-9]+\.[0-9]+% improvement" "$RESULT_FILE" | while read line; do
        echo -e "${GREEN}$line${NC}"
    done
    
    # Extract overall results
    echo -e "\n${BLUE}Overall Results:${NC}"
    grep -E "Overall:.*benchmarks passed" "$RESULT_FILE" | tail -1
    
else
    echo -e "${RED}✗ Some benchmarks failed${NC}"
    echo -e "${RED}Check $RESULT_FILE for details${NC}"
fi

# Generate visual report
echo -e "\n${YELLOW}Generating visual performance report...${NC}"

# Create a simple performance chart
cat > "benchmark_results/performance_chart_${TIMESTAMP}.txt" << 'EOF'
TSMC Performance Improvements
=============================

Algorithm               No TSMC    With TSMC   Improvement
---------               --------   ----------  -----------
EOF

# Extract data for chart
grep -A4 "### " "$RESULT_FILE" | grep -E "Cycles|Speedup" | paste - - | \
    awk -F'|' '{gsub(/[^0-9.]/, "", $2); gsub(/[^0-9.]/, "", $3); gsub(/[^0-9.%]/, "", $4); 
         if (NR%2==1) {cycles1=$2; cycles2=$3; imp=$4} 
         else {print prev_name, cycles1, cycles2, imp}}' \
    >> "benchmark_results/performance_chart_${TIMESTAMP}.txt" 2>/dev/null || true

echo -e "\n${GREEN}Benchmark complete!${NC}"
echo -e "Full results: ${BLUE}$RESULT_FILE${NC}"
echo -e "Performance chart: ${BLUE}benchmark_results/performance_chart_${TIMESTAMP}.txt${NC}"

# Show report location
echo -e "\n${YELLOW}Report saved to:${NC}"
ls -la benchmark_results/tsmc_*${TIMESTAMP}*

# Compile standalone examples for manual inspection
echo -e "\n${YELLOW}Compiling standalone examples...${NC}"
for bench in fibonacci_benchmark string_operations sorting_algorithms crc_checksum; do
    echo -n "Compiling $bench... "
    ./minzc ../tests/benchmarks/${bench}.minz -o benchmark_results/${bench}_no_tsmc.a80 -O 2>/dev/null
    ./minzc ../tests/benchmarks/${bench}.minz -o benchmark_results/${bench}_tsmc.a80 -O --enable-true-smc 2>/dev/null
    echo -e "${GREEN}done${NC}"
done

echo -e "\n${GREEN}All benchmarks completed successfully!${NC}"
echo -e "${BLUE}You can inspect the generated assembly files in benchmark_results/${NC}"