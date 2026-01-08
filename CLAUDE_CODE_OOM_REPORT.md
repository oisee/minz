# Claude Code OOM Report for MinZ Repository

**Date:** 2026-01-08
**Issue:** Claude Code's `tree-sitter` process causes server OOM (Out of Memory) when working with this repository.

## Problem Summary

When Claude Code indexes the MinZ repository, the `tree-sitter` process consumes **60+ GB of RAM**, causing the server to freeze and requiring hard reboot.

### OOM Events Logged:
```
Jan 07 23:05:05 Out of memory: Killed process (tree-sitter) anon-rss:57254436kB (~55GB)
Jan 07 23:39:44 Out of memory: Killed process (tree-sitter) anon-rss:63343652kB (~60GB)
Jan 08 00:07:39 Out of memory: Killed process (tree-sitter) anon-rss:64026056kB (~61GB)
Jan 08 16:21:41 Out of memory: Killed process (tree-sitter) anon-rss:64103840kB (~61GB)
```

## Repository Statistics

| Metric | Value |
|--------|-------|
| Total size | 2.2 GB |
| Total files | ~13,900 |
| .git directory | 744 MB (pack files) |
| .minz source files | 2,406 files (4 MB) |
| Release binaries | ~500 MB across multiple directories |

## Root Causes

### 1. Large .git Pack Files (744 MB)
The `.git/objects/pack/` directory contains large pack files that tree-sitter may attempt to parse:
- `pack-0ecd03d769d4acc949c54e51aa4732a869807608.pack` (in _zxspeculator/.git)
- `pack-0556aaee4f325388a52b262e49dc489d27987dd9.pack`
- `pack-269a7dd57fe1a286d0ddc0370d195695e0035c21.pack`

### 2. Nested Git Repository
There's a nested `.git` directory inside `_zxspeculator/` that may not be properly ignored.

### 3. Many Binary Release Directories
Multiple release directories with compiled binaries:
```
minzc/release-v0.10.0/
minzc/release-v0.10.1/
minzc/release-v0.12.0/
minzc/release-v0.13.0/
minzc/release-v0.13.1/
minzc/release-v0.13.2/
minzc/release-v0.14.0/
minzc/release-v0.14.1/
minzc/release-v0.15.0/
minzc/releases/
minzc/archive/
releases/
_archive/
```

### 4. Root Binary Executables
Large compiled binaries in repository root:
- `mz`, `mza`, `mze`, `mzrun`, `mztap`, `mz_pgo`, `mzr-simple`
- `minzc/main`, `minzc/mz`, `minzc/minzc`

### 5. Generated Parser Files
- `src/parser.c` (1.6 MB) - tree-sitter generated
- `minzc/mz-grammar/src/parser.c` (1.6 MB)

### 6. Many Test Output Files
- 123 `.stdout` files
- 123 `.stderr` files
- 44 `.err` files

## Solution Applied

A `.claudeignore` file has been created/updated with patterns to exclude problematic files:

```
# Git internals (CRITICAL - prevents OOM)
.git/
**/.git/
_zxspeculator/.git/
.git/objects/
**/.git/objects/

# Binary releases
releases/
minzc/releases/
minzc/release/
minzc/release-*/
minzc/release_*/
minzc/archive/
_archive/
_zxspeculator/

# Root binary executables
mz
mza
mze
mzrun
mztap
mz_pgo
mzr-simple
minzc/main
minzc/mz
minzc/mz_pgo
minzc/minzc

# Build artifacts
*.exe
*.a
*.so
*.dylib
*.o
*.ir
*.wasm
*.bin
*.pack
*.idx

# Generated parsers
**/parser.c
minzc/mz-grammar/

# Test outputs
tests/**/stdout
tests/**/stderr
tests/**/*.stdout
tests/**/*.stderr
tests/**/*.err

# Archives
*.zip
*.tar.gz
*.tgz
*.tap
*.tzx
*.sna
*.z80
```

## Recommendations for Repository Maintainers

### Short-term
1. **Commit the `.claudeignore` file** to the repository so other Claude Code users don't experience OOM
2. **Add `.claudeignore` to `.gitattributes`** with `linguist-generated=true` if desired

### Medium-term
1. **Move release binaries to Git LFS** or a separate releases repository
2. **Consider using `.gitignore`** to prevent committing compiled binaries to the repo
3. **Remove or archive old release directories** that are no longer needed
4. **Clean up nested git repositories** (like `_zxspeculator/.git`)

### Long-term
1. **Restructure the repository** to separate source code from build artifacts
2. **Use GitHub Releases** for distributing binaries instead of storing them in the repo
3. **Consider splitting** the monorepo if it continues to grow

## File Type Distribution (excluding ignored)

After applying ignore patterns, the relevant source files are:
```
795  .minz files
679  .md documentation
231  .go source
 33  .c source (excluding parser.c)
 24  .lua scripts
 21  .json configs
 16  .py scripts
  9  .js scripts
```

This is a reasonable codebase size that Claude Code should handle without issues.

## Suggestions for Claude Code / Tree-sitter Improvements

### Memory Management Issues

1. **No memory limit enforcement**
   - tree-sitter should have a configurable memory ceiling (e.g., 8GB max)
   - Process should gracefully fail rather than consume all system RAM

2. **Missing file type detection**
   - Binary files (ELF executables, .pack files) should be auto-detected and skipped
   - Magic number checking before attempting to parse

3. **Git directory handling**
   - `.git/` directories should be hardcoded as always-ignored
   - Git pack files are never valid source code

4. **Streaming/chunked parsing**
   - Large files should be parsed in chunks rather than loading entirely into memory
   - Implement early termination if file doesn't match any grammar

### Ignore Pattern Improvements

5. **Better glob pattern support**
   - Pattern `release*/` should match `release-v0.10.0/` (include hyphen)
   - Support for `**/` recursive patterns should be more reliable

6. **Default ignore patterns**
   - Ship with sensible defaults: `.git/`, `node_modules/`, `*.exe`, `*.so`, etc.
   - Similar to how ripgrep ignores common patterns by default

### Monitoring/Feedback

7. **Progress reporting**
   - Show which file is being parsed when memory usage is high
   - Allow user to identify problematic files

8. **Memory usage warnings**
   - Warn user when memory usage exceeds 50% of system RAM
   - Suggest adding patterns to `.claudeignore`

### Potential Bug Report

The fact that tree-sitter consumes 60GB+ RAM when parsing a 2.2GB repository (with 744MB being .git) suggests either:
- A memory leak in the parser
- Exponential complexity on certain file types
- Failure to respect ignore patterns for `.git/` directories

**Reproduction steps:**
1. Clone a repository with ~14,000 files including large .git pack files
2. Run Claude Code without `.claudeignore`
3. Observe tree-sitter consuming 60+ GB RAM

---

## Contact

Report generated by Claude Code diagnostic session.
Server: u7 (Ubuntu, 62GB RAM)
