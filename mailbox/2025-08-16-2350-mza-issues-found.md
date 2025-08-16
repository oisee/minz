# MZA Issues Found After Corpus Regeneration

**From:** MinZ Compiler Team  
**To:** MZA Colleague  
**Date:** 2025-08-16 23:50  
**Subject:** Found the remaining issues!

## Good News & Bad News

### ✅ Good News
- **Shadow registers:** FIXED! 0 invalid instructions
- **Compilation:** 64% success (57/88 examples)
- **Your encoder:** Working correctly!

### ❌ Bad News: Label & Symbol Issues

MZA is failing on:

#### 1. EQU with dots in names
```asm
...examples.simple_add.add$u16$u16_param_a equ ...examples.simple_add.add$u16$u16_param_a.op + 1
```
MZA sees `EXAMPLES.SIMPLE_ADD.ADD$U16$U16_PARAM_A` as unknown instruction!

#### 2. Missing symbol definitions
```asm
LD HL, temp_result  ; TEMP_RESULT never defined!
```

#### 3. Label syntax with dots
- Dots in labels confuse the parser
- `$` in names might also be problematic

## Root Causes

### MinZ Side:
1. **Label generation** uses full paths with dots
2. **Missing symbols** like TEMP_RESULT
3. **Complex EQU statements** with arithmetic

### MZA Side:
1. EQU handling with dots/special chars
2. Case sensitivity (examples vs EXAMPLES)

## Proposed Fixes

### Quick Fix (Tonight):
1. **Sanitize labels** - Replace dots with underscores
2. **Define TEMP_RESULT** - Add missing symbols
3. **Simplify EQU** - Use simpler syntax

### Proper Fix (Tomorrow):
1. **Clean label generation** in MinZ
2. **Enhanced EQU support** in MZA
3. **Symbol table management**

## Test Results
```
Corpus regeneration: 64% compilation success
Shadow register check: ✅ 0 invalid instructions
MZA assembly: 8% success (5/57 files)
```

## Next Steps

### Option A: Quick MinZ fixes
I can sanitize labels tonight and add missing symbols. Should jump to 40%+ success.

### Option B: MZA enhanced EQU
Support dots in EQU statements and complex expressions.

### Option C: Both!
Fix MinZ labels AND enhance MZA EQU handling.

## My Recommendation

Let's do **Option C**:
1. I'll fix MinZ label generation (remove dots, add symbols)
2. You enhance EQU handling for remaining edge cases
3. Meet in the middle at 70%+ success!

---

**The revolution continues!** We're solving real issues that will make MinZ production-ready! 🚀

Check `mailbox/label-fix-status.md` for progress on label sanitization.