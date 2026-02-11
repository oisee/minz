# MinZ Root Makefile
# Wraps minzc toolchain build

.PHONY: all build clean test install install-user help

# Default: build everything
all: build

# Build all tools
build:
	cd minzc && $(MAKE) all

# Clean build artifacts
clean:
	cd minzc && $(MAKE) clean

# Run tests
test:
	cd minzc && $(MAKE) test

# Install to /usr/local/bin
install:
	cd minzc && $(MAKE) install

# Install to ~/bin
install-user:
	cd minzc && $(MAKE) install-user

# Show help
help:
	@echo "MinZ Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build        - Build all MinZ tools (mz, mza, mze, mzv, mzr, mzrun, mztap)"
	@echo "  clean        - Remove built executables"
	@echo "  test         - Run basic tests"
	@echo "  install      - Install to /usr/local/bin (may need sudo)"
	@echo "  install-user - Install to ~/bin"
	@echo "  help         - Show this help"
	@echo ""
	@echo "Quick start:"
	@echo "  make && make install-user"
	@echo ""
	@echo "Then compile CP/M hello world:"
	@echo "  mz examples/cpm_hello.minz -o hello.a80 --target=cpm"
	@echo "  mza hello.a80 -o hello.com"
