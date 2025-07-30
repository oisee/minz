# MinZ Script Update Summary

This document summarizes the updates made to scripts in the MinZ project to work with the new directory structure.

## Scripts Updated

### 1. **scripts/build/compile_all_examples.sh**
- **Changes:**
  - Updated compiler binary path from `./minzc/main` to `./minzc/minzc`
  - Removed `timeout` command usage (not available on macOS by default)
  - Fixed output directory path issue (removed double "examples/" in path)
- **Status:** ✅ Updated and tested

### 2. **scripts/build/compile_all_smc.sh**
- **Changes:**
  - Updated compiler binary path from `./minzc/main` to `./minzc/minzc`
  - Removed `timeout` command usage
  - Fixed output directory path issue
- **Status:** ✅ Updated

### 3. **minzc/compile_all_examples.sh**
- **Changes:**
  - Fixed malformed shebang (`#\!/bin/bash` → `#!/bin/bash`)
  - Removed erroneous `EOF < /dev/null` at end of file
- **Status:** ✅ Updated

### 4. **minzc/test_suite.sh**
- **Changes:** None needed - paths are correct
- **Status:** ✅ No changes required

### 5. **minzc/test_all.sh**
- **Changes:**
  - Added support for testing files in the `tests/` directory
  - Now tests both `examples/*.minz` and `tests/**/*.minz`
- **Status:** ✅ Updated

### 6. **minzc/test_all_examples.sh**
- **Changes:**
  - Fixed malformed shebang
  - Removed erroneous `EOF < /dev/null`
  - Added support for testing files in the `tests/` directory
- **Status:** ✅ Updated

### 7. **minzc/Makefile**
- **Changes:**
  - Updated `run` target from `../test_simple.minz` to `../examples/simple_add.minz`
- **Status:** ✅ Updated

### 8. **Python Scripts**
- **scripts/analysis/ast-to-json.py**: No changes needed (uses relative paths)
- **scripts/analysis/test_compilation.py**: No changes needed (paths are correct)
- **minzc/score_true_smc.py**: No changes needed (uses command-line arguments)
- **Status:** ✅ All Python scripts checked

## Directory Structure Reference

The MinZ project uses the following directory structure for test files:

1. **`/Users/alice/dev/minz/examples/`** - Contains 120+ example programs demonstrating MinZ features
2. **`/Users/alice/dev/minz/tests/`** - Contains 22+ test files for specific language features
   - `tests/debug/` - Debug-specific test files

## Notes

1. All scripts assume they are run from the root directory (`/Users/alice/dev/minz`)
2. The MinZ compiler binary is located at `minzc/minzc`
3. Compiled output goes to `examples/compiled/` when using the build scripts
4. The `timeout` command was removed from scripts as it's not available by default on macOS

## Scripts That May Need Manual Review

None identified - all scripts have been updated and should work correctly with the new directory structure.

## Testing Recommendation

To verify all scripts work correctly:

```bash
# From the root directory:
cd /Users/alice/dev/minz

# Test compilation scripts
bash scripts/build/compile_all_examples.sh
bash scripts/build/compile_all_smc.sh

# Test from minzc directory
cd minzc
make build
make test
make run
./test_all.sh
./test_all_examples.sh
```