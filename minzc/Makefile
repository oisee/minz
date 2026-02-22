# MinZ Compiler Toolchain Makefile

# Installation directory (can be overridden)
PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin

# Build all tools
.PHONY: all
all: mz mza mze mzv mzr mzrun mztap

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

# Install to system (may require sudo)
.PHONY: install
install: all
	@echo "Installing MinZ tools to $(BINDIR)..."
	@mkdir -p $(BINDIR)
	@if [ -w $(BINDIR) ]; then \
		cp mz mza mze mzv mzr mzrun mztap $(BINDIR)/ && \
		echo "✅ MinZ tools installed to $(BINDIR)"; \
	else \
		echo "⚠️  $(BINDIR) is not writable. Try: sudo make install"; \
		exit 1; \
	fi

# Install to user's local directory (~/.local/bin)
.PHONY: install-user
install-user: all
	@mkdir -p ~/.local/bin
	@for f in mz mza mze mzv mzr mzrun mztap; do [ -f $$f ] && cp -f $$f ~/.local/bin/; done
	@echo "MinZ tools installed to ~/.local/bin/"

# Clean build artifacts
.PHONY: clean
clean:
	rm -f mz mza mze mzv mzr mzrun mztap

# Run tests
.PHONY: test
test: mz mza mze
	@echo "Running basic tests..."
	./mz ../examples/hello_clean.minz -o test.a80
	./mza -o test.bin test.a80
	@echo "✅ Basic compilation test passed"
	@rm -f test.a80 test.bin

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
	@echo ""
	@echo "Targets:"
	@echo "  all           - Build all tools"
	@echo "  install       - Install to $(BINDIR) (may need sudo)"
	@echo "  install-user  - Install to ~/.local/bin"
	@echo "  clean         - Remove built executables"
	@echo "  test          - Run basic compilation test"
	@echo "  help          - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make              # Build all tools"
	@echo "  make install      # Install to /usr/local/bin"
	@echo "  make install-user # Install to ~/.local/bin"
	@echo "  sudo make install # Install with sudo if needed"