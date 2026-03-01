# MinZ Development Plan (GenPlan)

*Living document — canonical development roadmap. Update as items are completed or priorities shift.*

*Last updated: March 2026*

---

## Current State (v0.19.4, March 2026)

| Metric | Value |
|--------|-------|
| Core examples | ~73 (100%) |
| All examples | ~272 total (~81% compile) |
| Stdlib modules | 10 |
| Z80 emulator coverage | 100% (1335/1335 FUSE tests) |
| Peephole patterns | 35+ |
| Active backends | 4 (Z80, 6502, C, Crystal) |
| Parser | Participle (native Go, zero deps) |
| Toolchain binaries | 9 |

### What's Working
- Core language: types, functions, structs, enums, arrays, control flow
- Advanced features: lambdas, TSMC, CTIE, UFCS, operator overloading, string interpolation
- Iterator chains: map/filter/forEach/take/skip + lambdas via DJNZ
- Metafunctions: @define, @print, @if/@elif/@else, @error
- Multi-target: ZX Spectrum, CP/M, Agon Light 2
- Full toolchain: MZC, MZA, MZE, MZX, MZD, MZR, MZRUN

### Known Blockers
- Register allocator: overwrites operands in while/for loops (same phys reg for two live virtuals)
- `loadToHL` uses stale values in multi-expression contexts
- Loop rerolling too aggressive across function call boundaries
- Iterator enumerate/reduce need OpPush in Z80 backend

---

## Phase 1: Stability & Bug Fixes (Q1 2026)

**Goal**: Make core codegen 100% reliable for all working language features.

### Register Allocator
- [x] Liveness analysis: loop back-edge awareness (extends intervals across DJNZ/JR NZ back-edges)
- [x] Dead register freeing: registers now freed when live interval ends (was a no-op)
- [x] Fix `loadToHL` stale values: dynamic `currentRegister` tracking replaces static allocator trust
- [ ] Verify no remaining register conflicts in complex loop programs
- See ADR-0006 (address widening), ADR-0007 (newline handling)

### Iterator Chain Codegen
- [x] Wire fusion optimizer (`pkg/optimizer/fusion.go`) into pipeline as Pass
- [ ] Implement fusion detection logic (currently detection-only scaffold)
- [ ] Implement OpPush/OpPop in Z80 backend for enumerate/reduce
- [ ] HL clobber fix verified (PUSH/POP HL around CALL in DJNZ loops)
- [ ] Expand test coverage for chained operations
- Ref: [Iterator Implementation Status](Iterator_Implementation_Status.md)

### Loop Rerolling
- [x] CR/LF boundary detection: putchar sequences split at newline characters
- [ ] Verify fix with programs that mix text output and newline() calls

### Constant Tracking
- [x] Invalidate at every `OpLabel` (labels are merge points)
- [x] All arithmetic/logic ops invalidate destination register
- [ ] Verify no remaining stale-constant bugs in complex programs

---

## Phase 2: Infrastructure & Performance (Q2 2026)

**Goal**: Shared packages, optimization pipeline, backend consistency.

### MZE / MZX Shared Packages
Extract duplicated code between headless emulator and ZX Spectrum emulator:

- [ ] `pkg/profile/` — shared profiler (ExecCount/ReadCount/WriteCount heatmaps, IO maps, basic-block trace)
- [ ] `pkg/console/` — shared console I/O (port mapping, stdin reader goroutine)
- [ ] Shared diagnostics — DiagString/DumpState formatting
- [ ] Shared optimizer infrastructure — peephole patterns reusable across backends

### Backend Harmonization
- [ ] Migrate backends to use shared Backend Toolkit patterns
- [ ] Consistent instruction selection across Z80, 6502, C, Crystal
- [ ] Create backend feature matrix documentation
- Ref: [Backend Harmonization Plan](../minzc/pkg/codegen/BACKEND_HARMONIZATION_PLAN.md)

### Superoptimizer Pipeline
- [ ] Wire superoptimizer-proven rules (602K Z80 optimizations) into peephole pass
- [ ] Integrate with existing 35+ peephole patterns
- See ADR-0009

---

## Phase 3: Language Features (Q2-Q3 2026)

**Goal**: Complete partially-implemented language features.

### Pattern Matching
- [ ] Complete codegen for match/case statements
- [ ] Pattern guards
- [ ] Exhaustiveness checking
- [ ] Jump table optimization for enum matching

### Generator Syntax
- [ ] `gen`/`yield` keywords for lazy iteration
- [ ] Integration with iterator chain pipeline
- [ ] Stack frame management for suspended generators

### Array Literal Optimization
- [ ] Complete IR skeleton to codegen path
- [ ] Constant array folding at compile time
- [ ] ROM-friendly read-only array placement

### MIR Improvements
- [ ] Complete array/struct support in MIR interpreter
- [ ] Expand `@minz[[[...]]]` compile-time execution capabilities

---

## Phase 4: Platform Expansion (Q3 2026)

**Goal**: Complete Agon eZ80 support, evaluate stretch platforms.

### Agon Light 2 / eZ80 (~70% complete)
- [x] Target configuration, eZ80 instructions, cross-mode calls
- [x] MOS/VDP stdlib modules
- [ ] 24-bit type codegen (u24/i24 arithmetic, LEA usage)
- [ ] Fixed-point math types (f8.8, f16.8, f8.16)
- [ ] Audio stdlib (`stdlib/agon/audio.minz`)
- [ ] Register mapping for extern: `extern fun f(x in HL) at 0x10;`
- [ ] Test on real Agon hardware
- Ref: [Agon eZ80 Plan](Agon_eZ80_Plan.md)

### Stretch Goals
- 65816 (SNES) — evaluate based on community interest
- ARM (Raspberry Pi / GBA) — experimental
- RISC-V — future-proofing

---

## Phase 5: Developer Experience (Q4 2026)

**Goal**: Professional tooling for real-world development.

### LSP Server
- [ ] Autocomplete for types, functions, struct fields
- [ ] Go-to-definition
- [ ] Error diagnostics with file:line:col
- [ ] Hover information for types and functions

### DAP Debugger
- [ ] Source-level debugging via MZE
- [ ] Breakpoints, step execution
- [ ] Variable inspection
- [ ] Integration with VS Code

### WASM Playground
- [ ] Online MinZ to Z80 compilation demo
- [ ] Embedded emulator for instant feedback
- [ ] Shareable code links

### Error Message Quality
- [ ] Source context in error output (show surrounding lines)
- [ ] Suggestion system for common mistakes
- [ ] Type mismatch explanations with fix hints

---

## Success Criteria

### v1.0 Release Gate
- 95%+ compilation success rate across all examples
- All core language features stable (no "experimental" warnings)
- Comprehensive stdlib for all 3 platforms (Spectrum, CP/M, Agon)
- Complete language reference documentation
- LSP server with basic functionality

### Performance Targets
- Iterator chains perform identically to hand-written DJNZ loops
- Interface dispatch has zero runtime overhead (no vtables)
- Lambda functions compile to direct CALLs
- Competitive with hand-written Z80 assembly on benchmarks

### Quality Targets
- Zero regressions in existing working examples
- All 1335 FUSE Z80 emulator tests pass
- CI runs full test suite on every commit

---

## Risk Mitigation

### Technical Risks

| Risk | Mitigation | Fallback |
|------|-----------|----------|
| Register allocator complexity | Start with conservative allocation, iterate | Feature flags for new allocation strategies |
| Parser edge cases | Comprehensive corpus testing (18 iterator programs + growing) | Simplified syntax for complex features |
| Code generation bugs | Assembly-level validation via MZE emulator | Conservative code generation mode |
| Performance regression | Benchmark suite with CI integration | Feature flags for new optimizations |

### Project Risks

| Risk | Mitigation | Fallback |
|------|-----------|----------|
| Scope creep | Strict phase gates, prioritize stability over features | Feature freeze after Phase 3 |
| Backward compatibility | Semantic versioning from v1.0 | Legacy mode for deprecated syntax |
| Platform fragmentation | Shared backend toolkit, consistent test matrix | Focus on Z80 as primary, others as best-effort |

---

## References

### Active Technical Plans
- [Iterator Implementation Status](Iterator_Implementation_Status.md) — iterator chain pipeline details
- [Agon eZ80 Plan](Agon_eZ80_Plan.md) — Agon Light 2 platform support
- [Backend Harmonization Plan](../minzc/pkg/codegen/BACKEND_HARMONIZATION_PLAN.md) — multi-backend consistency
- [Native Parser Plan](NATIVE_PARSER_PLAN.md) — Participle parser technical reference (completed)

### Architecture Decision Records
- `docs/adr/ADR-0006` — Address widening
- `docs/adr/ADR-0007` — Newline handling
- `docs/adr/ADR-0009` — Superoptimizer-driven peephole rules

### Project Documentation
- [CLAUDE.md](../CLAUDE.md) — AI colleague instructions and project overview
- [INTERNAL_ARCHITECTURE.md](../minzc/docs/INTERNAL_ARCHITECTURE.md) — Compiler internals
- [COMPILER_SNAPSHOT.md](../COMPILER_SNAPSHOT.md) — Current state tracking

### Archived Plans
Historical plans that informed this document are in [`docs/_archive_plans/`](_archive_plans/).

---

*MinZ: Modern programming abstractions with zero-cost performance on vintage hardware.*
