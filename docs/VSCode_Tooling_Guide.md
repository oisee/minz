# MinZ VSCode Tooling Guide

Complete guide to the MinZ VSCode extension (v0.5.0), LSP server, and DeZog source-level debugging.

---

## Installation

### 1. Build the Compiler + LSP Server

```bash
cd minzc
make build                         # Build mz compiler
go build -o mzlsp ./cmd/mzlsp/    # Build LSP server
```

Copy both `mz` and `mzlsp` to your PATH (or `~/.local/bin/`).

### 2. Install the VSCode Extension

```bash
cd tools/vscode-minz
npm install
npm run compile
npx @vscode/vsce package           # Creates .vsix file
```

Install the `.vsix` in VSCode: Extensions → "..." → "Install from VSIX..."

### 3. Configure (Optional)

In VSCode settings (`settings.json`):

```json
{
    "minz.compilerPath": "mz",          // Path to MinZ compiler
    "minz.lspPath": "",                  // Path to mzlsp (auto-detect if empty)
    "minz.outputDirectory": "./build",   // Compile output directory
    "minz.enableOptimizations": true,    // Enable -O by default
    "minz.enableSMC": false,             // Self-modifying code
    "minz.enableTrueSMC": false          // TRUE SMC (immediate anchors)
}
```

---

## Features

### Syntax Highlighting

The TextMate grammar covers the full MinZ language:

- **Keywords**: `fun`/`fn`, `let`, `const`, `global`, `struct`, `enum`, `import`, `if`/`else`, `while`, `for`, `match`, `gen`/`yield`
- **Types**: `u8`, `u16`, `u24`, `i8`, `i16`, `bool`, `void`, `Error`, fixed-point types
- **Lambdas**: `|x: u8, y: u8| => u8 { x + y }` — pipe delimiters and parameters highlighted
- **String interpolation**: `"Hello #{name}!"` — interpolated expressions get MinZ scoping
- **Inline assembly**: `asm { LD A, 42 }` — Z80 mnemonics, registers, flags, hex literals
- **Iterator chains**: `.iter()`, `.map()`, `.filter()`, `.forEach()`, `.take()`, `.skip()`, `.reduce()`, `.enumerate()`
- **Attributes**: `@[smc]`, `@[extern(0x0005)]`, `@[inline]`, `@[tailrec]`
- **Metafunctions**: `@define()`, `@print()`, `@if`/`@elif`/`@else`, `@error()`, `@emit()`
- **Metaprogramming blocks**: `@lua[[[...]]]`, `@minz[[[...]]]`
- **Operators**: `..` (range), `::` (scope), `->` (return type), `=>` (lambda arrow)

### LSP Server (mzlsp)

The LSP server provides real-time IDE features:

| Feature | Description |
|---------|-------------|
| **Diagnostics** | Red squiggles on parse/semantic errors, updated on every save |
| **Hover** | Hover over any symbol to see its type signature |
| **Go to Definition** | Ctrl+click (or F12) jumps to function/struct/enum/variable definitions |
| **Completion** | Keywords, types, metafunctions, symbols from analysis, iterator methods after `.` |

The server starts automatically when you open a `.minz` file. If `mzlsp` is not found, the extension falls back to compile-on-save diagnostics.

### Compile Commands

| Command | Shortcut | Description |
|---------|----------|-------------|
| **Compile to Z80 Assembly** | Cmd+Alt+B | Standard compile, opens .a80 side-by-side |
| **Compile to MIR** | Cmd+Alt+M | Dump MIR intermediate representation |
| **Compile to ASM** | Cmd+Alt+I | Compile to .a80, opens side-by-side |
| **Compile All** | Cmd+Alt+Shift+B | MIR + ASM in one shot |
| **Compile Optimized** | Cmd+Alt+O | Full optimizations + SMC flags |
| **Show AST** | Cmd+Alt+A | Dump AST in JSON format |
| **Debug Build** | Cmd+Alt+D | Compile with SLD source maps for DeZog |
| **Start Debugging** | F5 | Debug build + launch DeZog |

All commands are also available in the editor context menu (right-click) and the editor title bar.

### Problem Matcher

Compiler errors appear in the **Problems** panel with click-to-navigate:

```
fibonacci.minz:12:5: error: undeclared variable 'x'
```

Click the error to jump directly to the source location.

---

## Source-Level Debugging with DeZog

### Prerequisites

1. Install the [DeZog extension](https://marketplace.visualstudio.com/items?itemName=maziac.dezog) in VSCode
2. Build `mz` with `--emit-sld` support (included by default)

### How It Works

1. **Debug Build**: The compiler emits `; @src:filename:line` annotations in the Z80 assembly output
2. **SLD Generation**: The `--emit-sld` flag assembles the .a80 and generates a `.sld` file mapping Z80 addresses to MinZ source lines
3. **DeZog Integration**: DeZog reads the `.sld` file to map breakpoints and step-through to MinZ source

### Usage

**One-click debugging**:
1. Open a `.minz` file
2. Set breakpoints (click in the gutter)
3. Press **F5** (or run "MinZ: Start Debugging")
4. The extension builds with SLD, then launches DeZog

**Manual debug build**:
```bash
mz program.minz -o build/program.a80 --emit-sld
# Generates: build/program.a80, build/program.mir, build/program.sld
```

**Manual DeZog launch** (`launch.json`):
```json
{
    "type": "dezog",
    "request": "launch",
    "name": "Debug MinZ",
    "sjasmplus": [{
        "path": "${workspaceFolder}/build/program.sld"
    }],
    "topOfStack": "0xFFF0",
    "load": "${workspaceFolder}/build/program.bin",
    "startAddress": "0x8000"
}
```

### SLD File Format

The SLD (Source Level Debug) file is a pipe-delimited text format:

```
|SLD.data.version|1
||<file>|<line>|<col>|<page>|<address>|F|<label>     ← label entry
||<file>|<line>|<col>|<page>|<address>||<size>        ← source mapping
```

Example from `fibonacci.minz`:
```
|SLD.data.version|1
|||0|0|0|32768|F|FIBONACCI_MAIN
|||0|0|0|32797|F|FIBONACCI_FIBONACCI_U8
||examples/fibonacci.minz|3|0|0|32802||1
||examples/fibonacci.minz|7|0|0|32847||1
||examples/fibonacci.minz|11|0|0|32874||3
||examples/fibonacci.minz|18|0|0|32995||3
```

---

## Architecture

### Extension (`tools/vscode-minz/`)

```
tools/vscode-minz/
├── src/extension.ts        # Commands, LSP client, DeZog config provider
├── syntaxes/
│   └── minz.tmLanguage.json  # TextMate grammar (full language)
├── snippets/
│   └── minz-snippets.json    # Code snippets
├── language-configuration.json
└── package.json              # Commands, keybindings, settings, problem matcher
```

### LSP Server (`minzc/pkg/lsp/`, `minzc/cmd/mzlsp/`)

```
minzc/
├── cmd/mzlsp/main.go        # Entry point (stdio JSON-RPC)
└── pkg/lsp/
    ├── protocol.go           # LSP message types (subset of 3.17)
    └── server.go             # Handler: diagnostics, hover, goto-def, completion
```

The LSP server reuses the existing parser and semantic analyzer. On each document change:
1. Write content to temp file → `parser.ParseFile()` → AST
2. `semantic.Analyze()` → IR module + errors
3. Build symbol table from AST declarations
4. Publish diagnostics, serve hover/definition/completion from symbol table

### SLD Generation (`minzc/pkg/codegen/sld.go`)

The SLD pipeline:
1. **Semantic analysis** sets `SourceLine`/`SourceFile` on IR instructions via `trackSourcePos()` + `stampSourcePositions()`
2. **Z80 codegen** emits `; @src:file:line` comments in assembly when source info is present
3. **CLI** (`--emit-sld`): assembles the .a80, parses `@src` annotations, writes `.sld`

---

## Troubleshooting

**LSP not starting?**
- Check that `mzlsp` is in your PATH or set `minz.lspPath` in settings
- Look at the MinZ output channel (View → Output → MinZ) for errors

**No syntax highlighting?**
- Ensure the file has a `.minz` or `.mz` extension
- Check that the extension is installed and enabled

**SLD file empty?**
- The source file might only contain `asm` blocks (no MinZ-level statements to map)
- Check that `--emit-sld` flag is passed to the compiler

**DeZog not stopping at breakpoints?**
- Ensure the `.sld` file path in `launch.json` matches the actual file
- Verify the SLD has entries for the line where the breakpoint is set
