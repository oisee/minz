# MinZ Language Specification Article Plan

## Article Title
**"MinZ: A Zero-Cost Abstraction Language for Z80 - From Theory to Implementation"**

## Target Audience
- Language designers interested in embedded systems
- Z80/retro computing enthusiasts
- Compiler engineers
- Systems programmers

## Article Structure

### Part I: Language Philosophy & Design
*Ground truth from actual implementation*

#### 1.1 Core Philosophy
**Source:** `docs/145_TSMC_Complete_Philosophy.md`
- Zero-cost abstractions on 8-bit hardware
- True self-modifying code as optimization
- Modern syntax, vintage performance

#### 1.2 Type System
**Source:** `minzc/pkg/semantic/analyzer.go:270-280` (addBuiltins)
```go
// Built-in types directly from code
u8, u16, i8, i16, bool, void
// Fixed-point: f8.8, f16.8
```

#### 1.3 Memory Model
**Source:** `minzc/pkg/codegen/z80.go:850-950` (register allocation)
- Hierarchical register allocation
- Shadow register utilization
- SMC-based locals

### Part II: Parser Architecture
*Dual-parser approach with grounded examples*

#### 2.1 Grammar Definition
**Source:** `grammar.js` (Tree-sitter)
**Source:** `minzc/grammar/MinZ.g4` (ANTLR)

Show actual grammar rules:
```javascript
// From grammar.js
function_declaration: $ => seq(
    choice('fun', 'fn'),
    $.identifier,
    '(',
    optional($.parameter_list),
    ')',
    '->', 
    $.type,
    $.block
)
```

#### 2.2 AST Structure
**Source:** `minzc/pkg/ast/ast.go`
- Node types hierarchy
- Declaration vs Expression nodes
- Actual struct definitions

### Part III: Semantic Analysis
*How MinZ understands code*

#### 3.1 Two-Pass Analysis
**Source:** `minzc/pkg/semantic/analyzer.go:171-240`
```go
// First pass: Register types and signatures
// Second pass: Analyze function bodies
```

#### 3.2 Type Checking
**Source:** `minzc/pkg/semantic/analyzer.go:4500-4600`
- Type inference implementation
- Overload resolution algorithm
- Interface dispatch

### Part IV: MIR - MinZ Intermediate Representation
*The heart of optimization*

#### 4.1 MIR Instructions
**Source:** `minzc/pkg/ir/mir.go`
```go
type OpCode int
const (
    OpAdd OpCode = iota
    OpSub
    OpMul
    // ... actual opcodes
)
```

#### 4.2 MIR Generation
**Source:** `minzc/pkg/semantic/mir_gen.go`
- SSA-style IR generation
- Register allocation strategy

#### 4.3 MIR Optimization
**Source:** `minzc/pkg/optimizer/`
- Peephole patterns: `minzc/pkg/codegen/peephole_z80.go`
- Constant folding
- Dead code elimination

### Part V: Code Generation
*Platform-specific backends*

#### 5.1 Multi-Backend Architecture
**Source:** `minzc/pkg/codegen/`
```
codegen/
├── z80.go       // Primary target
├── mos6502.go   // 6502 backend
├── m68k.go      // 68000 backend
├── c.go         // C transpiler
├── llvm.go      // LLVM IR
└── wasm.go      // WebAssembly
```

#### 5.2 Z80 Optimization Techniques
**Source:** `minzc/pkg/codegen/z80.go:2000-3000`
- DJNZ loop optimization
- Register pair utilization
- Self-modifying code generation

#### 5.3 Tree-Shaking Implementation
**Source:** `minzc/pkg/codegen/z80.go` (usedFunctions tracking)
**Source:** `minzc/pkg/codegen/z80_stdlib.go`
- 74% size reduction achieved
- Conditional stdlib generation

### Part VI: Metaprogramming System
*Compile-time code generation*

#### 6.1 Preprocessor (@define)
**Source:** `minzc/pkg/semantic/template_expander.go`
```go
// Template expansion before parsing
type Template struct {
    Name       string
    Parameters []string
    Body       string
}
```

#### 6.2 Compile-Time Execution (@minz)
**Source:** `minzc/pkg/semantic/minz_interpreter.go`
- @emit mechanism
- Immediate execution model
- Generated code injection

#### 6.3 Lua Integration (@lua)
**Source:** `minzc/pkg/meta/lua_evaluator.go`
- Full Lua VM at compile-time
- emit() function binding

### Part VII: Optimization Pipeline
*How MinZ achieves zero-cost abstractions*

#### 7.1 Lambda-to-Function Transform
**Source:** `minzc/pkg/semantic/lambda_transform.go`
- Iterator chain optimization
- DJNZ generation for loops

#### 7.2 Interface Devirtualization
**Source:** `minzc/pkg/semantic/analyzer.go:7500-7600`
- Static dispatch when possible
- Zero-cost interfaces

#### 7.3 True Self-Modifying Code (TSMC)
**Source:** `docs/145_TSMC_Complete_Philosophy.md`
**Source:** `minzc/pkg/codegen/z80_smc.go`
- Instruction patching
- Parameter injection
- Behavioral morphing

### Part VIII: Toolchain
*Complete development environment*

#### 8.1 Compiler (mz)
**Source:** `minzc/cmd/minzc/main.go`
- CLI interface
- Multi-backend selection

#### 8.2 Assembler (mza)
**Source:** `minzc/pkg/z80asm/assembler.go`
- Macro support
- Built-in Z80 assembler

#### 8.3 Emulator (mze)
**Source:** `minzc/pkg/emulator/z80.go`
- Complete Z80 emulation
- Integrated debugger

#### 8.4 REPL (mzr)
**Source:** `minzc/cmd/repl/main.go`
- Interactive development
- History support

#### 8.5 MIR VM (mzv)
**Source:** `minzc/pkg/mir/interpreter/interpreter.go`
- MIR testing
- Compile-time execution

### Part IX: Real-World Examples
*Actual working code*

#### 9.1 Basic Programs
**Source:** `examples/fibonacci.minz`
**Source:** `examples/simple_test.minz`

#### 9.2 Advanced Features
**Source:** `examples/interface_simple.minz`
**Source:** `examples/lambda_simple_test.minz`

#### 9.3 Metaprogramming
**Source:** `test_minz_immediate.minz`
**Source:** `tests/minz/test_define.minz`

### Part X: Performance Analysis
*Measurable results*

#### 10.1 Compilation Metrics
**Source:** `docs/225_Tree_Shaking_Implementation_E2E_Report.md`
- Size reductions
- Optimization effectiveness

#### 10.2 Runtime Performance
**Source:** `docs/145_TSMC_Complete_Philosophy.md`
- T-state comparisons
- SMC benefits

### Part XI: Future Directions
*Based on actual TODO items and roadmap*

#### 11.1 Language Features
**Source:** `STABILITY_ROADMAP.md`
- Remaining 20% to implement
- Generic types decision

#### 11.2 Optimization Opportunities
**Source:** `docs/149_World_Class_Multi_Level_Optimization_Guide.md`
- Advanced peephole patterns
- Global optimization

## Supporting Materials

### Code Repository Structure
```
minz-ts/
├── minzc/              # Go compiler implementation
│   ├── cmd/           # CLI tools
│   ├── pkg/           # Core packages
│   │   ├── ast/       # AST definitions
│   │   ├── parser/    # Dual parsers
│   │   ├── semantic/  # Analysis
│   │   ├── ir/        # MIR
│   │   ├── codegen/   # Backends
│   │   └── optimizer/ # Optimizations
│   └── tests/         # Test suite
├── grammar.js         # Tree-sitter grammar
├── examples/          # Working examples
└── docs/             # 226+ documentation files
```

### Key Documentation References
1. `docs/145_TSMC_Complete_Philosophy.md` - Core innovation
2. `docs/149_World_Class_Multi_Level_Optimization_Guide.md` - Optimization strategy
3. `docs/225_Tree_Shaking_Implementation_E2E_Report.md` - Size optimization
4. `docs/226_Metafunction_Design_Decisions.md` - Metaprogramming design
5. `minzc/docs/INTERNAL_ARCHITECTURE.md` - Complete internals
6. `STABILITY_ROADMAP.md` - Path to v1.0
7. `CLAUDE.md` - AI assistance integration

### Metrics to Include
- **Compilation success rate:** 63% (tree-sitter)
- **Tree-shaking reduction:** 74%
- **Number of backends:** 8
- **Peephole patterns:** 35+
- **Documentation files:** 226+
- **Test examples:** 170+

### Code Snippets Format
Each code example should:
1. Reference the actual source file
2. Include line numbers where applicable
3. Show before/after for optimizations
4. Link to full implementation

### Visual Diagrams to Create
1. **Parser Pipeline:** Source → Tokens → AST → MIR → Assembly
2. **Type Hierarchy:** Basic types → Composite → Interfaces
3. **Register Allocation:** Physical → Shadow → Memory
4. **Optimization Pipeline:** Peephole → Tree-shaking → SMC
5. **Backend Architecture:** MIR → Platform-specific generators

## Article Tone
- **Technical but accessible**
- **Grounded in actual implementation**
- **Honest about limitations**
- **Excited about achievements**
- **Links to real code throughout**

## Publishing Strategy
1. **Technical blog post** (10-15 min read)
2. **GitHub repository documentation**
3. **Conference paper** (if expanded)
4. **Series of smaller articles** (each part standalone)

## Call to Action
- Try MinZ online (when playground ready)
- Contribute to the compiler
- Port to new platforms
- Build retro software with modern tools

---

*This plan ensures every claim in the article can be verified by examining actual source code, making it a truly grounded technical specification rather than aspirational documentation.*