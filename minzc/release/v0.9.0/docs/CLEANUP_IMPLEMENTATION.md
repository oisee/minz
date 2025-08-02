# MinZ Project Cleanup Implementation Guide

## Phase 1: Create Directory Structure

```bash
# Create new directories
mkdir -p docs/release-notes
mkdir -p docs/design
mkdir -p docs/technical
mkdir -p scripts/build
mkdir -p scripts/analysis
mkdir -p tests/integration
mkdir -p tests/debug
mkdir -p releases/v0.2.0
mkdir -p releases/v0.3.0  
mkdir -p releases/v0.4.0
mkdir -p archive/old-docs
mkdir -p archive/backups
mkdir -p archive/temp-files
```

## Phase 2: Move Documentation Files

```bash
# Move release notes
mv RELEASE_NOTES_v*.md docs/release-notes/
mv CHANGELOG.md docs/

# Move design documents
mv DESIGN_NOTES.md MINZ_DEEP_DESIGN_PHILOSOPHY.md POINTER_PHILOSOPHY.md docs/design/
mv MODERN_POINTER_DESIGN.md METAPROGRAMMING_DESIGN.md docs/design/

# Move technical documentation
mv COMPILER_ARCHITECTURE.md IR_GUIDE.md OPTIMIZATION_GUIDE.md docs/technical/
mv LUA_METAPROGRAMMING.md MODULE_SYSTEM.md SMC_*.md docs/technical/
mv Z80_*.md ZVDB_*.md docs/technical/

# Move implementation/progress docs
mv IMPLEMENTATION_STATUS.md PROGRESS_REPORT.md SESSION_SUMMARY_*.md docs/
mv COMPILATION_RESULTS_*.md FINAL_STATS_100.md docs/

# Move planning docs
mv HIGHLEVEL_PLAN.md IMMEDIATE_ACTIONS.md TOOLING_AUDIT_REPORT.md docs/
mv RCA_ANALYSIS.md RELEASE_CHECKLIST.md docs/
mv TSMC_*.md docs/technical/
```

## Phase 3: Move Scripts

```bash
# Move shell scripts
mv compile_all_*.sh scripts/build/
mv consolidate_examples.sh organize_smc_examples.sh scripts/

# Move utility scripts
mv ast-to-json.py parse-to-json.js export-ast.js scripts/analysis/
mv test-ast.js test-tree-sitter.js scripts/analysis/
mv test_compilation.py scripts/analysis/
mv check_opcode.go scripts/analysis/
```

## Phase 4: Move Test Files

```bash
# Move debug test files
mv debug_*.minz tests/debug/

# Move test files
mv test_*.minz tests/
mv fibonacci_recursive.minz tests/

# Special test files that might be examples
# Review these before moving:
# - mnist_*.minz (multiple versions - need consolidation)
```

## Phase 5: Clean Build Artifacts

```bash
# Create a build artifacts directory (if keeping any)
mkdir -p build/artifacts

# Move or delete build outputs
rm *.a80 *.mir  # OR: mv *.a80 *.mir build/artifacts/

# Delete temporary output files
rm ast*.json cast_test.json parse.json
rm parse_output.txt tree-sitter-output.txt sexp-output.txt
rm compilation_results_*.txt
```

## Phase 6: Consolidate Releases

```bash
# Move release directories
mv release-v0.2.0/* releases/v0.2.0/
mv release-v0.3.2/* releases/v0.3.0/  # Note: v0.3.2 -> v0.3.0 dir
mv minzc/release-v0.4.0/* releases/v0.4.0/

# Move loose release files
mv _release/* releases/
mv minz-*.tar.gz releases/
mv minzc-*.tar.gz releases/

# Clean up empty directories
rmdir release-v0.2.0 release-v0.3.2 _release
rmdir minzc/release-v0.4.0
```

## Phase 7: Archive Old/Duplicate Content

```bash
# Archive duplicate directories
mv examples_backup archive/
mv minz-sdk-v0.2.0 archive/

# Archive old release files if needed
# mv releases/old-releases archive/
```

## Phase 8: Handle MNIST Files

```bash
# Review and consolidate MNIST examples
# First, check which versions are most recent/complete:
ls -la mnist_*.minz

# Then move the best versions to examples:
# mv mnist_complete.minz examples/mnist/
# mv mnist_simple.minz examples/mnist/
# Archive or delete redundant versions
```

## Phase 9: Final Cleanup

```bash
# Update .gitignore if needed
echo "# Additional ignores for new structure" >> .gitignore
echo "build/artifacts/" >> .gitignore
echo "archive/temp-files/" >> .gitignore

# Create LICENSE file
cat > LICENSE << 'EOF'
MIT License

Copyright (c) 2024 MinZ Project Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
EOF

# Verify structure
tree -L 2 -d
```

## Verification Steps

1. Check that build scripts still work:
   ```bash
   cd minzc && make build
   ```

2. Verify tree-sitter generation:
   ```bash
   tree-sitter generate
   ```

3. Check examples can be compiled:
   ```bash
   cd minzc && ./minzc ../examples/fibonacci.minz
   ```

## Rollback Plan

Before starting, create a backup branch:
```bash
git checkout -b pre-cleanup-backup
git checkout main  # or master
```

If issues arise, you can always revert:
```bash
git checkout pre-cleanup-backup
```