# MinZ Compiler Toolchain Makefile

# Installation directory (can be overridden)
PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin

# Version info for mzx
MZX_VERSION := 0.1.0
MZX_BUILD_FILE := cmd/mzx/BUILD
MZX_BUILD_NUM = $(shell cat $(MZX_BUILD_FILE) 2>/dev/null || echo 0)
MZX_GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
MZX_BUILD_DATE = $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
MZX_LDFLAGS = -X 'main.version=$(MZX_VERSION)' -X 'main.buildNum=$(MZX_BUILD_NUM)' -X 'main.gitCommit=$(MZX_GIT_COMMIT)' -X 'main.buildDate=$(MZX_BUILD_DATE)'

# Build all tools
.PHONY: all
all: mz mza mze mzv mzr mzrun mztap mzd mzx

# Build the main compiler
mz:
	go build -o mz ./cmd/minzc

# Build the assembler
mza:
	go build -o mza ./cmd/mza

# Build the emulator
mze:
	go build -o mze ./cmd/mze

# Build the MZV virtual machine
mzv:
	go build -o mzv ./cmd/mzv

# Build the REPL
mzr:
	go build -o mzr ./cmd/mzr

# Build the DZRP remote runner
mzrun:
	go build -o mzrun ./cmd/mzrun

# Build the TAP loader
mztap:
	go build -o mztap ./cmd/mztap

# Build the disassembler
mzd:
	go build -o mzd ./cmd/mzd

# Build the ZX Spectrum emulator (auto-increments build number)
mzx:
	@echo $$(($(MZX_BUILD_NUM) + 1)) > $(MZX_BUILD_FILE)
	go build -ldflags "$(MZX_LDFLAGS)" -o mzx ./cmd/mzx

# Install to system (may require sudo)
.PHONY: install
install: all
	@echo "Installing MinZ tools to $(BINDIR)..."
	@mkdir -p $(BINDIR)
	@if [ -w $(BINDIR) ]; then \
		cp mz mza mze mzv mzr mzrun mztap mzd mzx $(BINDIR)/ && \
		echo "MinZ tools installed to $(BINDIR)"; \
	else \
		echo "⚠️  $(BINDIR) is not writable. Try: sudo make install"; \
		exit 1; \
	fi

# Install to user's local directory (~/.local/bin)
.PHONY: install-user
install-user: all
	@mkdir -p ~/.local/bin
	@for f in mz mza mze mzv mzr mzrun mztap mzd mzx; do [ -f $$f ] && cp -f $$f ~/.local/bin/ && (codesign -s - ~/.local/bin/$$f 2>/dev/null || true); done
	@echo "MinZ tools installed to ~/.local/bin/"

# Clean build artifacts
.PHONY: clean
clean:
	rm -f mz mza mze mzv mzr mzrun mztap mzd mzx

# Run basic smoke test
.PHONY: test
test: mz mza mze
	@echo "Running basic tests..."
	./mz ../examples/hello_clean.minz -o test.a80
	./mza -o test.bin test.a80
	@echo "Basic compilation test passed"
	@rm -f test.a80 test.bin

# Run ALL Go unit tests across the entire toolchain
.PHONY: test-all
test-all:
	@echo "=== Running all Go unit tests ==="
	@echo ""
	@echo "--- Z80 Emulator (FUSE test suite) ---"
	go test ./pkg/emulator/... -count=1 -v 2>&1 | tail -5
	@echo ""
	@echo "--- Z80 Assembler ---"
	go test ./pkg/z80asm/... -count=1 -v 2>&1 | tail -5
	@echo ""
	@echo "--- Spectrum Emulator (MZX) ---"
	go test ./pkg/spectrum/... -count=1 -v 2>&1 | tail -5
	@echo ""
	@echo "--- Disassembler ---"
	go test ./pkg/disasm/... -count=1 -v 2>&1 | tail -5
	@echo ""
	@echo "--- Parser ---"
	go test ./pkg/parser/... -count=1 -v 2>&1 | tail -5
	@echo ""
	@echo "--- All remaining packages ---"
	go test ./... -count=1 2>&1 | grep -E '(ok|FAIL|---)'
	@echo ""
	@echo "=== Test run complete ==="

# Show help
.PHONY: help
help:
	@echo "MinZ Compiler Toolchain"
	@echo ""
	@echo "Tools:"
	@echo "  mz    - MinZ compiler (generates Z80 assembly)"
	@echo "  mza   - Z80 assembler (generates binary)"
	@echo "  mze   - Z80 emulator (run/debug binaries)"
	@echo "  mzv   - MZV virtual machine (platform abstraction)"
	@echo "  mzr   - MinZ REPL (interactive mode)"
	@echo "  mzrun - DZRP remote runner (run on emulators)"
	@echo "  mztap - TAP file loader (load .tap via DZRP)"
	@echo "  mzd   - Z80 disassembler"
	@echo "  mzx   - ZX Spectrum emulator (T-state accurate)"
	@echo ""
	@echo "Targets:"
	@echo "  all           - Build all tools"
	@echo "  install       - Install to $(BINDIR) (may need sudo)"
	@echo "  install-user  - Install to ~/.local/bin"
	@echo "  clean         - Remove built executables"
	@echo "  test          - Run basic compilation test"
	@echo "  test-all      - Run ALL Go unit tests (emulator, assembler, spectrum, etc.)"
	@echo "  help          - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make              # Build all tools"
	@echo "  make install      # Install to /usr/local/bin"
	@echo "  make install-user # Install to ~/.local/bin"
	@echo "  sudo make install # Install with sudo if needed"