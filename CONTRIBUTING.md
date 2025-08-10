# Contributing to MinZ

Thank you for your interest in contributing to the MinZ programming language! This guide will help you get started.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/minz.git`
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

## CLI Standards

All MinZ command-line tools **MUST** follow these standards for consistency:

### Library Choice
- **REQUIRED**: Use `github.com/spf13/cobra` for all CLI tools
- **RATIONALE**: Cobra provides standard Unix-style CLI conventions automatically
- **DO NOT**: Use Go's standard `flag` package directly

### Option Conventions
All options must follow standard Unix/POSIX conventions:

#### Short Options
- Single dash: `-v`, `-o`, `-h`
- Single character only
- Can be combined: `-vc` equals `-v -c`

#### Long Options
- Double dash: `--verbose`, `--output`, `--help`
- Use kebab-case: `--enable-smc`, not `--enableSMC`
- Should be self-documenting

#### Standard Options
| Short | Long | Purpose |
|-------|------|---------|
| `-h` | `--help` | Show help text |
| `-v` | `--version` or `--verbose` | Version or verbose output |
| `-o` | `--output` | Output file |
| `-t` | `--target` | Target platform |
| `-d` | `--debug` | Debug mode |

### Implementation Example
```go
var rootCmd = &cobra.Command{
    Use:   "tool [input]",
    Short: "Brief description",
    Long:  `Detailed help text...`,
}

func init() {
    rootCmd.Flags().StringVarP(&output, "output", "o", "", "output file")
    rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose")
}
```

## Code Style

- Go code follows standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions focused and small
- CLI tools must use Cobra for consistency

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