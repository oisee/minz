# Contributing to MinZ

Thank you for your interest in contributing to the MinZ programming language! This guide will help you get started.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/minz-ts.git`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes (see Testing section)
6. Commit with meaningful messages
7. Push to your fork: `git push origin feature/your-feature-name`
8. Open a Pull Request

## Development Setup

### Prerequisites
- Node.js (for tree-sitter)
- Go 1.21+ (for compiler)
- tree-sitter CLI: `npm install -g tree-sitter-cli`

### Building
```bash
# Generate tree-sitter parser
npm install
tree-sitter generate

# Build the compiler
cd minzc && make build

# Run tests
cd minzc && make test
```

## Code Style

- Go code follows standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions focused and small

## Testing

- Add tests for new features
- Ensure all existing tests pass: `cd minzc && make test`
- Test with example programs: `cd minzc && make run`

## Documentation

- Update README.md if adding new features
- Add examples demonstrating new functionality
- Update relevant documentation in docs/

## Pull Request Guidelines

1. **One feature per PR** - Keep PRs focused
2. **Clear description** - Explain what and why
3. **Tests required** - Include tests for new code
4. **Documentation** - Update docs as needed
5. **Clean history** - Squash commits if needed

## Areas for Contribution

- **Compiler optimizations** - Improve code generation
- **Language features** - Implement planned features
- **Documentation** - Improve guides and examples
- **Testing** - Add more test coverage
- **Tools** - IDE support, debuggers, etc.

## Questions?

Feel free to open an issue for questions or discussions about potential contributions.

Thank you for helping make MinZ better!