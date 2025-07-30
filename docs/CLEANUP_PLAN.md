# MinZ Project Cleanup Plan

## Current State Analysis

The MinZ project root directory is cluttered with many files that should be organized into appropriate subdirectories. This plan outlines the reorganization strategy.

## Files to Keep in Root
These files should remain in the root directory:
- `README.md` - Main project documentation
- `CLAUDE.md` - Claude AI instructions
- `LICENSE` (if exists, should be created if not)
- `grammar.js` - Core tree-sitter grammar
- `binding.gyp` - Node.js binding configuration  
- `package.json` - NPM configuration
- `package-lock.json` - NPM lock file
- `Cargo.toml` - Rust configuration
- `Cargo.lock` - Rust lock file
- `.gitignore` (if exists)

## Reorganization Plan

### 1. Documentation Files → `docs/`
Move all documentation files currently in root:
- `CHANGELOG.md`
- `COMPILATION_RESULTS_*.md`
- `COMPILER_ARCHITECTURE.md`
- `DESIGN_NOTES.md`
- `FINAL_STATS_100.md`
- `HIGHLEVEL_PLAN.md`
- `IMMEDIATE_ACTIONS.md`
- `IMPLEMENTATION_STATUS.md`
- `INSTALLATION.md`
- `IR_GUIDE.md`
- `LUA_METAPROGRAMMING.md`
- `METAPROGRAMMING_DESIGN.md`
- `MINZ_DEEP_DESIGN_PHILOSOPHY.md`
- `MODERN_POINTER_DESIGN.md`
- `MODULE_SYSTEM.md`
- `OPTIMIZATION_GUIDE.md`
- `POINTER_PHILOSOPHY.md`
- `PROGRESS_REPORT.md`
- `RCA_ANALYSIS.md`
- `RELEASE_CHECKLIST.md`
- `RELEASE_NOTES_*.md`
- `SESSION_SUMMARY_*.md`
- `SMC_*.md`
- `TOOLING_AUDIT_REPORT.md`
- `TSMC_*.md`
- `Z80_*.md`
- `ZVDB_*.md`

### 2. Scripts → `scripts/`
Move all shell scripts and utility scripts:
- `compile_all_*.sh`
- `consolidate_examples.sh`
- `organize_smc_examples.sh`
- `ast-to-json.py`
- `check_opcode.go`
- `parse-to-json.js`
- `export-ast.js`
- `test-ast.js`
- `test-tree-sitter.js`
- `test_compilation.py`

### 3. Test Files → `tests/`
Move all test-related files from root:
- `test_*.minz` files
- `debug_*.minz` files
- `test/` directory contents (merge with tests/)

### 4. Build Artifacts → `build/` or cleanup
- `*.a80` files (compiled assembly)
- `*.mir` files (intermediate representation)
- `*.json` files (AST outputs)
- `*.xml` files
- `*.dylib` files
- `compilation_results_*.txt` files

### 5. Release Archives → `releases/`
Consolidate all release-related directories:
- `_release/`
- `release-v*/` directories  
- `minz-*.tar.gz` files
- `minzc-*.tar.gz` files

### 6. Example Files → Keep in `examples/`
- Move any example `.minz` files from root to `examples/`
- `mnist_*.minz` files
- `fibonacci_*.minz` files

### 7. Archive/Backup → `archive/`
- `examples_backup/` → `archive/examples_backup/`
- `minz-sdk-v*/` directories
- Old documentation that's been superseded

### 8. Cleanup Candidates (possibly delete)
These appear to be temporary or debug files:
- `parse_output.txt`
- `tree-sitter-output.txt`
- `sexp-output.txt`
- `parse.json`
- `cast_test.json`
- `ast*.json` files (unless needed)

### 9. Article/Blog → Keep in `article/`
The `article/` directory seems appropriately placed.

### 10. IDE Support → Keep in `vscode-minz/`
The VS Code extension directory is appropriately placed.

## New Directory Structure
```
minz-ts/
├── README.md
├── CLAUDE.md
├── LICENSE (create if missing)
├── grammar.js
├── binding.gyp
├── package.json
├── package-lock.json
├── Cargo.toml
├── Cargo.lock
├── .gitignore
├── docs/                 # All documentation
├── scripts/              # Build and utility scripts
├── tests/                # Test files and test data
├── examples/             # Example MinZ programs
├── releases/             # Release archives
├── archive/              # Old/backup files
├── build/                # Build artifacts (git-ignored)
├── src/                  # Generated parser source
├── bindings/             # Language bindings
├── stdlib/               # Standard library
├── minzc/                # Compiler implementation
├── node_modules/         # NPM dependencies
├── target/               # Rust build output
├── vscode-minz/          # VS Code extension
├── article/              # Blog/article content
├── queries/              # Tree-sitter queries
└── rca/                  # Root cause analyses
```

## Implementation Steps

1. **Create new directories**:
   ```bash
   mkdir -p docs/release-notes
   mkdir -p scripts/build
   mkdir -p tests/integration
   mkdir -p releases/v0.2.0 releases/v0.3.0 releases/v0.4.0
   mkdir -p archive/old-docs
   ```

2. **Move documentation files** to `docs/`

3. **Move scripts** to `scripts/`

4. **Move test files** to `tests/`

5. **Clean up or move build artifacts**

6. **Consolidate release files** to `releases/`

7. **Move example files** from root to `examples/`

8. **Archive old/backup directories**

9. **Delete temporary files** after confirmation

10. **Update any references** in README.md or other docs

## Duplicate Files Analysis

### Confirmed Duplicates
1. **examples_backup/** - Appears to be an exact copy of examples/
   - Action: Archive to `archive/examples_backup/`
   
2. **minz-sdk-v0.2.0/** - Contains copies of docs and examples from v0.2.0
   - Action: Archive to `archive/minz-sdk-v0.2.0/`

3. **Multiple release directories**:
   - `release-v0.2.0/` 
   - `release-v0.3.2/`
   - `minzc/release-v0.4.0/` (should be moved to root releases/)
   - Action: Consolidate under `releases/`

### Temporary/Output Files to Delete
- AST output files: `ast*.json`, `cast_test.json`, `parse.json`
- Parser output: `parse_output.txt`, `tree-sitter-output.txt`, `sexp-output.txt`
- Compilation results: `compilation_results_100.txt`
- Build artifacts in root: `*.a80`, `*.mir` files (14 total)

### Files Needing Special Attention
- **mnist_*.minz files in root** (multiple versions):
  - `mnist_complete.minz`
  - `mnist_minimal.minz`
  - `mnist_simple.minz`
  - `mnist_editor_*.minz` (multiple variants)
  - `mnist_test*.minz`
  - Action: Review and consolidate best versions to examples/mnist/

## File Count Summary
- **Documentation files (.md)**: 36 files to move to docs/
- **Test files (.minz)**: 41 files to move to tests/
- **Shell scripts (.sh)**: 5 files to move to scripts/
- **Build artifacts**: 14 files to clean up
- **Python/JS scripts**: ~8 files to move to scripts/

## Notes
- All moves should preserve git history
- Update .gitignore to exclude build artifacts in new locations
- Verify no build scripts break after reorganization
- Consider creating a CONTRIBUTING.md with the new structure
- Create a LICENSE file (currently missing)