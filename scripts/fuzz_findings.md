# Differential fuzz smoke run

Main: `ed55c1c7fa0c775522b06154e4670b29ac1981a3` (also the branch base).
Command: `python3 scripts/fuzz_diff.py --mz /tmp/j2a-main-mz --seed 0 --count 500 -j 16 --output /tmp/j2a-fuzz-verified`.
Wall time: 5.19 seconds; exit code 1 (findings). 250 passed, 2 wrong-value
mismatches, 248 assembly errors, 0 interpreter errors. Compiler fixes are
outside this task.

| Seed | Original got / want | Reduced got / want | Reproducer |
| --- | --- | --- | --- |
| 56 | 15015 / 15271 | 15014 / 15270 | [seed-56.nanz](testdata/fuzz_diff/seed-56.nanz) |
| 399 | 456 / 712 | 17814 / 18070 | [seed-399.nanz](testdata/fuzz_diff/seed-399.nanz) |

Both reduced programs disagree by 256. Forcing MIR2 on the reduced
reproducers returns the same values as Z80: seed 56 returns 15014 and seed 399
returns 17814. These are MIR2≠oracle findings, not evidence of Z80≠MIR2.
The frontend/HIR literal typing or the oracle semantics need investigation:
the Python oracle defaults literal-only subexpressions to 16 bits. Seed 56 contains `(255 + 3)`
inside a u16 expression; seed 399 contains `(2 - 100)`. These are observed
wrong-value failures against the prototype's Python reference semantics;
the likely area to investigate is the width of nested literal arithmetic.
The saved sources are local deletion minima over helper functions, control
blocks and statement lines; expression simplification is not attempted.

Assembly failures (first invalid instruction):

- `LD` (202 seeds): 0, 1, 3, 6, 8, 9, 11, 13, 20, 21, 22, 24, 27, 30, 34, 43, 46, 49, 50, 52, 54, 61, 63, 64, 68, 69, 70, 77, 79, 80, 82, 84, 85, 86, 89, 90, 91, 97, 98, 102, 105, 106, 108, 111, 114, 115, 117, 119, 120, 121, 126, 128, 131, 132, 133, 135, 138, 139, 140, 145, 148, 150, 157, 162, 163, 164, 168, 169, 171, 172, 173, 176, 177, 178, 181, 184, 189, 190, 191, 192, 196, 200, 201, 202, 203, 208, 209, 210, 211, 212, 214, 219, 220, 224, 226, 228, 229, 231, 234, 235, 237, 238, 242, 244, 245, 246, 247, 249, 251, 256, 257, 258, 260, 262, 263, 267, 268, 272, 275, 276, 277, 278, 282, 287, 288, 292, 293, 294, 295, 299, 303, 306, 308, 309, 313, 315, 319, 321, 324, 326, 327, 331, 334, 335, 337, 338, 344, 348, 355, 356, 357, 358, 360, 363, 368, 370, 371, 372, 382, 385, 390, 392, 394, 395, 398, 413, 417, 418, 420, 426, 429, 430, 433, 435, 439, 440, 441, 443, 444, 446, 447, 448, 453, 454, 455, 457, 462, 464, 467, 469, 470, 472, 473, 474, 476, 479, 480, 481, 484, 497, 498, 499.
- `JP` (42 seeds): 4, 15, 29, 36, 37, 44, 47, 51, 109, 134, 142, 152, 158, 175, 179, 180, 187, 222, 239, 248, 255, 283, 296, 300, 305, 311, 329, 333, 342, 349, 369, 376, 377, 400, 402, 410, 419, 436, 449, 456, 471, 483.
- `SBC` (4 seeds): 118, 144, 270, 411.

All failures can be regenerated with the command above; `results.json` includes each full first diagnostic.

## Compiler-stage verification (FIX1)

Local `origin/main`: `e0fabc22473881d3967f2944744f965966c56c10`, built
with reporting/selection instrumentation while retaining its codegen and
assert recognition/force behavior. Command:

```sh
python3 scripts/fuzz_diff.py --mz /tmp/j2a-fix1-main-mz --seed 0 --count 500 -j 16 --no-reduce --output /tmp/j2a-fix1-fuzz-triage
```

Exit 1, wall 17.07 seconds. Primary classes: 250 pass, 5 MIR2≠oracle,
0 Z80≠MIR2, 245 assembly failure, 0 other compiler/oracle errors. MIR2/oracle
discrepancies take precedence over assembly failures in the primary class;
three seeds have both findings, so total observed Z80 assembly failures are
248. Full sources and both backend diagnostics are saved in the output.
`--no-reduce` deliberately saves full reproducers for this smoke; it makes
no claim that these newly recorded programs are minimal.

| Seed | MIR2 got / oracle | Z80 result |
| --- | --- | --- |
| 56 | 15015 / 15271 | Same wrong value |
| 200 | 0 / 45640 | Assembly failure: LD |
| 235 | 0 / 54048 | Assembly failure: LD |
| 399 | 456 / 712 | Same wrong value |
| 498 | 0 / 6026 | Assembly failure: LD |

The additional MIR2 discrepancies at 200, 235 and 498 were previously
hidden by assembly failures; their cause is not established. For the saved
reduced 56/399 fixtures, real compiler integration tests also verify that
forced MIR2 and Z80 return identical wrong values (15014 and 17814).
The old wrong-value count described only completed Z80 evaluations; it
must not be interpreted as the count of Z80 codegen discrepancies.
