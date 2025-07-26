# MinZ VSCode Extension Build System
# Based on BPL project structure and best practices

.PHONY: help install build package clean test bump-version deploy install-extension uninstall-extension

# Default target
help:
	@echo "MinZ VSCode Extension Build System"
	@echo "=================================="
	@echo ""
	@echo "Available targets:"
	@echo "  install            - Install dependencies"
	@echo "  build              - Compile TypeScript and build extension"
	@echo "  package            - Create .vsix package"
	@echo "  bump-version       - Increment patch version and build"
	@echo "  test               - Run extension tests"
	@echo "  clean              - Clean build artifacts"
	@echo "  install-extension  - Install the extension locally"
	@echo "  uninstall-extension- Uninstall the extension"
	@echo "  deploy             - Full build and install workflow"
	@echo "  help               - Show this help message"

# Install dependencies
install:
	@echo "Installing dependencies..."
	npm install
	@echo "Dependencies installed successfully!"

# Build TypeScript
build:
	@echo "Building MinZ VSCode extension..."
	npm run compile
	@echo "Build completed!"

# Create VSIX package
package: build
	@echo "Packaging extension..."
	npm run package
	@echo "Package created successfully!"

# Bump version and build
bump-version:
	@echo "Bumping version and building..."
	node scripts/build.js

# Run tests
test:
	@echo "Running tests..."
	npm test

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf out/
	rm -rf node_modules/
	rm -f *.vsix
	@echo "Clean completed!"

# Install the extension locally
install-extension: package
	@echo "Installing MinZ extension locally..."
	@VSIX_FILE=$$(ls -t *.vsix | head -n1); \
	if [ -n "$$VSIX_FILE" ]; then \
		echo "Installing $$VSIX_FILE..."; \
		code --install-extension "$$VSIX_FILE"; \
		echo "Extension installed! Restart VSCode to use."; \
	else \
		echo "No .vsix file found. Run 'make package' first."; \
		exit 1; \
	fi

# Uninstall the extension
uninstall-extension:
	@echo "Uninstalling MinZ extension..."
	code --uninstall-extension minz-language.minz-language-support || true
	@echo "Extension uninstalled!"

# Full deployment workflow
deploy: clean install bump-version install-extension
	@echo "======================="
	@echo "✅ Deployment Complete!"
	@echo "======================="
	@echo "The MinZ VSCode extension has been:"
	@echo "  ✓ Built with latest changes"
	@echo "  ✓ Version bumped"
	@echo "  ✓ Packaged as .vsix"
	@echo "  ✓ Installed locally"
	@echo ""
	@echo "🚀 Restart VSCode to use the updated extension!"
	@echo ""
	@echo "To test the extension:"
	@echo "  1. Open a .minz file"
	@echo "  2. Check syntax highlighting"
	@echo "  3. Try code snippets (type 'fun', 'loopat', etc.)"
	@echo "  4. Verify new keywords are highlighted"

# Development workflow
dev: clean install build
	@echo "Development build completed!"
	@echo "Run 'make install-extension' to test locally."

# Show current version
version:
	@node -e "console.log('Current version:', require('./package.json').version)"

# Validate package.json
validate:
	@echo "Validating package.json..."
	@node -e "JSON.parse(require('fs').readFileSync('package.json', 'utf8')); console.log('✓ package.json is valid')"

# Quick test compile
quick-test:
	@echo "Quick compile test..."
	npm run compile
	@echo "✓ TypeScript compiles successfully"