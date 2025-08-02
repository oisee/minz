---
name: Assembly Pattern Issue
about: Automatically detected suspicious assembly pattern
title: '[AUTO] Suspicious Pattern: {PATTERN_NAME}'
labels: bug, assembly, automated
assignees: ''

---

## Suspicious Pattern Detected

**Pattern Type:** {PATTERN_NAME}  
**Severity:** {SEVERITY}  
**Detection Date:** {DATE}  
**Compiler Version:** {VERSION}

## Details

**File:** `{FILE}:{LINE}`

### Assembly Context
```asm
{CONTEXT}
```

### Pattern Description
{DESCRIPTION}

### Suggested Fix
{SUGGESTION}

## Additional Information

This issue was automatically detected by the MinZ assembly analyzer. The pattern may indicate:
- A compiler bug
- A missed optimization opportunity  
- An inefficient code generation pattern

### Example Fix

For parameter overwrite bugs:
```diff
- LD A, 0    ; First parameter
- LD A, 0    ; Second parameter (overwrites!)
+ LD A, 0    ; First parameter  
+ LD B, 0    ; Second parameter (different register)
```

---
*This issue was automatically generated. Please verify before implementing fixes.*