.PHONY: build test clean run release repl asm emu all

# Build compiler, REPL, assembler, and emulator
all: build repl asm emu

# Get version info from git
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.10.0")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_NUMBER := $(shell cat .build-number 2>/dev/null || echo "0")

# Increment build number
INCREMENT_BUILD := $(shell echo $$(($(BUILD_NUMBER) + 1)) > .build-number)

# Build flags with version info
LDFLAGS := -ldflags "-X github.com/minz/minzc/pkg/version.Version=$(GIT_TAG) \
	-X github.com/minz/minzc/pkg/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/minz/minzc/pkg/version.GitTag=$(GIT_TAG) \
	-X github.com/minz/minzc/pkg/version.BuildDate=$(BUILD_DATE) \
	-X github.com/minz/minzc/pkg/version.BuildNumber=$(shell cat .build-number)"

# Build the compiler
build:
	@echo "Building MinZ compiler $(GIT_TAG)+$(shell cat .build-number)"
	go build $(LDFLAGS) -o mz cmd/minzc/main.go
	@ln -sf mz minzc  # Keep backward compatibility

# Build the REPL
repl:
	go build -o mzr cmd/repl/main.go cmd/repl/compiler.go
	@ln -sf mzr minz  # Keep backward compatibility

# Build the assembler
asm:
	go build -o mza cmd/mza/main.go

# Build the emulator
emu:
	go build -o mze cmd/mze/main.go

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f mz mzr mza mze minz
	rm -f *.a80
	rm -f *.mir
	rm -f *.bin

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
RELEASE_LDFLAGS := -ldflags "-s -w \
	-X github.com/minz/minzc/pkg/version.Version=$(GIT_TAG) \
	-X github.com/minz/minzc/pkg/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/minz/minzc/pkg/version.GitTag=$(GIT_TAG) \
	-X github.com/minz/minzc/pkg/version.BuildDate=$(BUILD_DATE) \
	-X github.com/minz/minzc/pkg/version.BuildNumber=$(shell cat .build-number)"

release:
	@echo "Building release binaries for $(GIT_TAG)..."
	@mkdir -p ../release/bin
	
	# Current platform
	go build $(RELEASE_LDFLAGS) -o ../release/bin/minzc cmd/minzc/main.go
	go build -ldflags="-s -w" -o ../release/bin/minz cmd/repl/main.go cmd/repl/compiler.go
	
	# Cross-compile compiler
	GOOS=darwin GOARCH=amd64 go build $(RELEASE_LDFLAGS) -o ../release/bin/minzc-darwin-amd64 cmd/minzc/main.go
	GOOS=darwin GOARCH=arm64 go build $(RELEASE_LDFLAGS) -o ../release/bin/minzc-darwin-arm64 cmd/minzc/main.go
	GOOS=linux GOARCH=amd64 go build $(RELEASE_LDFLAGS) -o ../release/bin/minzc-linux-amd64 cmd/minzc/main.go
	GOOS=linux GOARCH=arm64 go build $(RELEASE_LDFLAGS) -o ../release/bin/minzc-linux-arm64 cmd/minzc/main.go
	GOOS=windows GOARCH=amd64 go build $(RELEASE_LDFLAGS) -o ../release/bin/minzc-windows-amd64.exe cmd/minzc/main.go
	
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

# Install to user's local bin (no sudo required)
install-local: build
	@mkdir -p ~/.local/bin
	@echo "Installing MinZ compiler to ~/.local/bin..."
	@cp mz-wrapper.sh ~/.local/bin/mz
	@chmod +x ~/.local/bin/mz
	@ln -sf ~/.local/bin/mz ~/.local/bin/minzc
	@echo "Installation complete!"
	@echo "  mz    - MinZ compiler (also: minzc)"
	@echo "Make sure ~/.local/bin is in your PATH"
	@echo "NOTE: mz will work from any directory!"

# Install grammar files to ~/.local/share/mz/grammar
install-grammar:
	@echo "Installing MinZ grammar files..."
	@mkdir -p ~/.local/share/mz/grammar
	@mkdir -p ~/.local/share/mz/grammar/src
	@mkdir -p ~/.local/share/mz/grammar/src/tree_sitter
	@cp ../grammar.js ~/.local/share/mz/grammar/
	@cp ../src/grammar.json ~/.local/share/mz/grammar/src/
	@cp ../src/parser.c ~/.local/share/mz/grammar/src/
	@cp ../src/node-types.json ~/.local/share/mz/grammar/src/
	@echo '#ifndef TREE_SITTER_PARSER_H_' > ~/.local/share/mz/grammar/src/tree_sitter/parser.h
	@echo '#define TREE_SITTER_PARSER_H_' >> ~/.local/share/mz/grammar/src/tree_sitter/parser.h
	@echo 'typedef struct TSLanguage TSLanguage;' >> ~/.local/share/mz/grammar/src/tree_sitter/parser.h
	@echo '#endif' >> ~/.local/share/mz/grammar/src/tree_sitter/parser.h
	@echo "Grammar files installed to ~/.local/share/mz/grammar/"

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