# Parser Analysis Report - January 2026

## Executive Summary

**Tree-sitter is NOT holding us back.** The parser works correctly on 100% of examples. All failures are in semantic analysis.

## Failure Analysis

### Current Stats
- **Total examples:** 53
- **Parse success:** 53 (100%)
- **Compile success:** 49 (92.4%)
- **Semantic failures:** 4 (7.6%)

### Failure Breakdown

| Example | Error | Category |
|---------|-------|----------|
| cpm_hello.minz | undefined function: putchar | Missing stdlib |
| loops_indexed.minz | undefined type: idx | Type declaration |
| nested_loops.minz | cannot dereference non-pointer | Pointer semantics |
| pointer_arithmetic.minz | cannot dereference non-pointer | Pointer semantics |

### Root Causes

1. **Pointer Type Tracking (2 failures)**
   - Semantic analyzer loses pointer level during operations
   - `*ptr` on u16 should work if ptr was declared as pointer
   - Need to track pointer depth in type system

2. **Missing Standard Library (1 failure)**
   - `putchar` not defined
   - CP/M examples need BDOS function wrappers

3. **Type Declaration Handling (1 failure)**
   - Custom type `idx` not recognized
   - May be alias or typedef issue

## Parser Comparison

### Tree-sitter (Current) ✅ RECOMMENDED

| Aspect | Rating | Notes |
|--------|--------|-------|
| Parse accuracy | 100% | All examples parse correctly |
| Incremental parsing | ⭐⭐⭐ | Perfect for LSP real-time feedback |
| Error recovery | ⭐⭐⭐ | Continues with partial parse |
| Syntax highlighting | ⭐⭐⭐ | Built-in, free |
| Maintenance | ⭐⭐ | Grammar in JavaScript |
| Go integration | ⭐⭐ | Works via CGO bindings |

### ANTLR ❌ NOT RECOMMENDED

| Aspect | Rating | Notes |
|--------|--------|-------|
| Parse accuracy | 5%* | Regression from 75% |
| Incremental parsing | ⭐ | Not designed for this |
| Error recovery | ⭐⭐ | Good but not as flexible |
| Syntax highlighting | ⭐ | Separate tool needed |
| Maintenance | ⭐⭐⭐ | Clean grammar notation |
| Go integration | ⭐⭐ | antlr4-go runtime |

*ANTLR was attempted and regressed badly. Root cause unknown but likely grammar/action translation issues.

### Native Go Parser ⚠️ FUTURE OPTION

| Aspect | Rating | Notes |
|--------|--------|-------|
| Parse accuracy | N/A | Would need implementation |
| Incremental parsing | ⭐ | Would need custom implementation |
| Error recovery | ⭐ | Manual implementation required |
| Syntax highlighting | ⭐ | Separate implementation |
| Maintenance | ⭐⭐⭐ | Full control, pure Go |
| Go integration | ⭐⭐⭐ | Native, no dependencies |

## Recommendations

### Immediate (Q1 2026)

1. **Keep Tree-sitter** - It's working, don't fix what isn't broken
2. **Fix pointer semantics** - 2 examples will pass
3. **Add stdlib functions** - 1 example will pass
4. **Fix type aliases** - 1 example will pass

### For LSP Server

Tree-sitter is **ideal** for LSP because:
- Incremental parsing = fast response on every keystroke
- Error recovery = useful feedback even with broken code
- Already integrated = no new parser needed

### Architecture

```
                    ┌─────────────────────┐
                    │   LSP Server        │
                    │   (real-time)       │
                    └──────────┬──────────┘
                               │
              ┌────────────────┴────────────────┐
              ▼                                 ▼
    ┌─────────────────────┐         ┌─────────────────────┐
    │   Tree-sitter       │         │   Full Compiler     │
    │   (incremental)     │         │   (batch)           │
    │                     │         │                     │
    │   • Syntax errors   │         │   • Tree-sitter     │
    │   • Highlighting    │         │   • Semantic check  │
    │   • Quick feedback  │         │   • IR generation   │
    │                     │         │   • Optimization    │
    │                     │         │   • Code gen        │
    └─────────────────────┘         └─────────────────────┘
```

## Action Items

### High Priority (This Month)

- [ ] Fix pointer dereference in semantic analyzer
- [ ] Add `putchar` to stdlib
- [ ] Handle type aliases properly
- [ ] Reach 96%+ compilation success

### Medium Priority (Q1 2026)

- [ ] Add line numbers to all error messages
- [ ] Implement LSP server using tree-sitter
- [ ] Add source location tracking throughout pipeline

### Low Priority (Future)

- [ ] Consider native Go parser only if tree-sitter becomes unmaintainable
- [ ] ANTLR should remain parked unless someone investigates the regression

## Conclusion

The parser is not the problem. Focus on:
1. Semantic analyzer fixes
2. LSP server implementation
3. Better error messages

Tree-sitter gives us the best of both worlds: fast incremental parsing for IDE features, and full parse accuracy for compilation.
