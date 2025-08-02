# MinZ VS Code Extension

**Status**: Syntax Highlighting Only

This VS Code extension provides syntax highlighting support for MinZ programming language files (`.minz`).

## Features

- ✅ **Syntax Highlighting** - Full syntax highlighting for MinZ language constructs
- ✅ **File Association** - Automatic recognition of `.minz` files
- ✅ **Theme Support** - Works with all VS Code color themes

## Limitations

This extension currently provides **syntax highlighting only**. The following features are **NOT** yet implemented:

- ❌ Language Server Protocol (LSP)
- ❌ IntelliSense/Autocompletion  
- ❌ Go to Definition
- ❌ Hover Information
- ❌ Error Checking
- ❌ Formatting
- ❌ Refactoring

## Installation

1. Copy the `vscode-extension` folder to your VS Code extensions directory:
   - Windows: `%USERPROFILE%\.vscode\extensions\`
   - macOS/Linux: `~/.vscode/extensions/`
2. Restart VS Code
3. Open any `.minz` file to see syntax highlighting

## Usage

Simply open any `.minz` file and you'll see syntax highlighting automatically applied.

### Supported Syntax Elements

- Keywords: `fun`, `let`, `mut`, `if`, `else`, `while`, `for`, `return`, etc.
- Types: `u8`, `u16`, `i8`, `i16`, `bool`, `void`
- Operators: `+`, `-`, `*`, `/`, `==`, `!=`, `&&`, `||`, etc.
- Comments: `//` single-line and `/* */` multi-line
- Strings: `"string literals"` with escape sequences
- Numbers: Decimal, hexadecimal (`0x`), binary (`0b`)
- Metafunctions: `@print`, `@abi`, `@lua`, etc.
- Attributes: `#[no_mangle]`, `#[export]`, etc.

## Future Plans

Full language server support with IntelliSense, error checking, and more is planned for a future release. This will require:

1. Implementing a Language Server Protocol (LSP) server in the MinZ compiler
2. Updating this extension to communicate with the LSP server
3. Adding configuration options for compiler paths and settings

## Contributing

The syntax highlighting grammar is defined in:
- `syntaxes/minz.tmLanguage.json`

To improve syntax highlighting, edit this file and submit a pull request.

## License

Same as the MinZ project - see LICENSE file in the root directory.