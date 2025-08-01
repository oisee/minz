# MinZ Project Cleanup Summary

## Current Issues Found

### 1. Root Directory Clutter
- **36 documentation files** (.md) scattered in root
- **41 test files** (.minz) in root directory  
- **14 build artifacts** (.a80, .mir) in root
- **5 shell scripts** in root
- **8+ utility scripts** (Python, JS) in root

### 2. Duplicate Content
- `examples_backup/` - Complete duplicate of `examples/`
- `minz-sdk-v0.2.0/` - Contains old copies of docs and examples
- Multiple MNIST example versions with unclear differences

### 3. Inconsistent Release Structure
- Release directories scattered in multiple locations:
  - `release-v0.2.0/` in root
  - `release-v0.3.2/` in root  
  - `minzc/release-v0.4.0/` nested in compiler directory
  - `_release/` with mixed content
  - Loose release archives in root

### 4. Missing Essential Files
- No LICENSE file (critical for open source project)
- No CONTRIBUTING.md guide

### 5. Temporary/Debug Files
- Multiple AST output JSON files
- Parser debug output text files
- Compilation result logs
- These should be git-ignored and removed

## Recommended Actions

### Immediate (High Priority)
1. Create LICENSE file
2. Move all documentation to `docs/` with proper subdirectories
3. Create proper `releases/` structure

### Short Term (Medium Priority)  
1. Move all test files to `tests/`
2. Move all scripts to `scripts/`
3. Consolidate release directories
4. Archive duplicate content

### Cleanup (Low Priority)
1. Remove temporary output files
2. Clean up build artifacts
3. Review and consolidate MNIST examples

## Benefits After Cleanup

1. **Cleaner root directory** - Only essential files (README, LICENSE, configs)
2. **Better organization** - Clear separation of docs, tests, scripts, examples
3. **Easier navigation** - Developers can find files quickly
4. **Professional appearance** - Well-organized structure
5. **Reduced confusion** - No duplicate files or unclear versions

## Root Directory After Cleanup
Should only contain:
- README.md
- CLAUDE.md  
- LICENSE
- grammar.js
- binding.gyp
- package.json / package-lock.json
- Cargo.toml / Cargo.lock
- .gitignore
- Essential directories (docs/, examples/, minzc/, etc.)