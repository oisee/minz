.PHONY: build test clean run release repl all

# Build both compiler and REPL
all: build repl

# Build the compiler
build:
	go build -o mz cmd/minzc/main.go
	@ln -sf mz minzc  # Keep backward compatibility

# Build the REPL
repl:
	go build -o mzr cmd/repl/main.go cmd/repl/compiler.go
	@ln -sf mzr minz  # Keep backward compatibility

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f mz mzr minzc minz
	rm -f *.a80
	rm -f *.mir

# Build and run on test file
run: build
	./mz ../examples/simple_add.minz

# Run the REPL
run-repl: repl
	./mzr

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
	go build -ldflags="-s -w" -o ../release/bin/minz cmd/repl/main.go cmd/repl/compiler.go
	
	# Cross-compile compiler
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minzc-darwin-amd64 cmd/minzc/main.go
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../release/bin/minzc-darwin-arm64 cmd/minzc/main.go
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minzc-linux-amd64 cmd/minzc/main.go
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../release/bin/minzc-linux-arm64 cmd/minzc/main.go
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minzc-windows-amd64.exe cmd/minzc/main.go
	
	# Cross-compile REPL
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minz-darwin-amd64 cmd/repl/main.go cmd/repl/compiler.go
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../release/bin/minz-darwin-arm64 cmd/repl/main.go cmd/repl/compiler.go
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minz-linux-amd64 cmd/repl/main.go cmd/repl/compiler.go
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../release/bin/minz-linux-arm64 cmd/repl/main.go cmd/repl/compiler.go
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../release/bin/minz-windows-amd64.exe cmd/repl/main.go cmd/repl/compiler.go
	
	@echo "Release binaries built in ../release/bin/"

# Install locally (both compiler and REPL)
install: all
	@echo "Installing MinZ tools to /usr/local/bin..."
	@sudo cp mz /usr/local/bin/
	@sudo cp mzr /usr/local/bin/
	@sudo ln -sf /usr/local/bin/mz /usr/local/bin/minzc
	@sudo ln -sf /usr/local/bin/mzr /usr/local/bin/minz
	@echo "Installation complete!"
	@echo "  mz    - MinZ compiler (also: minzc)"
	@echo "  mzr   - MinZ REPL (also: minz)"

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