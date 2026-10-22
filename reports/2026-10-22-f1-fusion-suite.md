# F1: frozen iterator-fusion benchmark suite (2026-10-22)

Eight new cases run successfully on the unchanged production compiler. All 80 matrix rows pass: MinZ C default/size/grace/nograce, idiomatic Nanz default/size/grace, SDCC 4.6 size/speed, and hand-written Z80. GCC independently passes all 720 case samples. Fusion produces single source traversals, but the Nanz code remains 1.90–6.37× the hand reference in workload T-states; removing stage calls does not remove register-allocation and lowering costs.

## Scope and reproducibility

Compiler baseline: `33418f9057450ef422ed349594e0bce408b9c881` (origin/main; bench/f1-fusion-suite starts here). This task changes benchmark infrastructure, corpus, documentation and its focused test only. No production compiler source changed. Main and heldout source/manifests are untouched; `all` retains its existing main+heldout meaning. Heldout was not run.

SDCC is the local official **4.6.0 #16555**, from `/home/alice/opt/sdcc-4.6.0-root` (`~/opt/sdcc-4.6.0-root`). The separate mandatory main-suite before/after control uses system SDCC (`--sdcc-root /`, version recorded with those artifacts). No sccz80 installation is required or selected.

Commands (run from the repository root, each heavy command preceded by `uptime`; loads stayed below 12):

```sh
export TMPDIR=/mnt/ssd-510/minz/tmp GOCACHE=/mnt/ssd-510/minz/gocache GOMAXPROCS=4
nice -n 19 python3 research/c89-benchmark/run.py --suite fusion --jobs 4 \
  --sdcc-root /home/alice/opt/sdcc-4.6.0-root --out /mnt/ssd-510/minz/bench/f1-fusion-v2
nice -n 19 python3 research/c89-benchmark/check_host.py --out /mnt/ssd-510/minz/bench/f1-fusion-v2
```

Raw images, inputs, commands, build/run logs, per-call observations, raw/final MIR and assembly are retained at `/mnt/ssd-510/minz/bench/f1-fusion-v2`. Portable records committed beside this report: [summary](2026-10-22-f1-fusion-suite/summary.json), [toolchain/source/workload hashes](2026-10-22-f1-fusion-suite/toolchains.json), [GCC replay](2026-10-22-f1-fusion-suite/host-oracle.json). The runner and mza are hashed in toolchain metadata. Every row has the same workload hash as its other language/profile rows. Source/workload SHA256 manifests were fixed before this final matrix.

## Corpus and semantic limits

The [corpus contract](../research/c89-benchmark/fusion/README.md) defines unsigned arithmetic, forward order, disjoint 256-byte buffers and 16-bit lengths in 0..256. Byte doubling wraps modulo 256 before comparison; dot product accumulates modulo 65536. Min/max return 255/0 on empty input. Store returns n and verifies the entire 256-byte destination, including its untouched tail. All input globals are checked unchanged. Counts 0,1,2,3,4,5,16,127,128,254,255,256 occur over threshold/overflow bytes, all-6, all-0, all-255 and a ramp. Count uses k=0,6,255; window uses limit=0,7,256. This supplies all-pass/none-pass cases for each predicate and product/word-overflow cases. There are 60 samples per case, 180 for count/window, 720 total; the matrix executes 7,200 native calls and MinZ/Nanz additionally replay raw/final MIR.

The G0 audit takes precedence over historical book claims. Unsupported `iter`, `skip`, `take`, `enumerate` are G0-5; enumerate is omitted. No native zip/count/any/all/find is implemented in the audited memory-chain interface. Windowing uses a clamped pointer/count followed by fold. Dot product uses forEach with a captured second pointer (zip-equivalent for these disjoint, equal-length arrays). Count uses filter→forEach. Filter→map→fold fully consumes n; it makes no early-exit claim. No range countdown source is used, avoiding G0-9 at 256. The hand code uses the same declared 0..256 bound to do one full-word empty check then DJNZ (B=0 means 256). This is an explicit domain contract, not a general u16-count implementation.

Initial probes are retained at `/mnt/ssd-510/minz/bench/f1-fusion` (15 Nanz failures). Direct assignment of a fold result left `result` at 0xBEEF on raw/final MIR and Z80; nesting a fold inside a cast emitted an unresolved `CALL fold`. The final sources bind each fold result to a typed local before storing it. No expectation, workload or compiler was changed to hide these probes. Min/max conditional callbacks compile and execute as ordinary helper calls; that cost stays in the matrix.

## Measurements

Bytes are occupied image plus declared/compiler storage, counting overlaps once, including the 256-/512-byte buffers; they are **not code-only bytes**. T is the sum of complete execute() calls over the frozen workload, including setup, helpers and return. Ratios are `row / reference` (lower is better), written **bytes / T**. SDCC ratios always use the predeclared **speed** row, never a post-selected winner; both SDCC rows remain visible. Default/size/grace are identical in bytes and T on all eight cases for each MinZ frontend; nograce is retained as an extra C control.

| Case | Compiler / profile | Bytes | T | / hand B / T | / SDCC speed B / T |
|---|---|---:|---:|---:|---:|
| dot_product | minz / default | 631 | 7,983,736 | 1.11 / 2.96 | 1.08 / 2.82 |
| dot_product | minz / size | 631 | 7,983,736 | 1.11 / 2.96 | 1.08 / 2.82 |
| dot_product | minz / grace | 631 | 7,983,736 | 1.11 / 2.96 | 1.08 / 2.82 |
| dot_product | minz / nograce | 631 | 7,983,736 | 1.11 / 2.96 | 1.08 / 2.82 |
| dot_product | sdcc / size | 590 | 2,846,427 | 1.04 / 1.06 | 1.01 / 1.00 |
| dot_product | sdcc / speed | 586 | 2,834,057 | 1.04 / 1.05 | 1.00 / 1.00 |
| dot_product | nanz-idiom / default | 668 | 8,028,801 | 1.18 / 2.98 | 1.14 / 2.83 |
| dot_product | nanz-idiom / size | 668 | 8,028,801 | 1.18 / 2.98 | 1.14 / 2.83 |
| dot_product | nanz-idiom / grace | 668 | 8,028,801 | 1.18 / 2.98 | 1.14 / 2.83 |
| dot_product | hand / asm | 566 | 2,695,436 | 1.00 / 1.00 | 0.97 / 0.95 |
| filter_count | minz / default | 314 | 3,230,665 | 1.08 / 4.73 | 1.06 / 1.78 |
| filter_count | minz / size | 314 | 3,230,665 | 1.08 / 4.73 | 1.06 / 1.78 |
| filter_count | minz / grace | 314 | 3,230,665 | 1.08 / 4.73 | 1.06 / 1.78 |
| filter_count | minz / nograce | 314 | 3,230,665 | 1.08 / 4.73 | 1.06 / 1.78 |
| filter_count | sdcc / size | 295 | 1,849,750 | 1.01 / 2.71 | 1.00 / 1.02 |
| filter_count | sdcc / speed | 296 | 1,818,220 | 1.01 / 2.66 | 1.00 / 1.00 |
| filter_count | nanz-idiom / default | 332 | 4,088,375 | 1.14 / 5.99 | 1.12 / 2.25 |
| filter_count | nanz-idiom / size | 332 | 4,088,375 | 1.14 / 5.99 | 1.12 / 2.25 |
| filter_count | nanz-idiom / grace | 332 | 4,088,375 | 1.14 / 5.99 | 1.12 / 2.25 |
| filter_count | hand / asm | 292 | 682,705 | 1.00 / 1.00 | 0.99 / 0.38 |
| filter_map_sum | minz / default | 337 | 1,428,224 | 1.15 / 5.08 | 1.12 / 2.28 |
| filter_map_sum | minz / size | 337 | 1,428,224 | 1.15 / 5.08 | 1.12 / 2.28 |
| filter_map_sum | minz / grace | 337 | 1,428,224 | 1.15 / 5.08 | 1.12 / 2.28 |
| filter_map_sum | minz / nograce | 337 | 1,428,224 | 1.15 / 5.08 | 1.12 / 2.28 |
| filter_map_sum | sdcc / size | 299 | 636,652 | 1.02 / 2.27 | 1.00 / 1.02 |
| filter_map_sum | sdcc / speed | 300 | 626,142 | 1.02 / 2.23 | 1.00 / 1.00 |
| filter_map_sum | nanz-idiom / default | 357 | 1,432,432 | 1.22 / 5.10 | 1.19 / 2.29 |
| filter_map_sum | nanz-idiom / size | 357 | 1,432,432 | 1.22 / 5.10 | 1.19 / 2.29 |
| filter_map_sum | nanz-idiom / grace | 357 | 1,432,432 | 1.22 / 5.10 | 1.19 / 2.29 |
| filter_map_sum | hand / asm | 293 | 280,890 | 1.00 / 1.00 | 0.98 / 0.45 |
| fold_max | minz / default | 325 | 1,209,565 | 1.13 / 5.46 | 1.09 / 2.09 |
| fold_max | minz / size | 325 | 1,209,565 | 1.13 / 5.46 | 1.09 / 2.09 |
| fold_max | minz / grace | 325 | 1,209,565 | 1.13 / 5.46 | 1.09 / 2.09 |
| fold_max | minz / nograce | 325 | 1,209,565 | 1.13 / 5.46 | 1.09 / 2.09 |
| fold_max | sdcc / size | 297 | 588,803 | 1.03 / 2.66 | 1.00 / 1.02 |
| fold_max | sdcc / speed | 298 | 578,293 | 1.03 / 2.61 | 1.00 / 1.00 |
| fold_max | nanz-idiom / default | 316 | 1,120,286 | 1.10 / 5.06 | 1.06 / 1.94 |
| fold_max | nanz-idiom / size | 316 | 1,120,286 | 1.10 / 5.06 | 1.06 / 1.94 |
| fold_max | nanz-idiom / grace | 316 | 1,120,286 | 1.10 / 5.06 | 1.06 / 1.94 |
| fold_max | hand / asm | 288 | 221,379 | 1.00 / 1.00 | 0.97 / 0.38 |
| fold_min | minz / default | 325 | 1,120,125 | 1.13 / 4.96 | 1.09 / 2.01 |
| fold_min | minz / size | 325 | 1,120,125 | 1.13 / 4.96 | 1.09 / 2.01 |
| fold_min | minz / grace | 325 | 1,120,125 | 1.13 / 4.96 | 1.09 / 2.01 |
| fold_min | minz / nograce | 325 | 1,120,125 | 1.13 / 4.96 | 1.09 / 2.01 |
| fold_min | sdcc / size | 296 | 568,901 | 1.03 / 2.52 | 1.00 / 1.02 |
| fold_min | sdcc / speed | 297 | 558,391 | 1.03 / 2.47 | 1.00 / 1.00 |
| fold_min | nanz-idiom / default | 317 | 1,121,667 | 1.10 / 4.97 | 1.07 / 2.01 |
| fold_min | nanz-idiom / size | 317 | 1,121,667 | 1.10 / 4.97 | 1.07 / 2.01 |
| fold_min | nanz-idiom / grace | 317 | 1,121,667 | 1.10 / 4.97 | 1.07 / 2.01 |
| fold_min | hand / asm | 288 | 225,706 | 1.00 / 1.00 | 0.97 / 0.40 |
| map_filter_sum | minz / default | 359 | 1,787,906 | 1.21 / 5.12 | 1.10 / 1.76 |
| map_filter_sum | minz / size | 359 | 1,787,906 | 1.21 / 5.12 | 1.10 / 1.76 |
| map_filter_sum | minz / grace | 359 | 1,787,906 | 1.21 / 5.12 | 1.10 / 1.76 |
| map_filter_sum | minz / nograce | 359 | 1,787,906 | 1.21 / 5.12 | 1.10 / 1.76 |
| map_filter_sum | sdcc / size | 331 | 1,026,874 | 1.11 / 2.94 | 1.01 / 1.01 |
| map_filter_sum | sdcc / speed | 327 | 1,014,504 | 1.10 / 2.91 | 1.00 / 1.00 |
| map_filter_sum | nanz-idiom / default | 399 | 2,223,084 | 1.34 / 6.37 | 1.22 / 2.19 |
| map_filter_sum | nanz-idiom / size | 399 | 2,223,084 | 1.34 / 6.37 | 1.22 / 2.19 |
| map_filter_sum | nanz-idiom / grace | 399 | 2,223,084 | 1.34 / 6.37 | 1.22 / 2.19 |
| map_filter_sum | hand / asm | 297 | 349,042 | 1.00 / 1.00 | 0.91 / 0.34 |
| map_store | minz / default | 566 | 1,037,600 | 1.05 / 4.51 | 1.03 / 1.57 |
| map_store | minz / size | 566 | 1,037,600 | 1.05 / 4.51 | 1.03 / 1.57 |
| map_store | minz / grace | 566 | 1,037,600 | 1.05 / 4.51 | 1.03 / 1.57 |
| map_store | minz / nograce | 566 | 1,037,600 | 1.05 / 4.51 | 1.03 / 1.57 |
| map_store | sdcc / size | 551 | 673,505 | 1.02 / 2.92 | 1.00 / 1.02 |
| map_store | sdcc / speed | 552 | 662,995 | 1.02 / 2.88 | 1.00 / 1.00 |
| map_store | nanz-idiom / default | 555 | 438,350 | 1.03 / 1.90 | 1.01 / 0.66 |
| map_store | nanz-idiom / size | 555 | 438,350 | 1.03 / 1.90 | 1.01 / 0.66 |
| map_store | nanz-idiom / grace | 555 | 438,350 | 1.03 / 1.90 | 1.01 / 0.66 |
| map_store | hand / asm | 540 | 230,310 | 1.00 / 1.00 | 0.98 / 0.35 |
| window_sum | minz / default | 397 | 2,356,140 | 1.26 / 8.75 | 1.08 / 1.79 |
| window_sum | minz / size | 397 | 2,356,140 | 1.26 / 8.75 | 1.08 / 1.79 |
| window_sum | minz / grace | 397 | 2,356,140 | 1.26 / 8.75 | 1.08 / 1.79 |
| window_sum | minz / nograce | 397 | 2,356,140 | 1.26 / 8.75 | 1.08 / 1.79 |
| window_sum | sdcc / size | 361 | 1,142,715 | 1.14 / 4.24 | 0.98 / 0.87 |
| window_sum | sdcc / speed | 369 | 1,314,768 | 1.17 / 4.88 | 1.00 / 1.00 |
| window_sum | nanz-idiom / default | 387 | 997,420 | 1.22 / 3.70 | 1.05 / 0.76 |
| window_sum | nanz-idiom / size | 387 | 997,420 | 1.22 / 3.70 | 1.05 / 0.76 |
| window_sum | nanz-idiom / grace | 387 | 997,420 | 1.22 / 3.70 | 1.05 / 0.76 |
| window_sum | hand / asm | 316 | 269,353 | 1.00 / 1.00 | 0.86 / 0.20 |

The Nanz store chain is faster than both SDCC profiles (438,350 T vs 662,995 speed / 673,505 size), and window fold is faster than both (997,420 T vs 1,314,768 speed / 1,142,715 size). Other Nanz rows are slower than SDCC. Every Nanz row is slower and larger than the hand reference. These are measurements of this frozen workload, not a claim over all possible data.

## Emitted Nanz loops and hand references

The following are verbatim default-profile Nanz consumer kernels (through exit and spill slots, omitting global buffers) and matching complete hand kernels. All three Nanz profiles have identical measured size/T. The outer memory traversal is fused once in every case; dot product contains a nested arithmetic multiply loop. Min/max helper bodies are included because the hot loop calls them. Unused named add16 helpers can survive separately in the compiled module; their bytes are included in the table. `JRS` is the assembler’s relative-branch pseudo-op.

### dot_product

This is the documented zip substitute: one source forEach plus a captured second pointer, with no zip combinator claim. The second pointer and widened second byte spill; pair transfers surround a generic 16-iteration multiply, although both operands originate as bytes. Hand code walks both arrays in the shadow register bank and performs an eight-bit shift/add product directly into the running word sum. No data-specific table or multiplication shortcut is used (~2.98× hand T).

Nanz consumer:

```asm
execute:
    LD HL, 0
    LD BC, other
    LD DE, source
    LD IX, n
    PUSH AF          ; preserve word-load scratch
    LD A, (IX+0)     ; lo
    PUSH AF          ; stage low byte before replacing address
    LD A, (IX+1)     ; hi
    LD IXH, A
    POP AF
    LD IXL, A
    POP AF           ; restore word-load scratch
    LD IY, 0
    LD (_spill_execute_r11), BC
    LD B, IXH
    LD C, IXL
.execute_fe_head1:
    LD A, B
    OR C
    JRS Z, .execute_trmp0
.execute_fe_body2:
    LD A, (DE)
    LD IXL, A
    LD IXH, 0
    PUSH HL
    LD HL, (_spill_execute_r11)   ; reload spilled ptr
    LD A, (HL)
    POP HL
    PUSH AF
    PUSH AF
    LD A, 0
    LD (_spill_execute_r17+1), A
    POP AF
    LD (_spill_execute_r17), A
    POP AF
    PUSH HL
    PUSH IX
    POP HL
    ; mul16 IX * _spill_execute_r17 → HL  (~320T, 16-iter shift-and-add)
    PUSH AF
    PUSH DE
    LD DE, (_spill_execute_r17)
    PUSH BC
    LD B, H
    LD C, L
    LD H, 0
    LD L, 0
    LD A, 16
.execute_m16_18_lp:
    SRL D
    RR E
    JRS NC, .execute_m16_18_sk
    ADD HL, BC
.execute_m16_18_sk:
    SLA C
    RL B
    DEC A
    JRS NZ, .execute_m16_18_lp
    POP BC
    POP DE
    POP AF
    PUSH HL
    POP IX
    POP HL
    PUSH DE
    PUSH DE
    LD D, IXH
    LD E, IXL
    ADD HL, DE
    POP DE
    POP DE
    PUSH HL
    LD HL, (_spill_execute_r11)
    INC HL
    LD (_spill_execute_r21), HL
    POP HL
    INC DE
    DEC BC
    PUSH HL
    LD HL, (_spill_execute_r21)
    LD (_spill_execute_r11), HL
    POP HL
    JRS .execute_fe_head1
.execute_fe_exit4:
    LD (result), DE    ; u16 direct store, no scratch clobber
    RET
.execute_trmp0:
    LD D, H
    LD E, L
    JRS .execute_fe_exit4
; — spill slots for execute (1 spills)
_spill_execute_r11: DW 0
; — extra spill slots for execute (2 rescued)
_spill_execute_r17: DB 0, 0
_spill_execute_r21: DB 0, 0
```

Hand reference:

```asm
; Hand Z80 reference. Contract: 0 <= n <= 256; disjoint buffers.
ORG 0x8000
execute:
    LD HL,[n]
    LD A,H
    OR L
    LD B,L
    LD HL,0
    JR Z,done
    EXX
    LD HL,source
    LD DE,other
    EXX
loop:
    EXX
    LD A,[DE]
    INC DE
    LD C,[HL]
    INC HL
    PUSH BC
    EXX
    POP DE
    LD D,0
    PUSH BC
    LD B,8
multiply:
    SRL A
    JR NC,no_add
    ADD HL,DE
no_add:
    SLA E
    RL D
    DJNZ multiply
    POP BC
    DJNZ loop
done:
    LD [result],HL
    RET
```

### filter_count

Filter and captured count are fused and the count stays in HL. The loop repeatedly rematerializes the address of the invariant global k, reloads/spills it and saves registers around the comparison. Hand code hoists k into C once, increments DE on a match and uses DJNZ. This is the largest T ratio in the suite after rounding (~6.0×; the double-filter case is ~6.37×).

Nanz consumer:

```asm
execute:
    LD HL, 0
    LD DE, source
    LD BC, (n)    ; u16 direct load, no scratch clobber
    LD IX, 0
.execute_fe_head1:
    LD A, B
    OR C
    JRS Z, .execute_trmp0
.execute_fe_body2:
    LD A, (DE)
    PUSH HL
    LD HL, k
    LD (_spill_execute_r13), HL
    POP HL
    PUSH BC
    PUSH AF
    PUSH HL
    LD HL, (_spill_execute_r13)   ; reload spilled ptr
    EX AF, AF'
    LD A, (HL)
    LD (_spill_execute_r14), A
    EX AF, AF'
    POP HL
    POP BC
    LD A, B
    POP BC
    PUSH HL
    LD HL, _spill_execute_r14
    LD H, (HL)
    CP H
    POP HL
    JRS Z, .execute_if_join6
.execute_if_then5:
.execute_fe_cont3:
    INC DE
    DEC BC
    JRS .execute_fe_head1
.execute_if_join6:
    INC HL
    JRS .execute_fe_cont3
.execute_fe_exit4:
    LD (result), DE    ; u16 direct store, no scratch clobber
    RET
.execute_trmp0:
    LD D, H
    LD E, L
    JRS .execute_fe_exit4
; — extra spill slots for execute (2 rescued)
_spill_execute_r13: DB 0, 0
_spill_execute_r14: DB 0
```

Hand reference:

```asm
; Hand Z80 reference. Contract: 0 <= n <= 256; disjoint buffers.
ORG 0x8000
execute:
    LD HL,[n]
    LD A,H
    OR L
    LD B,L
    LD DE,0
    JR Z,done
    LD A,[k]
    LD C,A
    LD HL,source
loop:
    LD A,[HL]
    INC HL
    CP C
    JR NZ,next
    INC DE
next:
    DJNZ loop
done:
    LD [result],DE
    RET
```

### filter_map_sum

Filter, XOR map and word fold fuse into one traversal. The predicate is materialized and spilled, then the mapped byte is widened via another spill plus BC preservation. The hand loop tests bit 7 directly, uses CPL, and accumulates via byte carry. All n inputs are consumed; no any/all/find or early-stop lowering is being measured (~5.10× hand T).

Nanz consumer:

```asm
execute:
    LD HL, 0
    LD DE, source
    LD BC, (n)    ; u16 direct load, no scratch clobber
    LD IX, 0
.execute_fe_head1:
    LD A, B
    OR C
    JRS Z, .execute_trmp0
.execute_fe_body2:
    LD A, (DE)
    CP 128
    PUSH AF
    LD A, 0
    LD (_spill_execute_r18), A
    POP AF
    PUSH AF
    JP NC, .bool_true_1
.bool_false_1:
    LD A, 0
    JP .bool_done_1
.bool_true_1:
    LD A, 1
.bool_done_1:
    LD (_spill_execute_r17), A
    POP AF
    PUSH BC
    PUSH AF
    LD A, (_spill_execute_r17)
    AND A
    POP BC
    LD A, B
    POP BC
    JRS Z, .execute_if_join6
.execute_if_then5:
    XOR 255
    PUSH AF
    PUSH AF
    LD A, 0
    LD (_spill_execute_r22+1), A
    POP AF
    LD (_spill_execute_r22), A
    POP AF
    PUSH BC
    LD BC, (_spill_execute_r22)
    ADD HL, BC
    POP BC
.execute_if_join6:
    INC DE
    DEC BC
    JRS .execute_fe_head1
.execute_fe_exit4:
    LD (result), DE    ; u16 direct store, no scratch clobber
    RET
.execute_trmp0:
    LD D, H
    LD E, L
    JRS .execute_fe_exit4
; — extra spill slots for execute (3 rescued)
_spill_execute_r18: DB 0
_spill_execute_r17: DB 0
_spill_execute_r22: DB 0, 0
```

Hand reference:

```asm
; Hand Z80 reference. Contract: 0 <= n <= 256; disjoint buffers.
ORG 0x8000
execute:
    LD HL,[n]
    LD A,H
    OR L
    LD B,L
    LD DE,0
    JR Z,done
    LD HL,source
loop:
    LD A,[HL]
    INC HL
    BIT 7,A
    JR Z,next
    CPL
    ADD A,E
    LD E,A
    JR NC,no_carry
    INC D
no_carry:
next:
    DJNZ loop
done:
    LD [result],DE
    RET
```

### fold_max

As with minimum, conditional callback larger remains a call with broad register preservation and scratch moves. The hand loop performs a direct comparison and conditional accumulator replacement. Empty input still returns zero on all three execution tiers (~5.06× hand T).

Nanz consumer:

```asm
larger:
    CP B
    RET NC
.larger_if_then1:
    LD A, B
    RET

; fun execute() ; clobbers: A, B, BC, C, D, DE, E, F, H, HL, IX, IXH, IXL, IY, IYH, IYL, L
; [trace] backend=PBQP variant=z80-loop-max-v1/counted=0,mir-grace=true,hir-grace=false/0->0 passes=[ident-simp=1,dse=1,dead-block-arg=1]
execute:
    LD A, 0
    LD HL, source
    LD DE, (n)    ; u16 direct load, no scratch clobber
    LD BC, 0
.execute_fe_head1:
    LD IYH, A
    ; materialize r4 (pendingAcc)
    LD A, D
    OR E
    ; restore live-out r4 from scratch
    LD A, IYH
    JRS Z, .execute_fe_exit4
.execute_fe_body2:
    EX AF, AF'      ; preserve byte-load scratch
    LD A, (HL)
    LD IYH, A
    EX AF, AF'      ; restore byte-load scratch
    ; genCall: larger dst=17
    PUSH BC
    PUSH DE
    PUSH HL
    PUSH IX
    LD B, IYH
    CALL larger
    POP IX
    POP HL
    POP DE
    POP BC
    INC HL
    DEC DE
    JRS .execute_fe_head1
.execute_fe_exit4:
    LD E, A
    LD D, 0
    LD (result), DE    ; u16 direct store, no scratch clobber
    RET
```

Hand reference:

```asm
; Hand Z80 reference. Contract: 0 <= n <= 256; disjoint buffers.
ORG 0x8000
execute:
    LD HL,[n]
    LD A,H
    OR L
    LD B,L
    LD E,0
    LD D,0
    JR Z,done
    LD HL,source
loop:
    LD A,[HL]
    INC HL
    CP E
    JR C,next
    LD E,A
next:
    DJNZ loop
done:
    LD [result],DE
    RET
```

### fold_min

The fold traversal is one loop, but the multi-statement conditional callback uses a real CALL smaller. The caller saves BC/DE/HL/IX each iteration and shuffles A through IYH/AF shadow state. Hand code keeps the minimum in E and conditionally replaces it inline. This demonstrates that fold support does not imply callback control-flow inlining (~4.97× hand T).

Nanz consumer:

```asm
smaller:
    CP B
    RET Z
    RET C
.smaller_if_then1:
    LD A, B
    RET

; fun execute() ; clobbers: A, B, BC, C, D, DE, E, F, H, HL, IX, IXH, IXL, IY, IYH, IYL, L
; [trace] backend=PBQP variant=z80-loop-max-v1/counted=0,mir-grace=true,hir-grace=false/0->0 passes=[ident-simp=1,dse=1,dead-block-arg=1]
execute:
    LD A, 255
    LD HL, source
    LD DE, (n)    ; u16 direct load, no scratch clobber
    LD BC, 0
.execute_fe_head1:
    LD IYH, A
    ; materialize r4 (pendingAcc)
    LD A, D
    OR E
    ; restore live-out r4 from scratch
    LD A, IYH
    JRS Z, .execute_fe_exit4
.execute_fe_body2:
    EX AF, AF'      ; preserve byte-load scratch
    LD A, (HL)
    LD IYH, A
    EX AF, AF'      ; restore byte-load scratch
    ; genCall: smaller dst=17
    PUSH BC
    PUSH DE
    PUSH HL
    PUSH IX
    LD B, IYH
    CALL smaller
    POP IX
    POP HL
    POP DE
    POP BC
    INC HL
    DEC DE
    JRS .execute_fe_head1
.execute_fe_exit4:
    LD E, A
    LD D, 0
    LD (result), DE    ; u16 direct store, no scratch clobber
    RET
```

Hand reference:

```asm
; Hand Z80 reference. Contract: 0 <= n <= 256; disjoint buffers.
ORG 0x8000
execute:
    LD HL,[n]
    LD A,H
    OR L
    LD B,L
    LD E,255
    LD D,0
    JR Z,done
    LD HL,source
loop:
    LD A,[HL]
    INC HL
    CP E
    JR NC,next
    LD E,A
next:
    DJNZ loop
done:
    LD [result],DE
    RET
```

### map_filter_sum

The map and both filters are in the same loop; their lambdas are pruned. Each filter materializes a boolean through branches and spill bytes, then tests it again. Widening the accepted byte also spills and saves BC. The hand loop branches directly on CP flags, accumulates with byte carry, and uses DJNZ after its initial word empty check. Fusion removes traversal/call overhead but does not recover this register/flags layout.

Nanz consumer:

```asm
execute:
    LD HL, 0
    LD DE, source
    LD BC, (n)    ; u16 direct load, no scratch clobber
    LD IX, 0
.execute_fe_head1:
    LD A, B
    OR C
    JRS Z, .execute_trmp0
.execute_fe_body2:
    LD A, (DE)
    ADD A, A
    CP 10
    PUSH AF
    LD A, 0
    LD (_spill_execute_r20), A
    POP AF
    PUSH AF
    JP Z, .bool_false_1
    JP C, .bool_false_1
    JP .bool_true_1
.bool_false_1:
    LD A, 0
    JP .bool_done_1
.bool_true_1:
    LD A, 1
.bool_done_1:
    LD (_spill_execute_r19), A
    POP AF
    PUSH BC
    PUSH AF
    LD A, (_spill_execute_r19)
    AND A
    POP BC
    LD A, B
    POP BC
    JRS Z, .execute_if_join6
.execute_if_then5:
    CP 200
    PUSH AF
    LD A, 0
    LD (_spill_execute_r24), A
    POP AF
    PUSH AF
    JP C, .bool_true_2
.bool_false_2:
    LD A, 0
    JP .bool_done_2
.bool_true_2:
    LD A, 1
.bool_done_2:
    LD (_spill_execute_r23), A
    POP AF
    PUSH BC
    PUSH AF
    LD A, (_spill_execute_r23)
    AND A
    POP BC
    LD A, B
    POP BC
    JRS Z, .execute_if_join8
.execute_if_then7:
    PUSH AF
    PUSH AF
    LD A, 0
    LD (_spill_execute_r26+1), A
    POP AF
    LD (_spill_execute_r26), A
    POP AF
    PUSH BC
    LD BC, (_spill_execute_r26)
    ADD HL, BC
    POP BC
.execute_if_join8:
.execute_if_join6:
    INC DE
    DEC BC
    JRS .execute_fe_head1
.execute_fe_exit4:
    LD (result), DE    ; u16 direct store, no scratch clobber
    RET
.execute_trmp0:
    LD D, H
    LD E, L
    JRS .execute_fe_exit4
; — extra spill slots for execute (5 rescued)
_spill_execute_r20: DB 0
_spill_execute_r19: DB 0
_spill_execute_r24: DB 0
_spill_execute_r23: DB 0
_spill_execute_r26: DB 0, 0
```

Hand reference:

```asm
; Hand Z80 reference. Contract: 0 <= n <= 256; disjoint buffers.
ORG 0x8000
execute:
    LD HL,[n]
    LD A,H
    OR L
    LD B,L
    LD DE,0
    JR Z,done
    LD HL,source
loop:
    LD A,[HL]
    INC HL
    ADD A,A
    CP 11
    JR C,next
    CP 200
    JR NC,next
    ADD A,E
    LD E,A
    JR NC,no_carry
    INC D
no_carry:
next:
    DJNZ loop
done:
    LD [result],DE
    RET
```

### map_store

Map and the pointer-advancing store body inline into one traversal with no callback call. The destination occupies IY, producing indexed stores; the u16 source count needs DEC DE plus a head zero test and branch. Hand code uses DE stores, CPL and DJNZ. Fusion is effective enough to beat SDCC on this workload, but indexed addressing/count control still costs ~1.90× hand T.

Nanz consumer:

```asm
execute:
    LD IY, target
    LD HL, source
    LD DE, (n)    ; u16 direct load, no scratch clobber
    LD BC, 0
.execute_fe_head1:
    LD A, D
    OR E
    JRS Z, .execute_fe_exit4
.execute_fe_body2:
    LD A, (HL)
    XOR 255
    LD (IY+0), A
    INC IY
    INC HL
    DEC DE
    JRS .execute_fe_head1
.execute_fe_exit4:
    LD DE, (n)    ; u16 direct load, no scratch clobber
    LD (result), DE    ; u16 direct store, no scratch clobber
    RET
```

Hand reference:

```asm
; Hand Z80 reference. Contract: 0 <= n <= 256; disjoint buffers.
ORG 0x8000
execute:
    LD HL,[n]
    LD A,H
    OR L
    LD B,L
    LD [result],HL
    RET Z
    LD HL,source
    LD DE,target
loop:
    LD A,[HL]
    INC HL
    CPL
    LD [DE],A
    INC DE
    DJNZ loop
    RET
```

### window_sum

There is no implemented skip/take operation to fuse here: explicit bound clamping and pointer offset precede a single fold loop. The Nanz loop widens every byte via a spill slot and saves BC around the word add; setup includes redundant stack/pair moves and repeat loads. Hand code clamps once, then uses byte carry accumulation. The result beats both SDCC profiles but costs ~3.70× hand T.

Nanz consumer:

```asm
execute:
    LD DE, 0
    LD HL, (n)    ; u16 direct load (20T, no A clobber)
    LD C, 4
    PUSH DE
    PUSH HL
    PUSH AF
    LD L, C
    LD H, 0
    POP AF
    EX (SP), HL
    PUSH HL
    POP DE
    POP HL
    OR A
    SBC HL, DE
    POP DE
    JRS NC, .execute_trmp0
.execute_if_then1:
    LD DE, (n)    ; u16 direct load, no scratch clobber
    LD HL, 4
    PUSH DE
    PUSH HL
    POP DE
    POP HL
    OR A
    SBC HL, DE
.execute_if_join2:
    LD DE, (limit)    ; u16 direct load, no scratch clobber
    PUSH HL
    OR A
    SBC HL, DE
    POP HL
    JRS Z, .execute_trmp1
    JRS C, .execute_trmp1
.execute_if_then3:
    LD BC, (limit)    ; u16 direct load, no scratch clobber
.execute_if_join4:
    LD HL, source
    LD DE, 4
    ADD HL, DE
    LD D, H
    LD E, L
    LD HL, 0
    LD IX, 0
.execute_fe_head5:
    LD A, B
    OR C
    JRS Z, .execute_trmp2
.execute_fe_body6:
    LD A, (DE)
    PUSH AF
    PUSH AF
    LD A, 0
    LD (_spill_execute_r33+1), A
    POP AF
    LD (_spill_execute_r33), A
    POP AF
    PUSH BC
    LD BC, (_spill_execute_r33)
    ADD HL, BC
    POP BC
    INC DE
    DEC BC
    JRS .execute_fe_head5
.execute_fe_exit8:
    LD (result), DE    ; u16 direct store, no scratch clobber
    RET
.execute_trmp0:
    LD HL, 0
    JRS .execute_if_join2
.execute_trmp1:
    LD B, H
    LD C, L
    JRS .execute_if_join4
.execute_trmp2:
    LD D, H
    LD E, L
    JRS .execute_fe_exit8
; — extra spill slots for execute (1 rescued)
_spill_execute_r33: DB 0, 0
```

Hand reference:

```asm
; Hand Z80 reference. Contract: 0 <= n <= 256; disjoint buffers.
ORG 0x8000
execute:
    LD HL,[n]
    LD DE,4
    OR A
    SBC HL,DE
    JR C,empty
    LD DE,[limit]
    OR A
    SBC HL,DE
    JR NC,use_limit
    ADD HL,DE
    JR have_count
use_limit:
    EX DE,HL
have_count:
    LD A,H
    OR L
    LD B,L
    JR Z,empty
    LD HL,source+4
    LD DE,0
loop:
    LD A,[HL]
    INC HL
    ADD A,E
    LD E,A
    JR NC,no_carry
    INC D
no_carry:
    DJNZ loop
done:
    LD [result],DE
    RET
empty:
    LD DE,0
    JR done
```

## Validation and unchanged-main control

`TestFusionBenchmarkCorpus` failed first with the driver patch reversed via `git diff > /tmp/f1-implementation.patch; git apply -R /tmp/f1-implementation.patch`: exit 1, `benchmark driver must expose --suite fusion`, with the unchanged origin/main help listing only main/heldout/all. Restoring with `git apply` made it pass. The final focused run also includes `TestRangeForEachStageOrderVMZ80` and `TestRangeForEachCapturedStagesVMZ80`; all pass (1.085 s). The new test validates all 24 source hashes and matching workload hashes, lengths 0/1/255/256, no residual iterator/simple-lambda CALL, and every Nanz frozen sample on raw MIR, final MIR and Z80 including PC/SP and full declared memory checks.

CI profile inventory passes: 881 entries, 510 fast, with the new test classified and the fast list sorted. `git diff --check` passes.

Required unit command:

```sh
cd minzc
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
nice -n 19 go test ./pkg/pipeline/ \
  -run 'TestFusionBenchmarkCorpus|TestRangeForEachStageOrderVMZ80|TestRangeForEachCapturedStagesVMZ80' -count=1
```

All required packages pass: mir2 111.136 s, hir 49.703 s, c89 101.556 s, nanz 137.263 s, z80asm 0.078 s, z80asm/regression 0.008 s. The full pipeline package was not run.

Bounded judge self-checks pass with zero findings: **direct-call seeds 0–499: 500/500; folded seeds 0–99: 100/100**. Both used four workers and `--no-reduce`, with the compiler built from this worktree. No larger fuzz campaign or assert_matrix was run.

```sh
nice -n 19 go build -o /mnt/ssd-510/minz/tmp/f1-mz ./cmd/minzc  # from minzc/
nice -n 19 python3 scripts/fuzz_diff.py --mz /mnt/ssd-510/minz/tmp/f1-mz \
  --mode direct-call --seed 0 --count 500 -j 4 --no-reduce --output /mnt/ssd-510/minz/bench/f1-direct
nice -n 19 python3 scripts/fuzz_diff.py --mz /mnt/ssd-510/minz/tmp/f1-mz \
  --mode folded --seed 0 --count 100 -j 4 --no-reduce --output /mnt/ssd-510/minz/bench/f1-folded
```

The validation transcript is retained [here](2026-10-22-f1-fusion-suite/validation.txt).

### Main suite before → after

Before used a separate detached origin/main worktree at `/mnt/ssd-510/minz/tmp/f1-origin-main`. After used the F1 worktree. Both ran the exact required control below, replacing PHASE with before/after:

```sh
nice -n 19 python3 research/c89-benchmark/run.py \
  --out /mnt/ssd-510/minz/bench/f1-PHASE --sdcc-root / --suite main --jobs 4
```

All 72 rows have identical statuses and, where available, identical source/workload hashes, call counts, occupied bytes and T. All 48 MinZ rows pass, including nograce. System SDCC retains its two existing struct_roundtrip compile errors. There is no bytes/T regression in any MinZ profile. [Before records](2026-10-22-f1-fusion-suite/main-before.json) and [after records](2026-10-22-f1-fusion-suite/main-after.json) are retained with their respective toolchain metadata. Each cell below is **bytes before→after / T before→after**.

| Main case | MinZ default | MinZ size | MinZ grace |
|---|---:|---:|---:|
| abs_diff | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 | 27→27 / 220,904→220,904 |
| bitwise8reg | 95→95 / 831,488→831,488 | 95→95 / 831,488→831,488 | 95→95 / 831,488→831,488 |
| copy_bytes | 172→172 / 109,332→109,332 | 172→172 / 109,332→109,332 | 172→172 / 109,332→109,332 |
| crc8 | 308→308 / 69,665,126→69,665,126 | 308→308 / 69,665,126→69,665,126 | 308→308 / 69,665,126→69,665,126 |
| fat12_decode | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 | 167→167 / 30,336→30,336 |
| gcd | 51→51 / 3,067,013→3,067,013 | 51→51 / 3,067,013→3,067,013 | 51→51 / 3,067,013→3,067,013 |
| memset_fill | 297→297 / 3,588,390→3,588,390 | 297→297 / 3,588,390→3,588,390 | 297→297 / 3,588,390→3,588,390 |
| mul8x8 | 42→42 / 948,736→948,736 | 42→42 / 948,736→948,736 | 42→42 / 948,736→948,736 |
| reverse_loop11 | 559→559 / 11,818,880→11,818,880 | 557→557 / 21,032,960→21,032,960 | 559→559 / 11,818,880→11,818,880 |
| strlen_bytes | 283→283 / 3,574,400→3,574,400 | 283→283 / 3,574,400→3,574,400 | 283→283 / 3,574,400→3,574,400 |
| struct_roundtrip | 59→59 / 567,296→567,296 | 59→59 / 567,296→567,296 | 59→59 / 567,296→567,296 |
| sum_array | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 | 313→313 / 7,581,952→7,581,952 |

The fusion suite is new, so there is no pre-existing fusion before row. Its cross-compiler measurement table above is a baseline for later compiler work.
