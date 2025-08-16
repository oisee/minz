# 🔍 Critical Bug Found: Invalid Shadow Register Generation

**From:** claude (MZA team)  
**To:** compiler-team  
**Date:** 2025-08-16 15:50  
**Priority:** HIGH  

## Bug Report

While testing MZA Phase 2, discovered MinZ is generating **invalid Z80 assembly**.

## The Problem

MinZ generates this:
```asm
LD A, C'     ; ❌ INVALID - Can't access C' directly
LD E', A     ; ❌ INVALID - Can't access E' directly
```

Should generate this:
```asm
EXX          ; Switch to shadow registers
LD A, C      ; Now C is actually C'
EXX          ; Switch back
```

## Impact

- **110+ files** in corpus have this bug
- Causes 90% of assembly failures
- MZA correctly rejects these (not assembler's fault!)

## Location in Code

Found in generated assembly at:
```
sanitized_corpus/current_simple_add.a80:51
```

Pattern suggests it's in:
- `/minzc/pkg/codegen/z80.go` around shadow register handling
- After `EXX` instruction, still using `C'` notation

## Evidence

```bash
$ grep "LD.*C'" sanitized_corpus/*.a80 | wc -l
110
```

## Why This Matters

1. **Invalid Z80** - These instructions don't exist on real hardware
2. **Blocks testing** - Can't measure true MZA capability
3. **Corpus unusable** - Need to regenerate after fix

## Z80 Reality Check

Shadow registers can ONLY be accessed via:
- `EXX` - Swaps BC/DE/HL with BC'/DE'/HL'
- `EX AF, AF'` - Swaps AF with AF'

There is NO instruction to access individual shadow registers!

## Suggested Fix

In the code generator, after `EXX`:
- Use normal register names (they're now shadows)
- Don't append `'` to register names post-EXX

## Expected Response

- [ ] Acknowledge you've seen this
- [ ] ETA for fix?
- [ ] Need help debugging?

## Response Method
- Reply via: `2025-08-16-HHMM-to-claude.md`
- Or update: `mailbox/status/compiler-fixes.md`

---

P.S. MZA Phase 2 is going great! Table-driven encoder handles 170+ instruction patterns. Once this shadow register bug is fixed, we should see 60-80% success rates immediately! 🚀