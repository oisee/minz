#!/bin/bash
# Run MinZ corpus tests

set -e

# Get the script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/.."

# Set MinZ root for test runner
export MINZ_ROOT="$PROJECT_ROOT"

# Parse command line arguments
VERBOSE=""
FILTER=""
TIMEOUT="60s"

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE="-v"
            export VERBOSE=1
            shift
            ;;
        -f|--filter)
            FILTER="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  -v, --verbose    Enable verbose output"
            echo "  -f, --filter     Filter tests by pattern (e.g., 'basic' or 'fibonacci')"
            echo "  -t, --timeout    Test timeout (default: 60s)"
            echo "  -h, --help       Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Build the compiler first
echo "Building MinZ compiler..."
cd "$PROJECT_ROOT"
make build

# Run corpus tests
echo "Running MinZ corpus tests..."
cd "$PROJECT_ROOT/pkg/z80testing"

if [ -n "$FILTER" ]; then
    echo "Running tests matching: $FILTER"
    go test -timeout "$TIMEOUT" $VERBOSE -run "TestCorpus.*$FILTER.*"
else
    echo "Running all corpus tests..."
    go test -timeout "$TIMEOUT" $VERBOSE -run TestCorpus
fi

# Check if results directory was created
RESULTS_DIR="$PROJECT_ROOT/tests/corpus/results"
if [ -d "$RESULTS_DIR" ] && [ -f "$RESULTS_DIR/summary.json" ]; then
    echo ""
    echo "Test results saved to: $RESULTS_DIR/summary.json"
    cat "$RESULTS_DIR/summary.json"
fi