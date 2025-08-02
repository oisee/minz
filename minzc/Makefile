.PHONY: build test clean run release

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

# Release build (all platforms)
release:
	@echo "Building release binaries..."
	@mkdir -p ../release/bin
	
	# Current platform
	go build -ldflags="-s -w" -o ../release/bin/minzc cmd/minzc/main.go
	
	# Cross-compile
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minzc-darwin-amd64 cmd/minzc/main.go
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../release/bin/minzc-darwin-arm64 cmd/minzc/main.go
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minzc-linux-amd64 cmd/minzc/main.go
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../release/bin/minzc-linux-arm64 cmd/minzc/main.go
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minzc-windows-amd64.exe cmd/minzc/main.go
	
	@echo "Release binaries built in ../release/bin/"

# Install locally
install: build
	@echo "Installing minzc to /usr/local/bin..."
	@sudo cp minzc /usr/local/bin/
	@echo "Installation complete!"

# Development build with race detector
dev:
	go build -race -o minzc cmd/minzc/main.go

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"