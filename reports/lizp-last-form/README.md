# Lizp last-form returns: source contract and native proof

Non-void Lizp `defun`/`define` returns its final value form. Earlier forms execute in order; normalized blocks and both branches preserve explicit returns. The language contract is documented in Nanz Language Book v7 §21.4. Bare returns, missing result forms and void-valued returns are rejected. A `cond` clause now preserves all prefix forms instead of dropping everything after its first body form.

The unchanged `examples/lizp/cond_test.lizp` passed all 256 inputs to `classify`, plus original `main`, in ordinary, Grace Size and Grace Speed: 771 native executions and matching raw/final VM checks. The independent thresholds are zero→0, 1..9→1, 10..99→2, 100..255→255. Native classification checks A, exact return PC/SP and unchanged code; main checks HALT, unchanged SP and score=2.

A separate nested-form fixture passed 2304 native calls and matching raw/final VM checks: all byte inputs to three functions in three modes. It covers lexical blocks, explicit returns, early exit and ordered global effects. Checks include A, effects and exact return PC/SP. This is bounded state coverage, not a full-RAM proof.

All six compiled variants retain source, raw/final MIR, actual ASM/BIN and execution summaries below. The native tests exercise HIR Grace and MIR Grace in the two Grace modes. No optimization gain is claimed. These checks postdate the previous complete pipeline gate.

Replay from `minzc`:

```sh
MINZ_LIZP_TAIL_ARTIFACTS=/absolute/output go test ./pkg/pipeline -run '^(TestOriginalLizpCondLastFormNative|TestLizpLastFormEffectsNative)$' -count=1 -v
```
