# MinZ TODO - The Big Next Things

> Last Updated: January 2025 (v0.14.0)

## 🎯 Current Focus Areas

### 1. 🔧 Tree-Sitter Parser Fix (IMMEDIATE PRIORITY)
**Goal:** Reach 90%+ compilation success rate
- [ ] Fix parsing of complex expressions
- [ ] Handle all array literal cases
- [ ] Fix method call syntax
- [ ] Improve error recovery
- [ ] Create comprehensive parser test suite
- **Note:** ANTLR is PARKED (regression from 75% to 5%)
- **Current:** 63% success rate

### 2. 📦 MZP Package Manager (HIGH VALUE)
**Goal:** Simple package management for retro systems
- [ ] Implement manifest parsing (mzp.toml)
- [ ] Create local package installation
- [ ] Build dependency resolver
- [ ] Integrate with mz compiler
- [ ] Set up package registry (GitHub Pages)
- **Design:** See [doc #232](docs/232_MZP_Package_Manager_Design.md)

### 3. 🎮 Game Jam: Snake & Tetris (PROOF OF CONCEPT)
**Goal:** Create real, playable games for ZX Spectrum
- [ ] Build ZX Spectrum graphics library (attribute-based)
- [ ] Implement input handling module
- [ ] Create Snake game
- [ ] Create Tetris with rotation system
- [ ] Polish with title screens and sound
- **Plan:** See [doc #231](docs/231_Game_Jam_Snake_Tetris_Plan.md)

### 4. 💻 Language Server Protocol (DEVELOPER EXPERIENCE)
**Goal:** Professional IDE integration
- [ ] Implement LSP server in Go
- [ ] Add autocomplete support
- [ ] Add go-to-definition
- [ ] Create VSCode extension
- [ ] Support Vim/Emacs

### 5. ⚡ Optimization Improvements (PERFORMANCE)
**Goal:** Better code generation and optimization
- [ ] Improve tree-shaking beyond 74%
- [ ] Implement canonical reordering
- [ ] Add more peephole patterns (target: 50+)
- [ ] Implement currying for all functions
- [ ] Optimize lambda transformations

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