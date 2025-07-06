# MinZ Language Support for Visual Studio Code

A comprehensive VS Code extension for the MinZ programming language - a systems programming language designed for Z80-based computers like the ZX Spectrum.

## Features

### 🎨 Syntax Highlighting
- Complete syntax highlighting for MinZ language constructs
- Support for keywords, types, functions, comments, and literals
- Special highlighting for Lua metaprogramming blocks
- Attribute and annotation highlighting

### 🔧 Compilation Support
- **Compile to Z80 Assembly**: Generate `.a80` files compatible with sjasmplus
- **Compile to IR**: View intermediate representation for debugging
- **Optimized Compilation**: Enable advanced optimizations including SMC
- **AST Viewer**: Inspect the abstract syntax tree of your code

### 📝 Code Intelligence
- Code snippets for common MinZ patterns
- Auto-closing pairs and bracket matching
- Comment toggling (line and block comments)
- Indentation rules for clean code formatting

### ⚡ Commands

| Command | Shortcut | Description |
|---------|----------|-------------|
| `MinZ: Compile to Z80 Assembly` | `Ctrl+Shift+B` / `Cmd+Shift+B` | Compile current file to Z80 assembly |
| `MinZ: Compile Optimized` | `Ctrl+Shift+O` / `Cmd+Shift+O` | Compile with full optimizations |
| `MinZ: Compile to IR` | - | Generate intermediate representation |
| `MinZ: Show AST` | - | Display abstract syntax tree |

## Installation

### From Source
1. Clone the MinZ repository
2. Navigate to the extension directory:
   ```bash
   cd vscode-minz
   ```
3. Install dependencies:
   ```bash
   npm install
   ```
4. Compile the extension:
   ```bash
   npm run compile
   ```
5. Install the extension in VS Code:
   ```bash
   code --install-extension .
   ```

### Prerequisites
- MinZ compiler (`minzc`) installed and available in PATH
- Node.js and npm for tree-sitter support

## Configuration

Configure the extension through VS Code settings:

```json
{
  "minz.compilerPath": "minzc",
  "minz.outputDirectory": "./build",
  "minz.enableOptimizations": true,
  "minz.enableSMC": false,
  "minz.showCompilerOutput": true
}
```

### Settings Reference

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `minz.compilerPath` | string | `"minzc"` | Path to the MinZ compiler |
| `minz.outputDirectory` | string | `"./build"` | Directory for compiled output |
| `minz.enableOptimizations` | boolean | `true` | Enable optimizations by default |
| `minz.enableSMC` | boolean | `false` | Enable self-modifying code optimizations |
| `minz.showCompilerOutput` | boolean | `true` | Show compiler output in terminal |

## Code Snippets

The extension includes helpful code snippets for common MinZ patterns:

- `fn` - Function declaration
- `main` - Main function
- `struct` - Struct definition
- `enum` - Enum definition
- `if`, `while`, `for` - Control flow
- `let`, `const` - Variable declarations
- `asm` - Inline assembly
- `lua` - Lua metaprogramming blocks
- `interrupt` - Interrupt handlers with shadow registers

## Language Features

### MinZ Language Support
- **Type System**: `u8`, `u16`, `i8`, `i16`, `bool`, `void`, arrays, pointers
- **Structs and Enums**: Organized data structures
- **Module System**: Import/export with visibility control
- **Inline Assembly**: Direct Z80 assembly integration
- **Shadow Registers**: Z80 alternative register set support
- **Lua Metaprogramming**: Compile-time code generation

### Example Code

```minz
// Basic function with type annotations
fn calculate(a: u8, b: u8) -> u16 {
    let sum: u16 = a + b;
    return sum * 2;
}

// Struct with methods
struct Player {
    x: i16,
    y: i16,
    health: u8,
}

// Interrupt handler with shadow registers
@interrupt
@shadow_registers
fn vblank_handler() -> void {
    frame_counter = frame_counter + 1;
}

// Lua metaprogramming
@lua[[
    function generate_sine_table()
        local table = {}
        for i = 0, 255 do
            table[i + 1] = math.floor(math.sin(i * math.pi / 128) * 127)
        end
        return table
    end
]]

const SINE_TABLE: [i8; 256] = @lua_eval(generate_sine_table());
```

## Building and Development

### Building the Extension
```bash
# Install dependencies
npm install

# Compile TypeScript
npm run compile

# Watch for changes during development
npm run watch

# Package the extension
vsce package
```

### Project Structure
```
vscode-minz/
├── package.json              # Extension manifest
├── language-configuration.json # Language configuration
├── src/
│   └── extension.ts          # Main extension code
├── syntaxes/
│   └── minz.tmLanguage.json  # Syntax highlighting rules
└── snippets/
    └── minz-snippets.json    # Code snippets
```

## Troubleshooting

### Compiler Not Found
If you get "minzc not found" errors:
1. Ensure the MinZ compiler is installed
2. Add the compiler path to your system PATH
3. Or set the full path in `minz.compilerPath` setting

### Syntax Highlighting Issues
1. Ensure the file extension is `.minz` or `.mz`
2. Manually set language mode: `Ctrl+K M` and select "MinZ"
3. Reload the window: `Ctrl+Shift+P` → "Developer: Reload Window"

### Compilation Errors
1. Check the MinZ output channel for detailed error messages
2. Ensure your MinZ syntax is correct
3. Verify the output directory exists and is writable

## Contributing

Contributions are welcome! Please see the main MinZ repository for contribution guidelines.

## License

This extension is part of the MinZ project and is released under the MIT License.

## Links

- [MinZ Language Repository](https://github.com/minz-lang/minz)
- [MinZ Documentation](https://minz-lang.org/docs)
- [Report Issues](https://github.com/minz-lang/minz/issues)