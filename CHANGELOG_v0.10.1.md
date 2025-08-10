# Changelog for MinZ v0.10.1

## Release Date: August 10, 2025

## 🎯 Release Theme: **Professional Toolchain & Architecture**

This release focuses on making MinZ a truly professional compiler suite with standardized CLI interfaces, proper architecture documentation, and significant language improvements.

## ✨ Major Features

### 1. **CLI Standardization with Cobra** 🛠️
- All tools (`mz`, `mza`, `mze`) now use consistent Unix-style options
- Proper short (`-v`) and long (`--verbose`) option pairing
- Professional help text with examples and descriptions
- **Breaking Change**: Some options have changed (e.g., `-undoc` → `-u, --undocumented`)

### 2. **Architecture Decision Records (ADRs)** 🏛️
- New `/adr` directory for documenting technical decisions
- 5 initial ADRs documenting key architectural choices
- Professional engineering practice for long-term maintainability

### 3. **Enhanced Language Features** 🚀
- **Enum Support**: Full enum types with proper code generation
- **Logical Operators**: `&&` and `||` operators with short-circuit evaluation
- **Array Literals**: `[1, 2, 3]` syntax for array initialization
- **String Alias Fix**: `str` renamed to `String`, `*str` to `*u8` for C-style strings

### 4. **Platform Independence Improvements** 🌍
- Enhanced character literal support in assembly: `LD A, 'H'` and `LD A, '\n'`
- Better documentation of platform targeting
- Comprehensive platform independence guide

### 5. **Documentation Revolution** 📚
- Automatic documentation numbering system
- Comprehensive architecture audit (4-part deep dive)
- World-class optimization guide
- SHA256 cryptographic implementation example
- Complete hello world compilation guide

## 🔧 Tool Improvements

### mz (Compiler)
- Already used Cobra, now serves as reference implementation
- Enhanced help text with platform examples

### mza (Assembler)
- **Migrated to Cobra** from standard flag package
- Standardized options:
  - `-o, --output` for output file
  - `-l, --listing` for listing file
  - `-s, --symbols` for symbol table
  - `-u, --undocumented` for undocumented instructions
  - `-v, --verbose` for verbose output
  - `-c, --case-sensitive` for case-sensitive labels

### mze (Emulator)
- **Migrated to Cobra** from standard flag package
- Fixed duplicate option issue (`-t` and `-target` now unified)
- Standardized options:
  - `-t, --target` for platform selection
  - `-a, --address` for load address
  - `-v, --verbose` for verbose output
  - `-c, --cycles` for cycle counting

### mzr (REPL)
- Enhanced help system with categorized commands
- Improved welcome banner with quick start examples
- Better documentation of TAS debugging features

## 🐛 Bug Fixes
- Fixed import cycle in optimization system
- Resolved `str` type confusion in semantic analyzer
- Fixed duplicate CLI options in mze
- Corrected character literal parsing in assembler

## 📖 Documentation
- Added CONTRIBUTING.md with CLI standards
- Created ADR system with 5 initial records
- Enhanced README with toolchain overview
- Platform independence achievement article
- Multiple architecture analysis documents

## 💔 Breaking Changes
1. **CLI Options**: Some command-line options have changed format:
   - `mza -undoc` → `mza -u` or `mza --undocumented`
   - `mza -case` → `mza -c` or `mza --case-sensitive`
   - `mze` no longer has separate `-target` flag (use `-t` or `--target`)

2. **Type Names**: 
   - `str` type renamed to `String` (MinZ native strings)
   - `*str` should now be `*u8` (C-style strings)

## 📊 Statistics
- **5 ADRs** documenting key decisions
- **3 tools** migrated to Cobra
- **20+ commits** since v0.10.0
- **70% test success rate** maintained

## 🔮 Next Steps
- Continue stability improvements toward v1.0
- Implement module system
- Add more backend targets
- Improve standard library

## 📥 Installation
```bash
# Clone and install
git clone https://github.com/oisee/minz.git
cd minz/minzc
make install-user  # Installs to ~/bin
```

## 🙏 Acknowledgments
Thanks to all contributors who helped make MinZ more professional and maintainable!

---

*MinZ: Professional tools for retro systems* 🚀