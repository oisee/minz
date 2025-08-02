# MinZ v0.9.0 "String Revolution" Release

## Installation

1. Extract the archive to your desired location
2. Add the `bin` directory to your PATH
3. Test with: `minzc --version`

## Quick Start

```bash
# Compile a MinZ program
minzc examples/fibonacci.minz -o fibonacci.a80

# With optimizations
minzc examples/fibonacci.minz -o fibonacci.a80 -O --enable-smc

# Assemble with sjasmplus (not included)
sjasmplus fibonacci.a80
```

## What's New

See RELEASE_NOTES_v0.9.0.md for full details.

### Highlights:
- Revolutionary length-prefixed strings (25-40% faster)
- Enhanced @print with { constant } compile-time evaluation
- Smart string optimization (direct vs loop)
- Improved escape sequence handling
- Self-modifying code improvements

## Directory Structure

- `bin/` - MinZ compiler binary
- `docs/` - Documentation and guides
- `examples/` - Working example programs
- `stdlib/` - Standard library modules

## VS Code Extension Note

The VS Code extension currently supports **syntax highlighting only**.
Language server integration is planned for a future release.

## Support

Report issues at: https://github.com/minz-lang/minz/issues
