.PHONY: build test clean run

# Build the compiler
build:
	go build -o minzc cmd/minzc/main.go

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f minzc
	rm -f *.a80

# Build and run on test file
run: build
	./minzc ../examples/simple_add.minz

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run TSMC benchmarks
benchmark:
	go test -v ./pkg/z80testing -run TestTSMCBenchmarkSuite

# Generate performance report
perf-report:
	go run scripts/generate_performance_report.go -output .

# Run benchmarks and generate report
benchmark-report: benchmark perf-report