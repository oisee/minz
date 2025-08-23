# MinZ TODO - The Big Next Things

> Last Updated: January 2025 (v0.14.0)
> Success Rate: 47/58 examples (81%)

## 🎯 Current Focus Areas

### 1. 🎯 Pattern Matching (case/match) - HIGHEST IMPACT
**Goal:** Swift/Rust-style pattern matching with Z80 jump tables
- [ ] Implement case/match syntax parsing
- [ ] Generate efficient jump tables (<20 T-states)
- [ ] Support range patterns (1..10)
- [ ] Support enum patterns with dot notation
- [ ] Default case with `_`
```minz
case state {
    State.IDLE => State.RUNNING,     // ✅ Use dot notation
    State.RUNNING => State.STOPPED,
    _ => State.IDLE
}
```

### 2. 🔧 Fix Enum Member Access
**Goal:** Make State.IDLE syntax work properly
- [ ] Fix semantic analyzer for enum dot notation
- [ ] No `::` operator - only dot notation
- [ ] Should work in all contexts (case, assignments, comparisons)
- **Note:** Parser already recognizes it, semantic layer needs fix

### 3. 📦 Local/Nested Functions
**Goal:** Enable function definitions inside functions
- [ ] Parse nested function declarations
- [ ] Handle closure/scope properly
- [ ] Unblock 7+ failing examples
```minz
fun outer() -> void {
    fun inner() -> void {
        print("nested!");
    }
    inner();
}
```

### 4. 🔧 Tree-Sitter Parser Improvements
**Goal:** Reach 90%+ compilation success rate
- [ ] Fix parsing of complex expressions
- [ ] Handle all array literal cases
- [ ] Fix method call syntax
- [ ] Improve error recovery
- [ ] Create comprehensive parser test suite
- **Current:** 81% success rate (47/58 examples)

### 5. 📦 MZP Package Manager (HIGH VALUE)
**Goal:** Simple package management for retro systems
- [ ] Implement manifest parsing (mzp.toml)
- [ ] Create local package installation
- [ ] Build dependency resolver
- [ ] Integrate with mz compiler
- [ ] Set up package registry (GitHub Pages)
- **Design:** See [doc #232](docs/232_MZP_Package_Manager_Design.md)

### 6. 🎮 Game Jam: Snake & Tetris (PROOF OF CONCEPT)
**Goal:** Create real, playable games for ZX Spectrum
- [ ] Build ZX Spectrum graphics library (attribute-based)
- [ ] Implement input handling module
- [ ] Create Snake game
- [ ] Create Tetris with rotation system
- [ ] Polish with title screens and sound
- **Plan:** See [doc #231](docs/231_Game_Jam_Snake_Tetris_Plan.md)

### 7. 💻 Language Server Protocol (DEVELOPER EXPERIENCE)
**Goal:** Professional IDE integration
- [ ] Implement LSP server in Go
- [ ] Add autocomplete support
- [ ] Add go-to-definition
- [ ] Create VSCode extension
- [ ] Support Vim/Emacs

### 8. ⚡ Optimization Improvements (PERFORMANCE)
**Goal:** Better code generation and optimization
- [ ] Improve tree-shaking beyond 74%
- [ ] Implement canonical reordering
- [ ] Add more peephole patterns (target: 50+)
- [ ] Implement currying for all functions
- [ ] Optimize lambda transformations

## 🔧 Additional High-Impact Features

### Module Import System
**Goal:** Better code organization with dot notation
```minz
import zx.screen;
import std.mem;

fun main() -> void {
    screen.set_border(2);  // Namespace access
    mem.copy(src, dst, len);
}
```

### Error Propagation with `?`
**Goal:** Modern error handling
```minz
fun read_file(name: str) -> u8? {
    let file = open(name)?;  // Propagate error
    return file.read_byte();
}
```

### Self Parameter in Methods
**Goal:** Enable impl blocks with self
```minz
impl State {
    fun next(self) -> State {  // 'self' not recognized currently
        case self {
            State.IDLE => State.RUNNING,
            State.RUNNING => State.STOPPED
        }
    }
}
```

### Template Substitution in @minz blocks
- [ ] Fix for loop execution in @minz blocks
- [ ] Add Ruby-style `#{variable}` interpolation
- [ ] String concatenation for code generation

## ✅ Recently Completed (v0.14.0)

### Tree-Shaking Implementation
- [x] Only include used stdlib functions
- [x] 74% size reduction achieved (324 → 85 lines)
- [x] Modularized stdlib generation

### Metafunction Clarification
- [x] @minz[[[]]] - Immediate execution (no args!)
- [x] @define() - Template preprocessor (working!)
- [x] @lua[[[]]] - Lua scripting
- [x] Complete documentation

### CTIE (Compile-Time Interface Execution)
- [x] Pure functions executed at compile-time
- [x] 44+ T-state savings (negative-cost abstractions!)
- [x] 43.8% of functions optimizable
- [x] Replaces CALL with immediate values

### Toolchain Enhancements
- [x] mzv - MIR VM interpreter
- [x] mza - Macro support
- [x] mzr - REPL with history
- [x] mze - Full debugger
- [x] MCP - AI colleague integration

## 🚫 PARKED (Not Currently Active)

### ANTLR Parser - PARKED
- Was default parser in early v0.14.0
- Regression from 75% to 5% success rate
- Focus shifted to tree-sitter
- May revisit after tree-sitter reaches 90%

### Generic Types - PARKED
- Design phase only
- Not critical for current use cases
- Revisit after core stability

### Incremental Compilation - PARKED
- Compilation already fast enough (~200ms for 1000 lines)
- Not blocking development
- Revisit for large projects

### Web Playground - FUTURE
- Would be nice for adoption
- Requires WASM compilation
- Not priority for MVP

### Self-Hosting - DISTANT FUTURE
- Ultimate goal
- Requires 100% feature completeness
- Would be amazing achievement

## 📊 Success Metrics

### Parser Success
- **Current:** 63% (tree-sitter)
- **Target:** 90%+ 
- **Measure:** examples/ compilation rate

### Package Manager
- **MVP:** 10 working packages
- **Success:** 100+ packages in registry
- **Measure:** Successful installs and builds

### Games Performance
- **Snake:** 50 FPS with 100+ segments
- **Tetris:** No lag at level 20
- **Size:** Under 16KB each

### Optimization
- **Tree-shaking:** >80% reduction (from current 74%)
- **Peephole:** 50+ patterns (from current 35)
- **Benchmarks:** Match hand-coded assembly

## 🗓️ Rough Timeline

### Phase 1: Parser Fix (Weeks 1-2)
- Get to 75% success rate
- Fix critical parsing bugs
- Add test coverage

### Phase 2: Core Libraries (Weeks 3-4)
- ZX Spectrum modules
- Package manager MVP
- Basic registry

### Phase 3: Game Development (Weeks 5-8)
- Snake implementation
- Tetris implementation
- Library refinement

### Phase 4: Polish (Weeks 9-10)
- LSP implementation
- Optimization improvements
- Documentation

## 🔗 Key Documents

- [Complete Language Specification](docs/230_MinZ_Complete_Language_Specification.md)
- [Game Jam Plan](docs/231_Game_Jam_Snake_Tetris_Plan.md)
- [Package Manager Design](docs/232_MZP_Package_Manager_Design.md)
- [Tree-Shaking Report](docs/225_Tree_Shaking_Implementation_E2E_Report.md)
- [Metafunction Design](docs/226_Metafunction_Design_Decisions.md)
- [Session Report](docs/229_Session_Achievement_Report_v0_14_0.md)

## 💡 Why This Plan?

1. **Parser First** - Nothing works without parsing
2. **Games Prove It** - Real programs find real bugs
3. **Packages Enable Sharing** - Community growth
4. **Developer Experience Matters** - LSP makes it professional
5. **Performance Is Key** - Zero-cost or nothing

## 🚀 How to Contribute

1. **Fix parser bugs** - Most impactful
2. **Write game libraries** - Graphics, sound, input
3. **Create packages** - Share your code
4. **Test and report** - Find edge cases
5. **Document patterns** - Help others learn

## 📝 Previous TODO Items (Archived)

### From Architecture Audit (August 2024)
Many quick wins from the audit have been completed:
- ✅ Import statements work
- ✅ Error handling improved
- ✅ String literals functional
- ✅ Array support enhanced
- ⏸️ Some items superseded by new priorities

See git history for the original Architecture Audit TODO.

---

*This TODO represents the practical path forward for MinZ: Fix parsing, build real programs, enable code sharing, and optimize relentlessly. Everything else is secondary.*