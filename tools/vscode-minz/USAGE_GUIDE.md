# MinZ VSCode Extension Usage Guide

## 🚀 Quick Start

The MinZ VSCode extension is now installed and ready to use! This guide will help you get the most out of the extension.

## ✅ Installation Verification

1. **Restart VSCode** if you haven't already
2. Open any `.minz` file (try `test-modern-syntax.minz` in this directory)
3. You should see:
   - ✅ Syntax highlighting for MinZ keywords
   - ✅ Modern `fun` function syntax highlighted
   - ✅ `loop at` syntax highlighted correctly
   - ✅ New operators (`~`, `&`) highlighted

## 🎨 Syntax Highlighting Features

### Modern Keywords (July 2025 Update)
- `fun` - Modern function declaration
- `at` - For `loop at array -> item` syntax
- `do`, `times` - For `do N times` loops
- `~` - Bitwise NOT operator  
- `&` - Address-of operator

### Legacy Keywords (Still Supported)
- `fn` - Legacy function declaration
- All existing MinZ keywords and operators

## 📝 Code Snippets

Type these prefixes and press `Tab` to expand:

### Modern Syntax Snippets
- `fun` → Modern function declaration
- `loopat` → `loop at array -> item` syntax
- `dotimes` → `do N times` loop
- `shadowfun` → Shadow register function

### Legacy Snippets  
- `fn` → Legacy function declaration
- `main` → Main function template
- `if`, `while`, `for` → Control structures
- `struct`, `enum` → Data types

## 🔧 Extension Commands

Access these via Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`):

- **MinZ: Compile to Z80 Assembly** - Compile current file
- **MinZ: Compile Optimized** - Compile with optimizations
- **MinZ: Compile to IR** - Generate intermediate representation
- **MinZ: Show AST** - Display abstract syntax tree

## ⌨️ Keyboard Shortcuts

When editing `.minz` files:

- `Ctrl+Alt+B` (`Cmd+Alt+B` on Mac) - Compile to assembly
- `Ctrl+Alt+O` (`Cmd+Alt+O` on Mac) - Compile optimized
- `Ctrl+Alt+I` (`Cmd+Alt+I` on Mac) - Compile to IR
- `Ctrl+Alt+A` (`Cmd+Alt+A` on Mac) - Show AST

## ⚙️ Extension Settings

Configure via VSCode Settings (`Ctrl+,` / `Cmd+,`):

- **MinZ: Compiler Path** - Path to minzc compiler
- **MinZ: Output Directory** - Where to save compiled files
- **MinZ: Enable Optimizations** - Enable optimizations by default
- **MinZ: Enable SMC** - Enable self-modifying code optimizations
- **MinZ: Show Compiler Output** - Show compilation results

## 🧪 Testing the Extension

1. **Open** `test-modern-syntax.minz`
2. **Check** that syntax highlighting works:
   - `fun` keyword is highlighted
   - `loop at` syntax is highlighted
   - `~` and `&` operators are highlighted
   - Comments are styled correctly

3. **Try** code snippets:
   - Type `fun` and press Tab
   - Type `loopat` and press Tab  
   - Type `dotimes` and press Tab

4. **Test** commands:
   - Right-click in editor
   - Check MinZ commands in context menu
   - Try keyboard shortcuts

## 🐛 Troubleshooting

### Syntax Highlighting Not Working
- Ensure file has `.minz` extension
- Check file is detected as "MinZ" language (bottom status bar)
- Try reloading window (`Ctrl+Shift+P` → "Developer: Reload Window")

### Commands Not Available  
- Ensure you're in a `.minz` file
- Check extension is enabled in Extensions panel
- Verify minzc compiler is in PATH or configured correctly

### Snippets Not Working
- Ensure you're typing in a `.minz` file
- Try typing prefix and pressing `Tab` (not Enter)
- Check IntelliSense is enabled in VSCode settings

## 🔄 Updating the Extension

When new features are added:

```bash
cd vscode-minz
make deploy
```

This will:
1. Clean previous build
2. Install latest dependencies  
3. Bump version automatically
4. Build and package extension
5. Install updated extension locally

## 📦 Package Information

- **Name**: MinZ Language Support
- **Version**: 0.1.2 (auto-incremented)
- **Publisher**: minz-lang
- **Repository**: https://github.com/oisee/minz

## 🎯 What's New (July 2025)

✅ **Modern Syntax Support**:
- `fun` instead of `fn` for function declarations
- `loop at array -> item` iterator syntax
- `do N times` loops
- Bitwise NOT (`~`) and address-of (`&`) operators

✅ **Enhanced Snippets**:
- Modern function templates
- Iterator code completion
- SMC and shadow register templates

✅ **Build System**:
- Automated versioning
- One-command deployment
- Professional packaging

---

**🎉 Happy coding with MinZ!** Your beloved retro systems programming language now has modern IDE support!