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

## 🛠️ Development Tools & Status (v0.14.0+)

### ✅ Self-Contained Toolchain - NOW WITH 100% Z80 COVERAGE! 🎉
- **UPGRADED:** Built-in Z80 Assembler (`minzc/pkg/z80asm/`) with MZA improvements
- **BREAKTHROUGH:** Full Z80 Emulator with remogatto/z80 (19.5% → 100% coverage!)
- Interactive REPL (`cmd/repl/main.go`)
- Multi-Backend Support (Z80, 6502, WebAssembly, Game Boy, C, LLVM)
- Profile-Guided Optimization (PGO) with TAS debugger

### ✅ ACTUALLY Working Features (Tested Dec 2024)
- **Core Language**: Types (u8/u16/i8/bool), functions, control flow (if/while/for) ✅
- **Structs**: Declaration and field access ✅
- **Arrays**: Declaration and indexing ✅ (literals compile but need optimization)
- **Global variables**: With `global` keyword ✅
- **Function overloading**: Multiple signatures ✅
- **Lambdas**: Full closure support with zero-cost implementation ✅
- **Module imports**: Working with dot notation (`module.function()`) ✅
- **For loops**: Range iteration (`for i in 0..10`) ✅
- **Interfaces**: Declaration only (methods not implemented)
- **Enums**: Declaration only (values not accessible)
- **Metafunctions** (partially working):
  - `@define("template", args)` - Text substitution ✅
  - `@print` - Optimized string output ✅
  - `@if/@elif/@else` - Conditional compilation ✅
  - `@minz[[[...]]]` - Limited compile-time execution

### ❌ NOT Working Features (Need Implementation)
- **Error propagation**: `?` suffix and `??` operator NOT implemented
- **Method calls**: `obj.method()` syntax NOT working
- **Enum values**: `State::IDLE` syntax NOT working
- **Pattern matching**: Only basic support
- **Generics**: `<T>` NOT implemented
- **Array literals**: `[1,2,3]` generates 80+ lines (should be ~10)
- **Error messages**: No line numbers or source context
- **Self parameter**: Methods with self NOT working
- **Option/Result types**: Not implemented (needed for `?` operator)

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
- **ANTLR - PARKED** (regression from 75% to 5%, focusing on tree-sitter)
- **35+ peephole patterns** for Z80 optimization
- **Multi-backend support** with 8 targets
- **Tree-sitter focus** for parser improvements
- **🎉 100% Z80 instruction coverage** (upgraded from 19.5%!)

## 🔧 MinZ Toolchain Status & Next Steps

### MZE (Z80 Emulator) - ✅ Ready for Upgrade
**Current:** Basic 19.5% emulator  
**Available:** remogatto/z80 100% coverage integration  
**Next Step:** Update cmd/mze/main.go to use RemogattoZ80  
**Impact:** Full game testing, TSMC verification enabled

### MZA (Z80 Assembler) - 🚧 Enhanced, Ready for Phase 1  
**Current:** Basic assembler with recent improvements  
**Status:** Enhanced with @len, arithmetic expressions, alignment  
**Next Step:** Implement Phase 1 critical instructions (TODO_MZA.md)  
**Target:** 19.5% → 40% coverage in Week 1

### MZR (MinZ REPL) - ⏳ Basic Functionality
**Current:** Interactive MinZ compilation and execution  
**Status:** Works but limited by emulator coverage  
**Next Step:** Integrate with 100% coverage emulator  
**Benefits:** Full instruction set testing, immediate feedback

### MZV (MinZ Visualizer) - 💡 Concept Stage
**Current:** Not implemented  
**Vision:** SMC visualization, execution tracing, performance analysis  
**Next Step:** Design SMC heatmap and cycle visualization  
**Foundation:** SMC tracking already built into remogatto integration

## 📚 Documentation System

### Auto-Numbering System
All documentation (except README.md, TODO.md, STATUS.md, CLAUDE.md) uses automatic numbering:
- Format: `NNN_Title.md` (001-999)
- New docs go in `./inbox/` folder
- Run `./organize_docs.sh` to auto-number and move to `./docs/`
- Current count: 259 docs
- Next available: 260

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