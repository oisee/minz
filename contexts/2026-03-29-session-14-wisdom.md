# Session Wisdom: Session 14 (2026-03-28/29)

**Commits:** ~15 across main repo + VIR
**Highlights:** fib(7)=13 on Z80, Tetris on CP/M, Lizp→Scheme, branchless CMOV

---

## Breakthroughs

### 1. fib(7)=13 on Z80!
Three layers of fixes:
- callerSavePairs: use g.loc() not ar.Locs (sees physOverride runtime relocations)
- Self-recursive: ALL GPR marked clobbered (scratch regs not in static set)
- Island decomposition: skip for self-recursive → PBQP fallback

### 2. Hello Frill! fib(7)=13 gcd(12,8)=4 on CP/M
Working binary via PBQP path. putch() via inline asm (LD E,A; LD C,2; CALL 5).

### 3. Tetris on CP/M
tetris_cpm.nanz: import tui.render (stdlib BDOS VT100 implementation).
Renders field, game over screen through Z80 emulator.

### 4. Bool Convention GPU-Proven
Z→A branchless IMPOSSIBLE (Z flag write-only, proven 456K sequences).
CY→A = SBC A,A (1 instr, 4T). CMOV = 6 instr, 24T branchless select.
Bool representation: 0xFF/0x00 (boolean ops = FREE: AND/OR/XOR/CPL native).

### 5. Branchless Primitives Library (z80-optimizer)
- ABS: 6i 24T (sign-extend mask trick)
- MIN/MAX: 8i 32T (SBC A,A conditional select)
- CMOV CY?B:C: 6i 24T (bitwise select)
- div3 EXACT: A×171>>9 (256/256 verified)
- Conditional ADD: CY?(A+B):A = 3i 12T

### 6. Lizp → Scheme R5RS
Added: define, lambda, even?, odd?, zero?, null?, positive?, negative?,
not, min, max, abs. 22 new asserts in scheme_test.lizp.

### 7. ABAP FUNCTION/ENDFUNCTION
Preprocessor rewrites FUNCTION→FORM before abaplint. 7 asserts.

### 8. Assert Improvements
- --asserts mir2|z80|none|all CLI flag
- --asserts-force mir2 (override all 'via' annotations)
- assert func() (implies == true for bool)
- assert not func() (implies == false)
- assert func() == true/false literals

---

## Known Bugs

### VIR CALL Arg Setup — BLOCKER
VIR does not emit LD moves for CALL arguments (constant or cross-register).
Affects: putchar(72), parse_digit→is_digit, any CALL with args through VIR.
Workaround: --vir=false
Priority 1 for next session.

### VIR Island Decomposition for Recursive
Skip added (self-recursive → PBQP fallback). Proper fix: emit correct
multi-block code for recursive functions in VIR.

---

## Assert Corpus: ~1700+

| Language | Asserts | Status |
|----------|---------|--------|
| Frill | 436 | ✅ |
| C89 | 353 | ✅ |
| C99+ | 305 | ✅ |
| ObjC | 98 | ✅ |
| Pascal | 54 | ✅ |
| Nanz | 371+ | ✅ |
| ABAP | 31 | ✅ |
| Lizp | 31 | ✅ |
| PL/M | 9 | ✅ |
