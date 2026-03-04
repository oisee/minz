#!/bin/bash
# Run MIR Backend Tests
# Tests the MIR→Z80→binary→emulate pipeline for correctness and performance

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/.."

# Parse command line arguments
VERBOSE="-v"
BENCH=""
SUMMARY=""
TIMEOUT="120s"

while [[ $# -gt 0 ]]; do
    case $1 in
        --bench)
            BENCH=1
            shift
            ;;
        --summary)
            SUMMARY=1
            shift
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -q|--quiet)
            VERBOSE=""
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --bench      Run T-state benchmarks after tests"
            echo "  --summary    Run summary table (all tests in one table)"
            echo "  -t, --timeout  Test timeout (default: 120s)"
            echo "  -q, --quiet    Suppress verbose output"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

cd "$PROJECT_ROOT"

# Run individual tests
echo "=== MIR Backend Tests ==="
echo ""
go test ./pkg/codegen/ -run "^TestMIRBackend$" $VERBOSE -timeout "$TIMEOUT" -count=1 -vet=off
echo ""

# Run summary table if requested
if [ -n "$SUMMARY" ]; then
    echo "=== MIR Backend Summary ==="
    echo ""
    go test ./pkg/codegen/ -run "^TestMIRBackendSummary$" -v -timeout "$TIMEOUT" -count=1 -vet=off
    echo ""
fi

# Run benchmarks if requested
if [ -n "$BENCH" ]; then
    echo "=== MIR Backend Benchmarks ==="
    echo ""
    go test ./pkg/codegen/ -run "^$" -bench "^BenchmarkMIRBackend$" -timeout "$TIMEOUT" -count=1 -vet=off -benchmem
    echo ""
fi

echo "=== Done ==="
