# MinZ Cleanup Plan - February 2026

## Archive Branch
`archive/pre-cleanup-2026-02` at commit `11600f9`

---

## Phase 1: Remove Unused Infrastructure [DONE]

### Tree-sitter/Node.js (DELETED)
- [x] `grammar.js` - Tree-sitter grammar
- [x] `Cargo.toml` - Rust bindings
- [x] `package.json`, `package-lock.json` - Node.js
- [x] `binding.gyp` - Node bindings
- [x] `src/` - Generated parser.c (1.8 MB)
- [x] `bindings/` - Language bindings
- [x] `queries/` - Tree-sitter queries

### ANTLR (DELETED)
- [x] `minzc/grammar/MinZ.g4`
- [x] `minzc/mz-grammar/`
- [x] `minzc/Makefile.antlr`
- [x] `minzc/test_antlr_all.sh`
- [x] `minzc/pkg/parser/antlr_parser.go`
- [x] `minzc/pkg/parser/generated/`
- [x] `minzc/pkg/parser/minzparser/`

### Unused Parsers (DELETED)
- [x] `minzc/pkg/parser/sexp_parser.go`
- [x] `minzc/pkg/parser/simple_parser.go`
- [x] `minzc/pkg/parser/native_parser.go.disabled`
- [x] Multiple `grammar.js` copies

### Large Archives (DELETED)
- [x] `_archive/` - 146 MB old files
- [x] `releases/` - 101 MB old releases (v0.2-v0.4)
- [x] `_zxspeculator/` - 63 MB separate repo

**Space Freed: ~315 MB**

---

## Phase 2: Clean minzc/ Directory [IN PROGRESS]

### Build Artifacts to Remove
- [ ] `minzc/*.a80` - Compiled outputs (should be in build/)
- [ ] `minzc/*.mir` - MIR outputs
- [ ] `minzc/archive/` - Internal archive
- [ ] Old binaries: `ast-compare`, `ast-gen`, `backend-devkit`

### Keep
- [x] `minzc/pkg/parser/participle/` - Active parser
- [x] `minzc/pkg/parser/parser.go` - Parser interface
- [x] `minzc/cmd/` - CLI tools
- [x] `minzc/pkg/` - Core compiler

---

## Phase 3: Curate Documentation

### docs/ (343 files → ~50)
- [ ] Archive docs older than 2026 to `docs/_archive/`
- [ ] Keep: ADRs, specs, current roadmaps
- [ ] Remove: Old progress reports, obsolete designs

### Top-Level Markdown
- [ ] Consolidate: ROADMAP.md, STABILITY_ROADMAP.md, PLAN.md
- [ ] Review: All top-level .md files

---

## Phase 4: Curate Examples

### Current Status: 59/69 compile

### Keep (Working)
- Core demos for 3 platforms
- Demoscene examples

### Archive (Broken/Aspirational)
- [ ] `examples/archive/`
- [ ] `examples/aspirational/`
- [ ] `examples/experimental/`
- [ ] 10 failing examples

---

## Phase 5: Tool Binaries

### Remove Old Binaries
- [ ] `mz_pgo` - Superseded
- [ ] `mzr-simple` - Old REPL
- [ ] Evaluate: `mz`, `mza`, `mze`, `mzrun`, `mztap`

---

## Post-Cleanup Focus

### 3 Platforms
1. ZX Spectrum (classic demoscene)
2. Agon Light 2 (modern retro, eZ80)
3. CP/M (vintage computing)

### Features
- Graphics primitives
- Sound/beeper
- Fast math (sin/cos tables)
- Demoscene effects

### Stdlib Priority
- `math/fast.minz` - Lookup tables
- `graphics/screen.minz` - Drawing
- `sound/beep.minz` - Audio
- `input/keyboard.minz` - Input
