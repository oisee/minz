# CLAUDE.md

This file provides guidance to Claude Code when working with the MinZ compiler repository.

---

## 🚨 CURRENT PRIORITY: Native Parser Implementation

**Status:** IN PROGRESS (Q1 2026)

**Goal:** Replace tree-sitter with pure Go recursive descent parser to eliminate OOM issues.

**Key Documents:**
- 📋 **[NATIVE_PARSER_PLAN.md](docs/NATIVE_PARSER_PLAN.md)** - Complete implementation plan
- 🧪 **[Parser Test Corpus](minzc/tests/parser_corpus/)** - 220 regression test files

**Quick Commands:**
```bash
cd minzc
./ast-gen tests/parser_corpus ../stdlib ../examples ../tests  # Generate corpus
./ast-compare tests/parser_corpus                              # Validate
./ast-compare -parser './new-parser' tests/parser_corpus       # Test new parser
```

**Target Metrics:**
- 95%+ regression pass rate on 220 test files
- <50MB memory on 10MB input files
- Zero external dependencies

---

## 🎓 Quick Start for AI Colleagues

- **[MinZ Crash Course for AI Colleagues](AI_COLLEAGUES_MINZ_CRASH_COURSE.md)** - Complete training
- **[STABILITY_ROADMAP.md](../STABILITY_ROADMAP.md)** - 3-phase plan to v1.0
- **[Development Roadmap 2025](../docs/129_Development_Roadmap_2025.md)** - Current priorities

## 🏗️ Architecture References

- **[NATIVE_PARSER_PLAN.md](docs/NATIVE_PARSER_PLAN.md)** - 🚧 Parser replacement plan (ACTIVE)
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

## 🛠️ Development Tools & Status (v0.18.0)

### Self-Contained Toolchain
- **MZC** - MinZ Compiler (Go, ~90K LOC)
- **MZA** - Z80 Assembler (built-in)
- **MZE** - Z80 Emulator (100% coverage via remogatto/z80)
- **MZR** - Interactive REPL
- **MZRUN** - Remote runner (DZRP protocol)

---

## 📊 Feature Status Legend

| Tag | Meaning |
|-----|---------|
| ✅ **DONE** | Working in production |
| 🚧 **WIP** | In active development |
| 📋 **TOBE** | Planned for implementation |
| ⏸️ **PARKED** | Deferred, may return later |
| ❌ **REJECTED** | Will NOT be implemented |

---

## ✅ DONE (Working Features)

### Core Language
- Types: `u8`, `u16`, `i8`, `i16`, `bool`, `void`
- Functions: `fun`/`fn` declaration, multiple returns
- Control flow: `if`/`else`, `while`, `for i in 0..n`
- Structs: declaration and field access
- Arrays: declaration, indexing (literals need optimization)
- Global variables: `global` keyword
- Function overloading: multiple signatures

### Advanced Features
- **Ruby interpolation**: `"Hello #{name}!"` ✅
- **UFCS**: `obj.method()` via zero-cost interfaces ✅
- **Lambdas**: Full closure support, zero-cost ✅
- **TRUE SMC**: Self-modifying code optimization ✅
- **CTIE**: Compile-time function execution ✅
- **@extern FFI**: Call external ROM/BIOS functions ✅
- **RST optimization**: Auto-convert to RST instructions ✅
- **Operator overloading**: Custom operators for types ✅

### Metafunctions
- `@define("template", args)` - Text substitution ✅
- `@print` - Optimized string output ✅
- `@if/@elif/@else` - Conditional compilation ✅
- `@error` - Error propagation with CY flag ✅

### Tooling
- Error messages with file:line:col format ✅
- Multi-backend: Z80, 6502, Crystal, WASM, C ✅
- 100% Z80 instruction coverage in emulator ✅

---

## 🚧 WIP (In Development)

- **Pattern matching**: Syntax parses, codegen partial
- **@minz[[[...]]]**: Limited compile-time execution
- **MIR interpreter**: Arrays/structs now working

---

## 📋 TOBE (Planned)

### Week 2-4
- **Hand-written parser** - Replace tree-sitter (OOM fix)
- **Nested functions** - Functions inside functions

### Week 5-8
- **LSP server** - IDE support (autocomplete, errors)
- **DAP debugger** - Step-through debugging
- **WASM playground** - Online demo

### Future
- **Enum values**: `State::IDLE` syntax
- **Array literal optimization**: `[1,2,3]` → 10 lines not 80

---

## ⏸️ PARKED (Deferred)

- **Generics `<T>`** - Use function overloading instead
- **Local/nested functions** - Workaround: use lambdas
- **Option/Result types** - Use `@error` pattern instead
- **`?` operator** - Use explicit error checking

---

## ❌ REJECTED (Won't Implement)

- **C++ style templates** - Too complex for Z80 target
- **Multiple inheritance** - Use interfaces instead
- **Garbage collection** - Manual memory for retro targets
- **Exceptions** - Use `@error` with CY flag instead
- **Runtime reflection** - No runtime overhead allowed
- **Dynamic dispatch vtables** - Zero-cost interfaces only

## 🎯 Metafunction Design Decisions

**CRITICAL:** These are settled design decisions - do not confuse them!

- **@minz[[[...]]]** - Immediate compile-time execution
  - Takes NO ARGUMENTS (not a template!)
  - Uses @emit() to generate code line by line
  - Example: `@minz[[[ @emit("fun foo() -> void {}") ]]]`

- **@define("template", args...)** - Preprocessor macro substitution
  - Processed BEFORE parsing (pure text replacement)
  - Uses {0}, {1} placeholders for arguments
  - Example: `@define("fun {0}() -> {1}", "getName", "str")`
  - Status: ✅ FULLY IMPLEMENTED AND WORKING!

- **@lua[[[...]]]** - Lua compile-time execution
  - Full Lua scripting for complex metaprogramming
  - Has emit() function for code generation

See `docs/Metafunction_Design_Decisions.md` for complete details.

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

## 📚 Standard Library (v0.15.0+)

MinZ includes a comprehensive stdlib optimized for Z80/retro systems:

| Module | Description |
|--------|-------------|
| `math/fast` | Sin/cos/sqrt lookup tables (256 entries each) |
| `math/random` | LFSR PRNG, noise functions, probability helpers |
| `graphics/screen` | Pixel/line/circle/rectangle (ZX Spectrum optimized) |
| `input/keyboard` | Keyboard matrix reading, debouncing, game helpers |
| `text/string` | strlen, strcmp, strcpy, strcat, trim, etc. |
| `text/format` | Number to string (decimal, hex, binary) |
| `sound/beep` | Beeper SFX (click, buzz, jump, explosion) |
| `time/delay` | Frame timing, delays, animation helpers |
| `mem/copy` | Fast memcpy/memset/memcmp using LDIR |
| `cpm/bdos` | CP/M BDOS system calls |

### Usage Example
```minz
import stdlib.graphics.screen;
import stdlib.input.keyboard;
import stdlib.time.delay;

fun main() -> void {
    clear_screen();
    draw_circle(128, 96, 50);

    loop {
        wait_frame();
        let dx = get_key_dx();
        // Move sprite based on input...
    }
}
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
minz/
├── minzc/              # Go compiler
│   ├── cmd/           # CLI tools (minzc, repl, backend-info)
│   ├── pkg/           # Compiler packages
│   └── tests/         # Test files
├── stdlib/            # Standard library
│   ├── math/         # fast.minz, random.minz
│   ├── graphics/     # screen.minz
│   ├── input/        # keyboard.minz
│   ├── text/         # string.minz, format.minz
│   ├── sound/        # beep.minz
│   ├── time/         # delay.minz
│   ├── mem/          # copy.minz
│   └── cpm/          # bdos.minz
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

## 📊 Current Metrics (v0.18.0)

| Metric | Value |
|--------|-------|
| Core examples | 72/72 (100%) |
| All examples | ~81% |
| Stdlib modules | 10 |
| Z80 emulator coverage | 100% |
| Peephole patterns | 35+ |
| Backends | 8 |

---

## 🔧 Toolchain Component Status

| Tool | Status | Description |
|------|--------|-------------|
| **MZC** | ✅ DONE | MinZ Compiler (Go) |
| **MZA** | ✅ DONE | Z80 Assembler |
| **MZE** | ✅ DONE | Z80 Emulator (100% coverage) |
| **MZR** | 🚧 WIP | Interactive REPL |
| **MZRUN** | ✅ DONE | Remote runner (DZRP) |
| **MZV** | 📋 TOBE | SMC Visualizer |
| **LSP** | 📋 TOBE | Language Server Protocol |
| **DAP** | 📋 TOBE | Debug Adapter Protocol |

### MZRUN Usage
```bash
# Start ZXSpeculator with DZRP on port 11000, then:
./mzrun program.minz --reset -v
```

## Documentation System

### Date-Prefixed Naming Convention (v0.18+)
All documentation (except README.md, TODO.md, STATUS.md, CLAUDE.md) uses chronological numbering:
- Format: `YYYY-MM-DD-NNN-Topic.md`
- Example: `2026-01-08-261-Progress_Report_v0.18.md`
- New docs go in `./inbox/` folder
- Run `./organize_docs.sh` to auto-number with today's date and move to `./docs/`
- Current count: 300+ docs

### Benefits
- Chronological sorting by default
- Easy to find docs by date
- Preserves sequential numbering for reference

### Workflow
```bash
# Write new doc
echo "# My Feature" > inbox/My_Feature_Guide.md

# Auto-number and organize (adds today's date + next number)
./organize_docs.sh
# Creates: docs/2026-01-09-314-My_Feature_Guide.md

# Batch process multiple docs
cp *.md inbox/
./organize_docs.sh
```

### Finding Documents
```bash
ls docs/ | sort             # List chronologically
grep -l "TSMC" docs/*.md    # Find by topic
ls docs/2026-01-*           # All January 2026 docs
ls docs/*-26[0-9]-*         # Docs 260-269
```

See [Documentation Guide](DOCUMENTATION_GUIDE.md) for complete details.

## 🤖 AI Colleague Consultation

**Purpose**: Leverage AI tools (GPT-4, o4-mini, Claude) as virtual colleagues for architectural decisions, debugging, and design reviews.

### When to Consult
- Major architectural choices (parser strategy, optimization approaches)
- Stuck issues or nonobvious bugs  
- Design trade-offs and brainstorming
- Sanity-checking assumptions before large refactors

### How to Consult Effectively
1. **Provide full context**: Problem statement, what you've tried, relevant code snippets, constraints
2. **End with specific ask**: "Pros/cons of ANTLR vs hand-written parser" vs "Help with parser"
3. **Cross-check multiple models**: Run same question through 2+ AI colleagues for consensus
4. **Include concrete constraints**: Performance targets, maintenance concerns, team skills

### Evaluating AI Advice
- Treat as input for team discussion, not final authority
- Cross-check factual claims against official docs
- Plan small proof-of-concept to validate suggestions  
- When multiple models agree, confidence increases (but still validate)

### Documentation
Keep an **AI Consultation Log** in relevant docs:
- Date, participants (which AI models), original prompt
- Key advice given and follow-up questions
- Outcome and rationale for following/rejecting advice
- Link to related issues/PRs/commits

**Example Success**: August 2024 - Consulted GPT-4 and o4-mini on ANTLR vs hand-written parser for Z80 assembler. Both recommended keeping hand-written parser and fixing encoder issues instead. Decision saved significant development time and led to identifying the real problem.

### Best Practices
- Never merge critical changes solely on AI advice
- Always do code reviews and team discussion for broad-impact decisions
- Create prompt templates for consistent, high-quality consultations
- Review consultation logs in retrospectives to improve question quality

## 🔧 Documentation Style: "Pragmatic Humble Solid"

- ✅ **Transparent**: "Core features work" / "Experimental"  
- 🚧 **Status indicators**: Working/In Progress/Missing
- 📊 **Specific**: "60% of examples compile"
- ⚠️ **Honest warnings**: "Not production ready"

Celebrate real achievements without hype. Ground excitement in facts.

---

*MinZ: Modern programming abstractions with zero-cost performance on vintage Z80 hardware.*