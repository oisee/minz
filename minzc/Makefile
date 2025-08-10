# MinZ Compiler Toolchain Makefile

# Installation directory (can be overridden)
PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin

# Build all tools
.PHONY: all
all: mz mza mze mzr

# Build the main compiler
mz: 
	go build -o mz ./cmd/minzc

# Build the assembler
mza:
	go build -o mza ./cmd/mza

# Build the emulator
mze:
	go build -o mze ./cmd/mze

# Build the REPL
mzr:
	go build -o mzr ./cmd/mzr

# Install to system (may require sudo)
.PHONY: install
install: all
	@echo "Installing MinZ tools to $(BINDIR)..."
	@mkdir -p $(BINDIR)
	@if [ -w $(BINDIR) ]; then \
		cp mz mza mze mzr $(BINDIR)/ && \
		echo "✅ MinZ tools installed to $(BINDIR)"; \
	else \
		echo "⚠️  $(BINDIR) is not writable. Try: sudo make install"; \
		exit 1; \
	fi

# Install to user's home directory
.PHONY: install-user
install-user: all
	@echo "Installing MinZ tools to ~/bin..."
	@mkdir -p ~/bin
	@cp mz mza mze mzr ~/bin/
	@echo "✅ MinZ tools installed to ~/bin"
	@echo "📝 Make sure ~/bin is in your PATH:"
	@echo '   export PATH="$$HOME/bin:$$PATH"'

# Clean build artifacts
.PHONY: clean
clean:
	rm -f mz mza mze mzr

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
	@echo "Targets:"
	@echo "  all           - Build all tools (mz, mza, mze, mzr)"
	@echo "  install       - Install to $(BINDIR) (may need sudo)"
	@echo "  install-user  - Install to ~/bin (no sudo needed)"
	@echo "  clean         - Remove built executables"
	@echo "  test          - Run basic compilation test"
	@echo "  help          - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make              # Build all tools"
	@echo "  make install      # Install to /usr/local/bin"
	@echo "  make install-user # Install to ~/bin"
	@echo "  sudo make install # Install with sudo if needed"