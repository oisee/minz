# Next Session Seed

---

## Immediate: Check Overnight Results

```bash
# 6v dense GPU (66M shapes, was running overnight)
ddll send <z80-optimizer-id>:main "Did 6v dense finish?"

# VIR reliability sprint progress
ddll send <vir-id>:main "P6 (InlineTrivial asm-body labels) status?"
```

## Priority 1: VIR Reliability Sprint Results

VIR committed a reliability sprint plan (docs/VIR_Reliability_Sprint.md):
- P1: cross-block clobber constraints
- P5: edge-move emission for call-safe vregs
- P6: asm-body functions must NOT be inlined (ClobberAll guard)

When P6 lands → Pascal stdlib works on CP/M (casetest.pas, sieve.pas).
When P1+P5 land → Nanz Tetris variants work. ZSQL row counter works.

## Priority 2: C89 Width Promotion Fix

VIR checking pkg/c89/ type rules. `add8(u8,u8)→u8` should be `ADD A,C / RET` (2 inst) not 5 inst. Paper A signature count drops 315→~250 when fixed.

Test after fix:
```bash
cat > /tmp/test_width.c << 'EOF'
unsigned char add8(unsigned char a, unsigned char b) { return a + b; }
/* assert add8(3, 4) == 7 via z80 */
EOF
mz /tmp/test_width.c --vir -o /tmp/test_width.a80
grep "^add8:" -A3 /tmp/test_width.a80
# Expected: ADD A, C / RET (not LD L,A / LD H,0 / ADD HL,BC / LD A,L / RET)
```

## Priority 3: 6v Data Analysis

From z80-optimizer:
- 6v feasibility: only 0.9% feasible (double phase transition)
- 0.9% × 1.9B = ~17M feasible entries (same size as ≤5v table)
- Treewidth of 54 dense corpus functions: 46.3% tw≤3, 35.2% tw=4, 18.5% tw≥5
- Composition verification: 13.2M shapes, 5.06T avg overhead

Questions to resolve:
1. Do corpus 6v+ functions fall in the 0.9% feasible region?
2. Composition vs direct GPU cost for 6v corpus shapes
3. Self-hosting table size: ≤3v (2.7K) + treewidth DP → how much RAM on Z80?

## Priority 4: Paper A Final Draft

All data available. Need to integrate:
- Double phase transition (enumeration + feasibility)
- Cross-compiler proof (Nanz=SDCC for u8)
- Composition verification (13.2M points)
- Honest treewidth correction (99.5% random → 46.3% dense)
- 5-level pipeline coverage (80%+15%+5%=100%)

Review cycle: `ddll ask gpt54 -s paper-review @research/paper-a-draft.md "final review"`

## Backlog

### Frontend
- [ ] Nanz import paths for .lanz/.lizp modules
- [ ] Pascal stdlib (blocked on VIR P6)
- [ ] Frill match expression desugaring
- [ ] PL/M MOD operator in HIR lowerer
- [ ] ABAP OPEN SQL: GROUP BY, HAVING, ORDER BY

### Backend (VIR)
- [ ] InlineTrivial asm-body guard (P6)
- [ ] Cross-block clobber constraints (P1)
- [ ] C89 u16 promotion fix
- [ ] MZA instruction table expansion
- [ ] ZX Spectrum real SQLite (main() loop fix)

### Research
- [ ] Paper A final draft
- [ ] Paper B prototype (DP partition)
- [ ] Paper C compositional data
- [ ] ABI paper response to Philipp
- [ ] V6 incremental completion (7 nights)

### Demo
- [ ] ZX Spectrum real SQLite ALV (needs main() loop fix)
- [ ] ZSQL on MZX with ROM font + SQLite ports
- [ ] LinkedIn post — ship with clean screenshots

## Session IDs (will change on reboot)
- minz-vir: check with `ddll explore`
- z80-optimizer: check with `ddll explore`
- minz-abap: check with `ddll explore`
- GPT-5.4: `ddll ask gpt54 -s <session>`

## Key Files

| File | Purpose |
|------|---------|
| `research/README.md` | 4 papers overview |
| `research/paper-a/exhaustive-enumeration-strategy.md` | V4→V5→V6 strategy |
| `research/paper-a/cross-compiler-analysis.md` | Nanz vs SDCC |
| `research/paper-c-compositional-regalloc.md` | Treewidth decomposition |
| `docs/Anytime_Optimal_Register_Allocation.md` | 5-level pipeline |
| `docs/VIR_Reliability_Sprint.md` | VIR sprint plan |
| `contexts/2026-03-26-sprint-plan.md` | Frontend/backend task split |
| `docs/linkedin-post-abap-zx.md` | LinkedIn post draft |
