# zvdb demo — compiler codegen findings and repros

Two codegen **quality** findings (not correctness bugs) hit while writing
`examples/nanz/zvdb_demo_cpm.nanz`. Neither was worked around in the shipped
kernel: `zv_score` / `zv_top_k` stay plain Nanz so they can be asserted with
`via mir2` **and** `via z80`, as task Z1 requires. The numbers below are the
measured cost of that decision.

All measurements: `mz <file>.nanz -t cpm -o x.com && mze x.com -t cpm -c`, which
reports whole-process T-states. Each program loops its kernel 250 times, so
divide by 250 for the per-call cost.

## 1. 32-byte XOR + popcount-LUT scan — `repro-scan-nanz.nanz`

The demo's inner loop (`zvdb_kernel.zv_score_pv`): sum `pop[a[k] ^ b[k]]` over
32 bytes with a 256-byte LUT, 34 scans per pass (one `zv_top_k` over the
35-record data set).

| variant | whole run (250 passes) | per pass (34 scans) | per scan | per byte |
|---|---|---|---|---|
| `repro-scan-nanz.nanz` (plain Nanz) | 84,734,801 T | 338,939 T | 9,969 T | ~311 T |
| `repro-scan-asm.nanz` (inline Z80) | 35,017,301 T | 140,069 T | 4,120 T | ~129 T |

Both carry the same harness (`zv_vec` twice per scan plus the loop), so the
scan bodies are roughly 8,800 T vs 2,950 T — **about 3x**. The generated loop
(~35 instructions per byte) keeps three pointers plus a 16-bit accumulator live,
so PBQP spills the `b` pointer and the accumulator to memory and reloads the LUT
base every iteration (`PUSH IX / POP HL / ADD HL,DE`), while hand Z80 holds
`HL`=a, `DE`=b, `BC`=LUT, `IX`=accumulator, `IYL`=counter and needs no spills.

Consequence for the demo: one full ranking pass costs ~721k T-states instead of
~250k, i.e. 180 ms instead of ~63 ms at 4 MHz. Still instant under `mze`.

## 2. Dynamically indexed `u16` global array element move — `repro-array-move.nanz`

```
zv_val[k] = zv_val[k - 1]
zv_idx[k] = zv_idx[k - 1]
```

is the shift step of the demo's ranking insertion. 250 repetitions of a
200-element shift take 15,459,043 T — **309 T-states per element move**. The
emitted code re-materialises both array bases, doubles the index with
`SLA C / RL B`, stages the loaded low byte through `PUSH AF` and routes the
destination address through `PUSH IX / POP IX` (about 40 instructions).

A pointer walk doing the same job costs ~6 T per step. Rewriting the ranking as
three parallel byte arrays walked by pointers was tried and measured *slower*
end to end (1.54M T per `zv_top_k`, against 1.00M for the same arrays addressed
by index and 748k for the shipped `u16` score array): the walk needs four O(cnt)
passes over three arrays — find, two pointer walks to reach the copy window, and
the store — versus one indexed pass over two arrays. The shipped form is the
fastest of the three.

## What would fix it

1. Rematerialise a constant array base as `LD rr, base` instead of
   `PUSH IX / POP HL` inside loops, and prefer `LD A,(BC)` with the base held in
   a pair across the loop.
2. Recognise `arr[i] = arr[i-1]` (and `arr16[i] = arr16[i-1]`) on a global array
   as a pointer copy, halving the shift cost.
