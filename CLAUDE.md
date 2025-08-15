# CLAUDE.md

This file provides guidance to Claude Code when working with the MinZ compiler repository.

## 🎓 Quick Start for AI Colleagues

- **[MinZ Crash Course for AI Colleagues](AI_COLLEAGUES_MINZ_CRASH_COURSE.md)** - Complete training
- **[STABILITY_ROADMAP.md](../STABILITY_ROADMAP.md)** - 3-phase plan to v1.0
- **[Development Roadmap 2025](../docs/129_Development_Roadmap_2025.md)** - Current priorities

## 🏗️ Architecture References

- **[INTERNAL_ARCHITECTURE.md](minzc/docs/INTERNAL_ARCHITECTURE.md)** - Complete compiler internals
- **[COMPILER_SNAPSHOT.md](COMPILER_SNAPSHOT.md)** - Current state tracking
- **[149_World_Class_Multi_Level_Optimization_Guide.md](docs/149_World_Class_Multi_Level_Optimization_Guide.md)** - Revolutionary optimization strategy

## 🎯 Custom Commands

### Core Development
- `/upd` - Update all documentation
- `/release` - Prepare new release
- `/test-all` - Comprehensive test suite
- `/benchmark` - Performance benchmarks
- `/inbox` - Process articles from inbox/ to docs/

### AI Orchestration  
- `/ai-testing-revolution` - Build testing infrastructure
- `/parallel-development` - Execute multiple tasks
- `/performance-verification` - Verify optimization claims

### Fun Commands 🎉
- `/cuteify` - Add emojis and fun
- `/celebrate` - Achievement recognition

## 🛠️ Development Tools & Status (v0.10.0)

### ✅ Self-Contained Toolchain
- Built-in Z80 Assembler (`minzc/pkg/z80asm/`)
- Embedded Z80 Emulator (`minzc/pkg/emulator/z80.go`)
- Interactive REPL (`cmd/repl/main.go`)
- Multi-Backend Support (Z80, 6502, WebAssembly, Game Boy, C, LLVM)

### ✅ Working Features (75-80% success rate)
- Core types, functions, control flow, structs, arrays
- Function overloading and interface methods
- Error propagation with `?` suffix and `??` operator
- Global variables with `global` keyword
- Metafunctions: `@print`, `@abi`, `@error`
- Self-modifying code optimization

### 🚧 Current Limitations (Actually Less Than Expected!)
- Module imports WORK with full paths (e.g., `zx.screen.set_border()`)
- String literals FULLY FUNCTIONAL (length-prefixed)
- Arrays work for declaration/access (only literals `[1,2,3]` missing)
- Pattern matching partially implemented

## 🚀 TSMC: Revolutionary Paradigm

**True Self-Modifying Code** - Programs rewrite themselves for optimization:
- **Smart Patching**: Single-byte opcode changes (7-20 T-states vs 44+)
- **Parameter Injection**: Values patched into instruction immediates
- **Behavioral Morphing**: One function, infinite behaviors
- Complete docs: `docs/145_TSMC_Complete_Philosophy.md`

## 🏆 Zero-Cost Abstractions on Z80

### ✅ Zero-Cost Lambda Iterators (v0.10.0) 🎊
```minz
numbers.iter()
    .map(|x| x * 2)
    .filter(|x| x > 5)
    .forEach(|x| print_u8(x));
```
**Revolutionary**: Lambda-to-function transform with DJNZ optimization!

### ✅ Zero-Cost Interfaces
```minz
circle.draw()  // Direct CALL Circle_draw - NO vtables!
```

### ✅ Zero-Overhead Lambdas
```minz
let add = |x: u8, y: u8| => u8 { x + y };
add(5, 3)  // Direct CALL - 100% performance
```

## 📋 Development Commands

### Build & Test
```bash
# Build compiler
cd minzc && make build

# Test all examples
./compile_all_examples.sh

# Compile with optimizations
./minzc program.minz -O --enable-smc
```

### Multi-Backend Compilation
```bash
mz program.minz -b z80 -o program.a80    # Z80 (default)
mz program.minz -b c -o program.c        # C code
mz program.minz -b wasm -o program.wat   # WebAssembly
```

## 📁 Project Structure

```
minz-ts/
├── minzc/              # Go compiler
│   ├── cmd/           # CLI tools (minzc, repl, backend-info)
│   ├── pkg/           # Compiler packages
│   └── tests/         # Test files
├── grammar.js         # Tree-sitter grammar
├── examples/          # MinZ programs
├── docs/             # Documentation
└── releases/         # Release packages
```

## 🎯 Design Philosophy

### Ruby-Style Developer Happiness
```minz
// Flexible function declaration
fn add(a: u8, b: u8) -> u8 { ... }    // or 'fun'
fun subtract(a: u8, b: u8) -> u8 { ... }

// Clear global variables
global counter: u8 = 0;

// Function overloading
print(42);     // No more print_u8!
print("Hi");   // Just print!
```

### Target Architecture
One backend, multiple targets:
```bash
mz program.minz -b z80 --target=spectrum  # ZX Spectrum
mz program.minz -b z80 --target=cpm       # CP/M
```

## 📊 Current Metrics (v0.14.0-dev)
- **170 examples** in test suite (88 actively tested)
- **63% compilation success** with tree-sitter parser
- **5% compilation success** with ANTLR (regression - investigating)
- **35+ peephole patterns** for Z80 optimization
- **Multi-backend support** with 8 targets
- **Zero external dependencies** with ANTLR parser

## 📚 Documentation System

### Auto-Numbering System
All documentation (except README.md, TODO.md, STATUS.md, CLAUDE.md) uses automatic numbering:
- Format: `NNN_Title.md` (001-999)
- New docs go in `./inbox/` folder
- Run `./organize_docs.sh` to auto-number and move to `./docs/`
- Current count: 164 docs
- Next available: 165

### Workflow
```bash
# Write new doc
echo "# My Feature" > inbox/My_Feature_Guide.md

# Auto-number and organize
./organize_docs.sh
# Creates: docs/165_My_Feature_Guide.md

# Batch process multiple docs
cp *.md inbox/
./organize_docs.sh
```

### Finding Documents
```bash
ls docs/ | sort -n        # List by number
grep -l "TSMC" docs/*.md  # Find by topic
ls docs/15[0-9]_*.md      # Range 150-159
```

See [Documentation Guide](DOCUMENTATION_GUIDE.md) for complete details.

## 🔧 Documentation Style: "Pragmatic Humble Solid"

- ✅ **Transparent**: "Core features work" / "Experimental"  
- 🚧 **Status indicators**: Working/In Progress/Missing
- 📊 **Specific**: "60% of examples compile"
- ⚠️ **Honest warnings**: "Not production ready"

Celebrate real achievements without hype. Ground excitement in facts.

---

*MinZ: Modern programming abstractions with zero-cost performance on vintage Z80 hardware.*