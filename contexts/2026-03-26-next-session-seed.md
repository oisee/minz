# Next Session Seed

---

## Immediate: Check Results

```bash
ddll explore   # get session IDs

# 6v dense GPU — was it killed at 22% (537K sample) or did it continue?
ddll send <z80-optimizer>:main "6v status?"

# VIR reliability sprint — P1 (clobber), P6 (InlineTrivial labels)
ddll send <vir>:main "P1/P6 status? P6 fixes Pascal stdlib + ABAP seed SQL."
```

## Priority 1: @error Enforcement (Layer 2)

Layer 1 shipped: `@error(N)`, `@check`, `@propagate` as metafunctions (55 LOC).
Layer 2: parser enforcement of `?` naming convention.

```nanz
// ? in name = fallible
fun safe_div?(a: u8, b: u8) -> u8 { ... }

// Compiler ERROR if @check/@propagate missing after call to name?()
var x: u8 = safe_div?(10, y)
// NEXT LINE MUST BE @check or @propagate — otherwise compile error
@propagate
```

Implementation: ~50 LOC in parser. Check: after parsing `CALL name?()`,
verify next statement is `@check` or `@propagate`. Emit error if not.

## Priority 2: VIR P6 (InlineTrivial Label Fix)

When P6 lands, test:
```bash
# Pascal stdlib — should now work
mz examples/pascal/casetest.pas --target=cpm -o out.a80
mza out.a80 -o CASE.COM
echo "" | mze CASE.COM -t cpm
# Expected: "Beta" (CASE Ch of 66)

# Sieve of Eratosthenes
mz examples/pascal/sieve.pas --target=cpm -o out.a80
mza out.a80 -o SIEVE.COM
mze SIEVE.COM -t cpm
# Expected: prime count
```

## Priority 3: C89 Width Fix

When VIR fixes pkg/c89/ u16 promotion:
```bash
cat > /tmp/test.c << 'EOF'
unsigned char add8(unsigned char a, unsigned char b) { return a + b; }
/* assert add8(3, 4) == 7 via z80 */
EOF
mz /tmp/test.c --vir -o /tmp/test.a80
grep "^add8:" -A3 /tmp/test.a80
# Expected: ADD A, C / RET (2 inst, not 5)
```
Impact: Paper A signature count drops 315→~250.

## Priority 4: Paper A Final Draft

All data ready:
- 17.4M exhaustive ≤5v table
- 13.2M composition verification (5.06T overhead)
- Double phase transition (enumeration + feasibility)
- Cross-compiler: Nanz = SDCC for u8
- SDCC comparison table (swap 20:0)
- Treewidth: 99.5% random, 46.3% dense corpus
- 5-level pipeline: composition 80% + backtrack 15% + Z3 5% = 100%

Draft: `research/paper-a-draft.md`
Review: `ddll ask gpt54 -s paper-review @research/paper-a-draft.md "final review"`

## Priority 5: @error Examples + Tests

Add to test suite:
```bash
# Verify @error codegen
mz examples/nanz/14_error_propagation.nanz --lir=false -o /tmp/err.a80
grep "SCF" /tmp/err.a80     # should find SCF in safe_div
grep "RET C" /tmp/err.a80   # should find RET C in compute
```

Write more examples:
- `15_error_enum.nanz` — typed errors with enum
- I/O error handling (port read failure)
- CP/M BDOS error checking (file not found)

## Backlog

### Language
- [ ] @error `?` enforcement in parser (Layer 2)
- [ ] PL/M MOD operator in HIR lowerer
- [ ] Frill match expression desugaring
- [ ] Nanz import paths for .lanz/.lizp modules

### Backend (VIR)
- [ ] P1: cross-block clobber constraints → Tetris, ZSQL row counter
- [ ] P6: InlineTrivial asm-body label guard → Pascal stdlib, ABAP seeds
- [ ] C89 u16 promotion fix → Paper A signature count
- [ ] MZA instruction table expansion
- [ ] ZX Spectrum real SQLite (main() loop fix)

### Research
- [ ] Paper A final draft + GPT review
- [ ] Paper B prototype (DP partition)
- [ ] Paper C composition data on 6v
- [ ] ABI paper response to Philipp Krause
- [ ] V6 incremental or treewidth-filtered completion

### Demo
- [ ] ZX Spectrum real SQLite ALV (needs main() loop fix)
- [ ] Pascal CASE + Sieve on CP/M (needs VIR P6)
- [ ] LinkedIn post — ship with clean screenshots

## Key Files

| File | Purpose |
|------|---------|
| `docs/Error_Propagation_Design.md` | @error full design |
| `docs/Error_Propagation_Codegen.md` | What compiles to what |
| `examples/nanz/14_error_propagation.nanz` | @error examples |
| `research/README.md` | 4 papers overview |
| `research/paper-a/exhaustive-enumeration-strategy.md` | V4→V5→V6 |
| `research/paper-c-compositional-regalloc.md` | Treewidth decomposition |
| `docs/Anytime_Optimal_Register_Allocation.md` | 5-level pipeline |
| `docs/VIR_Reliability_Sprint.md` | VIR P1-P6 sprint |
| `contexts/2026-03-26-sprint-plan.md` | Frontend/backend task split |

## Session IDs (check with `ddll explore`)
- minz-vir: was jjjlhyva
- z80-optimizer: was um2dy4ex
- minz-abap: was fq7jsz_r
- GPT-5.4: `ddll ask gpt54 -s <session>`
- Gemini: `ddll ask gemini -s <session>`
