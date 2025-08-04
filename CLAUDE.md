# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 🎓 AI Colleague Resources

- **[MinZ Crash Course for AI Colleagues](AI_COLLEAGUES_MINZ_CRASH_COURSE.md)** - Complete training for MinZ development
  - Quick Start with real commands and project structure
  - Language essentials based on actual grammar
  - Current features (error propagation, basic optimization, core language)
  - Development workflow and debugging
  - Design philosophy and goals
  - Development checklist

## Documentation Style Guide

### The "Pragmatic Humble Solid" Tone

Our documentation has found the perfect balance - **pragmatic, humble but solid and proud**. This tone builds trust through honesty while celebrating real achievements.

#### Core Principles

1. **Be Transparent About Status**
   - ✅ "Core language features work" - Clear, factual
   - 🚧 "Experimental work ongoing" - Honest about limitations
   - ⚠️ "Not yet suitable for production" - Direct warnings
   
2. **Use Status Indicators Consistently**
   - ✅ **Working/Stable** - Features ready for use
   - 🚧 **In Progress/Experimental** - Partially working, evolving
   - ❌ **Missing/Planned** - Not yet implemented
   - 🔬 **Research** - Exploring possibilities

3. **Celebrate Achievements Without Hype**
   - YES: "Error propagation system recently implemented"
   - YES: "Generates working assembly for ZX Spectrum" 
   - NO: "Revolutionary breakthrough in compiler technology"
   - NO: "World's first zero-cost abstractions on 8-bit"

4. **Be Specific About Capabilities**
   - "60% of examples compile" - Quantifiable progress
   - "Basic loops work well" - Clear about what works
   - "Recently implemented" - Temporal context
   - "Research goal" - Clear about aspirations

5. **Provide Context and Alternatives**
   ```minz
   // Note: Iterator chains are experimental; basic for loops work well
   ```

6. **Use Professional But Accessible Language**
   - Short, clear sentences
   - Technical terms explained in context
   - Code examples for clarity
   - Helpful comments in examples

### Documentation Patterns

#### Feature Status Pattern
```markdown
### ✅ Error Propagation (Recently Implemented)
[Description of what works]

### 🚧 Iterator Chains (Research)  
[Description of experimental state]
Note: [Alternative that works]
```

#### Warning Pattern
```markdown
⚠️ **Important**: [Clear warning about limitations]
```

#### Progress Tracking Pattern
```markdown
| Feature | Status | Notes |
|---------|--------|-------|
| Basic functions | ✅ Working | Good reliability |
| Metaprogramming | 🚧 Research | Experimental |
```

### When to Show Pride

We CAN be proud and excited when we genuinely achieve something difficult:
- When zero-cost abstractions actually work - celebrate!
- When we solve "impossible" problems on 8-bit hardware - be proud!
- When benchmarks prove our optimization claims - share the data!

But always ground excitement in facts and measurements.

### Language to Avoid

- Superlatives without evidence ("revolutionary", "breakthrough")
- Absolute claims ("world's first", "never before")
- Marketing speak ("game-changing", "paradigm shift")
- Vague promises ("coming soon", "will be amazing")

### Language to Embrace

- Specific achievements ("recently implemented", "now working")
- Honest assessments ("experimental", "research phase")
- Clear roadmaps ("3-phase plan", "14 weeks to v1.0")
- Measured optimism ("promising initial results")

## 📋 Stability Roadmap

**IMPORTANT**: See [STABILITY_ROADMAP.md](../STABILITY_ROADMAP.md) for our detailed 3-phase plan to reach v1.0 production readiness by November 2025. This roadmap defines:
- What's stable vs experimental
- Priority order for stabilization
- Success metrics for each phase
- Timeline to production readiness

## 🚀 AI-Driven Development Practices

@CLAUDE_BEST_PRACTICES.md

## 🎯 Custom Commands Available

### Core Development Commands
- `/upd` - Update all documentation (README, CLAUDE.md, design docs) with latest changes
- `/release` - Prepare a new release (build binaries, update version, create notes, tag)
- `/test-all` - Run comprehensive test suite and generate report
- `/benchmark` - Run performance benchmarks and compare with baseline

### AI Orchestration Commands  
- `/ai-testing-revolution` - Build complete testing infrastructure using parallel agents
- `/parallel-development` - Execute multiple tasks simultaneously with AI orchestration
- `/performance-verification` - Verify optimization claims with comprehensive benchmarking

### Fun Commands 🎉
- `/cuteify` - Add appropriate emojis and fun elements to output (use sparingly!)
- `/celebrate` - Generate celebration message for achievements
- `/motivate` - Get a motivational message when stuck
- `/rubber-duck` - Explain the problem step by step to find the solution

### Quick Usage Examples:
```bash
# Build comprehensive testing
/ai-testing-revolution "Create E2E tests for new optimizer"

# Parallel task execution
/parallel-development "Fix bug X, implement feature Y, add tests for Z"

# Verify optimizations
/performance-verification "Prove new optimization delivers 25% improvement"

# Update all documentation with latest changes
/upd "Added new @minz[[[]]] syntax for compile-time code"

# Prepare a new release
/release v0.9.5 "Iterator improvements and @if completion"

# Add some fun to the output
/cuteify "Successfully compiled all examples!"

# Celebrate a milestone
/celebrate "Error propagation system complete!"
```

### Command: `/upd` - Documentation Sync

Updates all project documentation to maintain consistency:
1. **README.md** - Main project documentation
2. **CLAUDE.md** - AI assistant guidance
3. **STABILITY_ROADMAP.md** - Development roadmap
4. **Design docs** - Technical specifications
5. **Examples** - Code examples and tutorials

The command ensures all documentation reflects the latest:
- Feature implementations
- API changes
- Design decisions
- Status updates
- Known issues

### Command: `/release` - Automated Release Process

Prepares a complete release package:
1. **Version bump** - Updates version in all files
2. **Build binaries** - Cross-compile for all platforms
3. **Generate release notes** - From recent commits and completed tasks
4. **Run tests** - Ensure everything works
5. **Create GitHub release** - Tag, upload binaries, publish notes
6. **Update documentation** - Version numbers and compatibility

Example: `/release v0.9.5 "Iterator optimizations and @if completion"`

### Command: `/cuteify` - Add Personality ✨

Makes output more fun and engaging (use sparingly!):
- Adds appropriate emojis 🎉
- Uses encouraging language 💪
- Celebrates achievements 🏆
- Makes errors less scary 🤗

Example output transformation:
```
Before: "Successfully compiled 148 examples"
After:  "🎉 Woohoo! Successfully compiled all 148 examples! 🚀"
```

### Command: `/celebrate` - Achievement Recognition 🎊

Generates appropriate celebration for milestones:
- Major feature completion
- Performance improvements
- Bug fixes
- Test suite passing

Example: `/celebrate "Zero-cost abstractions actually working!"`
Output: "🎊 AMAZING ACHIEVEMENT! 🎊 Zero-cost abstractions on 8-bit hardware - what was thought impossible is now reality! This is genuinely revolutionary! 🚀✨"

## 🛠️ Development Tools & Capabilities (v0.9.0)

### ✅ What We've Already Built & Solved

**🎯 SELF-CONTAINED TOOLCHAIN (No External Dependencies!):**
- ✅ **MinZ REPL**: Interactive Z80 development environment (`docs/124_MinZ_REPL_Implementation.md`)
- ✅ **Built-in Z80 Assembler**: `minzc/pkg/z80asm/` - NO sjasmplus needed!
- ✅ **Embedded Z80 Emulator**: `minzc/pkg/emulator/z80.go` - cycle-accurate execution
- ✅ **Complete Pipeline**: MinZ → Assembly → Machine Code → Execution (all built-in)

**Testing Infrastructure:**
- ✅ **E2E Testing Pipeline**: `compile_all_examples.sh` - tests 148 examples automatically
- ✅ **Performance Benchmarking**: Automated measurement of optimization improvements
- ✅ **Statistics Generation**: Success rates, compilation analysis, feature coverage
- ✅ **Release Pipeline**: Complete automation from code → binaries → GitHub release

**Language Issues We've Fixed:**
- ✅ **Escape Sequences**: `\n`, `\t` in string literals work properly
- ✅ **SMC Optimization**: Self-modifying code detection and generation
- ✅ **String Architecture**: Length-prefixed strings with smart optimization  
- ✅ **Enhanced @print**: Compile-time constant evaluation with `{ expr }` syntax
- ✅ **Global Variables**: Basic types (u8, u16, bool) work perfectly
- ✅ **Global Struct Variables**: Complex structs can be declared globally
- ✅ **Error Propagation**: Functions with `?` suffix, @error, ?? operator
- 🚧 **@if Conditional**: Partial implementation, compile-time evaluation works
- 🚧 **@minz Redesign**: New @minz[[[]]] syntax designed, not yet implemented
- ✅ **Error Handling**: `@error` system with error propagation (`docs/127_Error_Propagation_System.md`)

**MinZ Language Features That Work (60% success rate):**
- ✅ **Core Types**: u8, u16, i8, i16, bool, arrays, pointers
- ✅ **Functions**: Parameters, returns, recursion, basic optimization
- ✅ **Control Flow**: if/else, while, for loops with ranges
- ✅ **Structs**: Definition, instantiation, field access
- ✅ **Arrays**: Fixed-size arrays, indexing, initialization
- ✅ **Metafunctions**: @print with interpolation, @abi for assembly, @error for error handling
- ✅ **Optimization**: -O flag, --enable-smc, register allocation

**Known Working Syntax Patterns:**
```minz
// These patterns are confirmed working:
global u8 simple_var = 42;
global ComplexStruct complex_var;  // ✅ This works!

fun function_name(param: u8) -> u16 { ... }  // ✅ "fun" not "fn"
fun error_func?(param: u8) -> u8 ? ErrorType { ... }  // ✅ Error-throwing functions
let local: Type = value;
struct_var.field = value;
array_var[index] = value;
@print("Text with {} interpolation", value);
let result = risky_operation?() ?? @error;  // ✅ Error propagation
@error(ErrorType.Variant);  // ✅ Explicit error throwing

// NEW: @ prefix = compile-time execution (universal rule)
@if(DEBUG, 10, 20);                    // 🚧 Partially working
@lua[[[print("Compile-time Lua")]]];   // ✅ Works
@minz[[[...]]]("param");                // 🚧 Designed, coming soon
```

### 🚧 Current Limitations (v0.9.0)

**Language Features Missing (40% failures):**
- ❌ **Interfaces**: `self` parameter resolution broken
- ❌ **Module Imports**: Import system not implemented
- ❌ **Advanced Metafunctions**: @hex, @bin, @debug, @format
- ❌ **Standard Library**: print_u8, print_u16, mem.*, str.* functions
- ❌ **Pattern Matching**: Grammar ready, semantics missing
- ❌ **Generics**: Type parameters not supported

**Compiler Issues to Fix:**
- ❌ **Global variable access in functions**: "undefined identifier" errors
- ❌ **Cast type inference**: "cannot determine type of expression being cast"
- ❌ **If expressions**: `if (cond) { val1 } else { val2 }` syntax issues

### 🎯 Essential Tools - All Self-Contained!

**🚀 NO EXTERNAL DEPENDENCIES NEEDED!** MinZ includes everything:

```bash
# MinZ Compiler (generates Z80 assembly)
cd minzc && ./minzc ../examples/fibonacci.minz -o fibonacci.a80

# Built-in Z80 Assembler (assembly → machine code)
# Located at: minzc/pkg/z80asm/ - full instruction set!

# Z80 Emulator (execute and debug)  
# Located at: minzc/pkg/emulator/z80.go - cycle accurate!

# Interactive REPL (compile + assemble + execute)
cd minzc && go run cmd/repl/main.go

# Complete Testing Pipeline
./minzc/compile_all_examples.sh  # Tests 148 examples
```

**Self-Contained Pipeline:**
```
MinZ Source → MinZ Compiler → Z80 Assembly → Built-in Assembler → Machine Code → Z80 Emulator
```

### 🎯 Testing Commands

```bash
# Test single example
./minzc ../examples/filename.minz -o output.a80 -O --enable-smc

# Test all examples (we have this!)
./compile_all_examples.sh

# Check success rate
grep "Successfully compiled" *.log | wc -l

# Analyze failures  
grep "Error:" *.log | sort | uniq -c | sort -nr
```

### 🧠 ZVDB-MinZ Project Status

**Current State**: Building complete 256-bit vector database in MinZ as showcase project

**What Works:**
- ✅ **Basic ZVDB**: 64-bit vectors, popcount, similarity search (zvdb_working.minz compiles)
- ✅ **Global structs**: Complex database structures can be declared
- ✅ **Core algorithms**: Hamming distance, similarity scoring, vector operations

**Current Issues (fixing these improves MinZ):**
- 🔧 **Global variable access**: Functions can't access global struct fields
- 🔧 **Cast inference**: Type system needs help with `value as type` expressions  
- 🔧 **If expressions**: Ternary-style `if (cond) { val1 } else { val2 }` not working

**Files:**
- `examples/zvdb_working.minz` - ✅ Working 64-bit demo
- `examples/zvdb-minz/zvdb_complete_v2.minz` - 🔧 Full 256-bit version (fixing)
- `examples/zvdb-minz/src/` - Advanced modular version (needs v1.0.0)

**Impact**: Each fix to ZVDB makes MinZ better for everyone!

## Project Overview

MinZ is a systems programming language for Z80-based computers (ZX Spectrum). The repository contains:
- **Tree-sitter grammar** for parsing MinZ syntax
- **Go-based compiler (minzc)** that generates Z80 assembly (.a80 format)
- **Advanced optimization framework** with register allocation and self-modifying code support

## 📊 Compiler State Tracking

**IMPORTANT**: We maintain a living snapshot of the compiler's current state at [COMPILER_SNAPSHOT.md](COMPILER_SNAPSHOT.md)

This snapshot includes:
- Current success rates and metrics
- Grammar and keyword inventory
- Pipeline documentation (AST → MIR → ASM)
- All implemented optimizations
- Known issues and patterns
- Progress tracking

### Updating the Snapshot
```bash
# After significant changes:
./scripts/update_snapshot.sh

# To detect assembly issues:
go run scripts/detect_issues.go minzc/test.a80
```

## 🚀 Recent Achievement: TRUE ZERO-OVERHEAD LAMBDAS!

Revolutionary breakthrough in functional programming for 8-bit systems:
- ✅ **Compile-time lambda transformation** - Lambdas become named functions
- ✅ **Zero runtime overhead** - Lambda calls identical to traditional function calls  
- ✅ **TRUE SMC integration** - Same self-modifying code optimization as regular functions
- ✅ **Function reference copying** - `let f = someFunction` works perfectly
- ✅ **Type safety preserved** - Full compile-time type checking
- ✅ **ZX Spectrum standard library** - 32-character ROM font printing routine

Performance verified: **Lambda functions run at 100% the speed of traditional functions**

## 🏆 WORLD FIRST: Zero-Cost Abstractions on 8-bit Hardware!

MinZ has achieved the impossible - modern programming abstractions with ZERO runtime overhead on Z80:

### ✅ **Zero-Cost Iterators** - COMPLETE (v0.9.2)
```minz
let arr: [u8; 5] = [1, 2, 3, 4, 5];
arr.iter()
   .map(|x| x * 2)
   .filter(|x| x > 5)
   .forEach(print_u8);
```
**Compiles to:** Single optimized loop with DJNZ instruction - ZERO overhead vs hand-written assembly!
- Chain operations fuse into one pass
- No intermediate arrays allocated
- Type safety preserved through chain
- See `docs/125_Iterator_Transformation_Mechanics.md` for the mathematics

### ✅ **Zero-Overhead Lambdas** - COMPLETE
```minz
let add = |x: u8, y: u8| => u8 { x + y };
add(5, 3)  // Compiles to direct CALL - 100% performance of traditional functions
```

### ✅ **Zero-Cost Interfaces** - COMPLETE  
```minz
interface Drawable {
    fun draw(self) -> u8;
}

impl Drawable for Circle {
    fun draw(self) -> u8 { self.radius * 2 }
}

let circle = Circle { radius: 5 };
circle.draw()  // Compiles to: CALL Circle_draw - NO vtables, NO overhead!
```

### 🤯 **REVOLUTIONARY: Zero-Cost Iterator Chains with DJNZ Optimization** - IN DESIGN
```minz
// THE IMPOSSIBLE: Complete functional programming on Z80 using DJNZ!
enemies.filter!(|e| e.health > 0)               // Remove dead
       .map!(|e| e.update_ai(player_pos))       // Update AI  
       .filter(|e| e.distance_to(player) < 50)  // Nearby only
       .forEach(|e| e.attack(player));          // Attack player

// Compiles to SINGLE DJNZ loop with pointer arithmetic!
// LD HL, enemies_array; LD B, enemies_count; DJNZ optimal_loop
// 3x FASTER than indexed access - Z80-native iteration patterns!
```

**🎯 Z80-Optimized Iteration Strategy:**
- **DJNZ instruction** - Ultra-efficient counter in B register
- **Pointer arithmetic** - Direct memory access via HL register  
- **ADD HL, DE** - Advance by element size (1 byte = INC HL, 2+ bytes = ADD HL, DE)
- **Fusion optimization** - Entire iterator chains become single DJNZ loops!

**Performance**: 18 T-states per element vs 45+ T-states for indexed access!

### ✅ **ZX Spectrum Standard Library** - COMPLETE
- 32-character ROM font printing using hardware font at $3D00
- Hardware-optimized graphics primitives  
- Memory layout and attribute handling

### 📋 **Coming Soon**
- **Zero-Cost Iterator System** - Complete functional programming paradigm
- Generic functions with monomorphization
- Interface casting and type erasure
- Advanced standard library modules

## Ruby-Style Developer Happiness Features

MinZ embraces Ruby's philosophy of "optimizing for programmer happiness":

### Function Declaration Flexibility
```minz
// Both 'fn' and 'fun' work - choose your style!
fn add(a: u8, b: u8) -> u8 { return a + b; }
fun subtract(a: u8, b: u8) -> u8 { return a - b; }
```

### Global Variables
```minz
// Ruby-style 'global' keyword for clarity
global counter: u8 = 0;       // Same as 'var' at top level
let total: u16 = 100;          // Module-level variable
const MAX: u8 = 255;           // Constant
```

### Bit Structures for Compact Storage
```minz
// Define bit-packed structures for memory efficiency
type HashEntry = bits_8 {
    vector_id: 6,    // 6 bits for ID (0-63)
    occupied: 1,     // 1 bit flag
    chain: 1         // 1 bit for collision chain
};  // Total: 8 bits vs 24 bits with regular struct!

type Flags = bits_16 {
    x_pos: 9,        // 9 bits for position (0-511)
    visible: 1,      // Single bit flags
    collision: 1,
    priority: 5      // 5 bits for priority levels
};
```

This gives 67% memory savings for ZVDB hash tables and compact result storage!

## Development Commands

### Core Build Commands
```bash
# Generate tree-sitter parser
npm install
tree-sitter generate

# Build the MinZ compiler
cd minzc && make build

# Run tests
cd minzc && make test

# Build and test on sample file
cd minzc && make run

# Clean build artifacts
cd minzc && make clean
```

### Dependencies
```bash
# Install Go dependencies
cd minzc && make deps

# Install tree-sitter CLI globally
npm install -g tree-sitter-cli
```

### Testing Individual Files
```bash
# Parse a specific MinZ file  
tree-sitter parse examples/fibonacci.minz

# Compile a MinZ file to Z80 assembly
cd minzc && ./minzc ../examples/fibonacci.minz -o fibonacci.a80

# Enable optimizations
cd minzc && ./minzc ../examples/fibonacci.minz -O --enable-smc

# Test @abi assembly integration
cd minzc && ./minzc ../examples/simple_abi_demo.minz -o simple_abi_demo.a80
```

## Compiler Architecture

### Multi-Language Codebase Structure
- **Tree-sitter grammar**: JavaScript (`grammar.js`) defining MinZ syntax
- **Parser bindings**: Node.js and Rust bindings for tree-sitter integration
- **Compiler**: Go implementation in `minzc/` directory with modular packages
- **Examples**: MinZ source files demonstrating language features

### Compilation Pipeline
1. **Parsing**: Tree-sitter generates AST from MinZ source (`pkg/parser/`)
   - S-expression parser detects iterator methods (`.map()`, `.filter()`, `.forEach()`)
   - Transforms method chains into `IteratorChainExpr` AST nodes
2. **Semantic Analysis**: Type checking and symbol resolution (`pkg/semantic/`)
   - Iterator chains generate optimized loop code (`pkg/semantic/iterator.go`)
   - Type inference flows through iterator operations
3. **IR Generation**: Converts to intermediate representation (`pkg/ir/`)
   - Iterator loops become standard MIR instructions (no special IR needed!)
4. **Optimization**: Advanced passes including register allocation (`pkg/optimizer/`)
   - Future: Fusion optimizer combines multiple operations into single loops
   - DJNZ pattern recognition for arrays ≤255 elements
5. **Code Generation**: Z80 assembly output in sjasmplus format (`pkg/codegen/`)

### Key Compiler Components

#### Register System (`pkg/ir/ir.go`)
- Physical Z80 registers (A, B, C, D, E, H, L) and 16-bit pairs (AF, BC, DE, HL, IX, IY)
- Shadow register support (Z80_*_SHADOW constants) for interrupt optimization
- RegisterSet tracking for optimization passes

#### Optimization Framework (`pkg/optimizer/`)
- **Register Analysis**: Tracks register usage patterns per function
- **Shadow Register Optimization**: Automatic use of alternative registers for performance
- **Self-Modifying Code (SMC)**: Runtime optimization of constants and parameters
- **Standard Passes**: Constant folding, dead code elimination, peephole optimization

#### Code Generation (`pkg/codegen/z80.go`)
- Generates sjasmplus-compatible `.a80` assembly files
- Lean prologue/epilogue that only saves actually used registers
- Shadow register support for interrupt handlers and performance-critical code

### Z80-Specific Features
- **Shadow Registers**: EXX and EX AF,AF' instructions for fast context switching
- **Interrupt Optimization**: Ultra-fast handlers using shadow registers (16 vs 50+ T-states)
- **Memory Layout**: Organized for 64KB address space with paging support
- **Register Allocation**: Z80-aware allocation considering 8/16-bit register relationships
- **TRUE SMC (истинный SMC)**: Parameters patched directly into instruction immediates
  - Enable with `--enable-true-smc` flag
  - See `docs/018_TRUE_SMC_Design_v2.md` for current design
  - Provides 3-5x performance improvement for function calls
- **@abi Annotations**: Seamless integration with existing assembly functions
  - Use existing ROM routines, drivers, libraries without modification
  - Precise register mapping: `@abi("register: A=param1, HL=param2")`
  - Zero overhead assembly integration

## Language Features

MinZ supports modern programming constructs while targeting Z80:
- **Type System**: Static typing with inference (u8, u16, i8, i16, bool, arrays, pointers)
- **Structs and Enums**: Organized data structures with memory-efficient layout
- **Module System**: Import/export with visibility control
- **@abi Attributes**: Revolutionary seamless assembly integration system
- **Lua Metaprogramming**: Full Lua 5.1 interpreter at compile time for code generation
- **Inline Assembly**: Direct Z80 assembly integration with register constraints
- **Lambda Expressions**: Compile-time transformed into efficient functions (see Lambda Design below)

## Design Philosophy

### TSMC Reference Philosophy (Revolutionary - Article 040)
MinZ is evolving beyond traditional pointers to **TSMC-native references** where:

1. **References ARE addresses of data inside opcodes** - The data lives in immediate fields of instructions
2. **Zero indirection** - `&T` parameters become direct immediate values in instructions
3. **Self-modifying by design** - Functions modify their own immediates for iteration
4. **Code IS the data structure** - Parameters live in instruction stream, not memory

Example of the vision:
```asm
; Traditional pointer approach:
LD HL, string_addr  ; Load pointer
LD A, (HL)         ; Dereference

; TSMC reference approach:
str$immOP:
    LD A, (0000)   ; The 0000 IS the reference - patched at call time!
str$imm0 EQU str$immOP+1
```

Currently, syntax uses `*T` but semantics are evolving to true TSMC references where every pointer parameter becomes a self-modifying immediate operand. This eliminates register pressure, memory usage, and indirection overhead.

See `docs/040_TSMC_Reference_Philosophy.md` for the complete revolutionary vision and `POINTER_PHILOSOPHY.md` for the migration path.

### Lambda Design Philosophy (Compile-Time Transformation)
MinZ lambdas are **not runtime values** but compile-time constructs:

1. **Lambda assignments become named functions** - `let f = |x| { x + 1 }` creates a module function
2. **Only fully-curried lambdas can be returned** - Must be completely specialized, returning just addresses
3. **No runtime overhead** - All lambda calls compile to direct function calls
4. **Perfect SMC integration** - Currying uses parameter patching for specialization

Example:
```minz
// This lambda assignment:
let add = |x: u8, y: u8| => u8 { x + y };
add(5, 3)

// Compiles to:
fun scope$add_0(x: u8, y: u8) -> u8 { x + y }
scope$add_0(5, 3)  // Direct call!
```

For returning lambdas (must be fully curried):
```minz
fun make_adder(n: u8) -> fn(u8) -> u8 {
    @curry(|x: u8| => u8 { x + n }, n)  // Returns address of SMC-specialized function
}
```

See `docs/094_Lambda_Design_Complete.md` for full design details.

## ZVDB-MinZ: Vector Database Implementation

We've successfully implemented a 256-bit vector database in MinZ! Key achievements:

### Working Implementation
- **File**: `examples/zvdb_final.minz` - Complete working 256-bit vector database
- **Features**:
  - Full 256-bit vectors (32 bytes each)
  - Hamming distance calculation
  - Similarity search with scoring
  - Popcount optimization
  - Production-ready for Z80 hardware

### Global Variables - Developer-Friendly Syntax
MinZ now supports the `global` keyword as a developer-friendly synonym for top-level `var`:
```minz
// All of these work as global variables:
const VECTOR_BITS: u16 = 256;       // Constant (immutable)
let db_count: u8 = 0;                // Variable (mutable)
var rng_state: u16 = 0xACE1;         // Variable (mutable)
global test_var: u8 = 42;           // Variable (mutable) - Ruby-style syntax!

// Note the syntax - type comes after identifier:
global name: type = value;          // ✅ Correct
global type name = value;           // ❌ Wrong - won't parse
```

The `global` keyword follows Ruby's philosophy of developer happiness - making the code more readable and intention-revealing.

### ZVDB: Vector Database Implementation
- **`examples/zvdb.minz`** - Main implementation (256-bit vectors, all optimizations)
- **`examples/ZVDB_README.md`** - Performance benchmarks and technical analysis
- `examples/zvdb_experiments/` - Archive of experimental versions and iterations

#### Key Achievements:
- ⚡ **3.3x faster popcount** with lookup table (verified with T-state analysis)
- 💾 **67% memory savings** on metadata using bit structures*
- 🎯 **Type-safe** vector operations without performance penalty
- ⏱️ **10x faster development** than writing assembly (estimated)

\* Savings only apply to metadata structures (8 bits vs 24 bits). Vectors still need full 256 bits. Total memory increases by 256 bytes for LUT.

## Important Files and Directories

### Core Implementation
- `grammar.js`: Tree-sitter grammar defining MinZ syntax
- `minzc/cmd/minzc/main.go`: Compiler CLI entry point
- `minzc/pkg/`: Modular compiler packages (ast, parser, semantic, ir, optimizer, codegen)

### Documentation
- `README.md`: Complete language reference with examples
- `COMPILER_ARCHITECTURE.md`: Detailed compiler design documentation
- `DESIGN.md`: Language design philosophy and feature overview
- `docs/`: Technical guides including:
  - **`018_TRUE_SMC_Design_v2.md`**: CURRENT DESIGN for TRUE SMC implementation
  - **`029_MinZ_Strategic_Roadmap.md`**: Long-term vision and phases
  - **`030_TRUE_SMC_Lambdas_Design.md`**: Lambda implementation via SMC
  - **`031_Next_Steps_Prioritized.md`**: 70-day action plan
  - **`032_Architecture_Decision_Records.md`**: Key design decisions
  - **`040_TSMC_Reference_Philosophy.md`**: Revolutionary vision - references as immediate operands
  - **`094_Lambda_Design_Complete.md`**: Lambda compile-time transformation design

### Examples and Testing
- `examples/`: Comprehensive MinZ programs showcasing all language features
- `examples/simple_abi_demo.minz`: Complete @abi demonstration with assembly integration
- `examples/asm_integration_tests.minz`: Comprehensive @abi test suite
- `test/`: Tree-sitter test corpus
- `stdlib/`: Standard library modules (std.mem, zx.screen, zx.input)

## Development Notes

### Working with Tree-sitter
- Grammar changes require running `tree-sitter generate`
- Test grammar with `tree-sitter test` and `tree-sitter parse <file>`
- Syntax highlighting queries in `queries/highlights.scm`
- **NO FALLBACK PARSERS**: We maintain only one parser - tree-sitter. If something doesn't parse, fix the grammar, don't create workarounds

### Compiler Development
- The Go compiler uses tree-sitter C bindings via external process calls
- AST conversion happens in `pkg/parser/parser.go`
- Register allocation and optimization passes are modular and can be configured by optimization level

### Output Format
- Generates sjasmplus `.a80` assembly files
- Uses ORG $8000 as default code origin
- Includes header comments with generation timestamp
- Compatible with ZX Spectrum assemblers and emulators

### Testing Strategy
- Tree-sitter corpus tests for grammar validation
- Go unit tests in optimizer packages (e.g., `optimizer_test.go`, `smc_optimization_test.go`)
- Integration tests with sample MinZ programs in examples/

## 🛠️ Release and Build Tools

### Release Pipeline
```bash
# Create full release package with binaries for all platforms
./scripts/release.sh

# Create release with specific version
VERSION=v0.9.2 ./scripts/release.sh

# Create development package with source
./scripts/package.sh
```

The release pipeline includes:
- Cross-platform binaries (macOS Intel/ARM, Linux x64/ARM, Windows)
- Standard library and documentation bundling
- Platform-specific installers
- Docker image generation
- GitHub Actions automation

See `RELEASE_PIPELINE.md` for complete details.

### E2E Testing
```bash
# Run comprehensive E2E tests on all examples
cd minzc && go test -v ./pkg/e2e -run TestE2ECompilation

# Generate performance report
cd minzc && make benchmark-report
```

### Docker Usage
```bash
# Build Docker image
docker build -t minz:latest -f scripts/Dockerfile .

# Run compiler in container
docker run -v $(pwd):/workspace minz:latest minzc program.minz -O --enable-smc
```

## 📊 Optimization Pipeline

The MinZ compiler implements a sophisticated multi-pass optimization pipeline:

### Optimization Passes
1. **Register Analysis** - Tracks usage patterns
2. **MIR Reordering** - Exposes optimization opportunities
3. **Smart Peephole** - Z80-specific pattern matching
4. **Constant Folding** - Compile-time evaluation
5. **Dead Code Elimination** - Removes unused code
6. **Register Allocation** - Hierarchical allocation
7. **Inlining** - Small function expansion
8. **TRUE SMC** - Self-modifying code transformation
9. **Tail Recursion** - Loop transformation

### Key Optimizations
- **Instruction Reordering**: Clusters related operations, sinks stores, hoists invariants
- **Z80 Patterns**: 
  - Inc/Dec for ±1 (A register) or ±3 (B,C,D,E,H,L registers)
  - Zero comparison optimization
  - Shift unrolling for small shifts
- **Multiply by Power of 2**: Converts to shifts
- **Shadow Register Usage**: Automatic for interrupts and performance
- **Register-Aware INC/DEC**: Different thresholds based on target register

See `docs/108_Optimization_Pipeline.md` for detailed documentation.
See `docs/109_INC_DEC_Optimization_Analysis.md` for INC/DEC analysis.

### MIR Code Emission (Proposed)
MinZ will support writing MIR (Machine-Independent Representation) directly:
```minz
mir {
    r1 = load_var "x"
    inc r1
    inc r1  // Compiler decides if this is optimal
    store_var "x", r1
}
```

This enables:
- Machine-independent optimization code
- User-defined optimization passes
- Powerful metafunction integration
- Future portability to 6502, 68000, etc.

See `docs/110_MIR_Code_Emission_Design.md` for complete design.