#!/bin/bash

# E2E Test Runner for MinZ TSMC Performance Testing

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

echo "MinZ E2E Test Runner"
echo "===================="
echo ""

# Check if minzc is built
if [ ! -f "$PROJECT_ROOT/minzc" ]; then
    echo "Building MinZ compiler..."
    cd "$PROJECT_ROOT"
    make build
fi

# Check if sjasmplus is available
if [ ! -f "/Users/alice/dev/bin/sjasmplus" ]; then
    echo "Error: sjasmplus not found at /Users/alice/dev/bin/sjasmplus"
    echo "Please install sjasmplus to run E2E tests"
    exit 1
fi

# Run the E2E tests
echo "Running E2E tests..."
cd "$PROJECT_ROOT"

# Set test timeout
export TESTTIMEOUT=120s

# Run specific E2E test files
echo ""
echo "Running E2E harness tests..."
go test -v ./pkg/z80testing -run TestE2E -timeout $TESTTIMEOUT

# Run benchmarks if requested
if [ "$1" == "--bench" ]; then
    echo ""
    echo "Running E2E benchmarks..."
    go test -bench=BenchmarkE2ETSMC ./pkg/z80testing -benchtime=10s
fi

# Generate coverage report if requested
if [ "$1" == "--coverage" ]; then
    echo ""
    echo "Generating coverage report..."
    go test -coverprofile=e2e_coverage.out ./pkg/z80testing -run TestE2E
    go tool cover -html=e2e_coverage.out -o e2e_coverage.html
    echo "Coverage report saved to e2e_coverage.html"
fi

echo ""
echo "E2E tests completed!"