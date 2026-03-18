# FAT12 MIR2 Comparison: Nanz vs C89

**Date:** 2026-03-17
**Method:** Both libraries compiled through `mz --emit=mir2`, instruction counts compared per function.

## Summary

| Function | Nanz | C89 | Delta | Winner |
|----------|------|-----|-------|--------|
| `ld_word` | 9 | 9 | 0 | TIE |
| `st_word` | 10 | 8 | +2 | C89 |
| `read_fat12` | 15 | 16 | -1 | Nanz |
| `classify_fat12` | 21 | 21 | 0 | TIE |
| `clst2sect` | 9 | 9 | 0 | TIE |
| `is_deleted` | 11 | 11 | 0 | TIE |
| `sfn_checksum` | 18 | 19 | -1 | Nanz |
| `dbc_1st` | 2 | 2 | 0 | TIE |
| `dbc_2nd` | 2 | 2 | 0 | TIE |
| `chain_length/follow_chain` | 36 | 35 | +1 | C89 |

**Score: 6 TIE, 2 Nanz wins, 2 C89 wins.**

## Detailed Analysis

### `st_word` — C89 wins (8 vs 10, +2 instr)

**Root cause:** Nanz uses explicit `& 0xFF` masks, C89 uses `(BYTE)` casts.

Nanz source:
```nanz
let lo: u8 = val & 0xFF
let hi: u8 = hi_shifted & 0xFF
```

C89 source:
```c
ptr[0] = (BYTE)val;
ptr[1] = (BYTE)(val >> 8);
```

Nanz MIR2 (10 instr):
```
%r15 = const 255 : u16
%r16 = and %r14, %r15 : u16       // explicit mask
%r17 = const 8 : u16
%r18 = shr %r14, %r17 : u16
%r19 = const 255 : u16
%r20 = and %r18, %r19 : u16       // explicit mask again
...
```

C89 MIR2 (8 instr):
```
%r16 = trunc %r14 : u16 -> u8     // free on Z80 (just use L register)
%r19 = const 8 : i16
%r20 = shr %r14, %r19 : i16
%r21 = trunc %r20 : i16 -> u8     // free on Z80
...
```

**Takeaway:** `trunc` is cheaper than `const + and`. The Nanz frontend could emit `trunc` when assigning u16 to u8 instead of requiring explicit `& 0xFF`.

---

### `read_fat12` — Nanz wins (15 vs 16, -1 instr)

**Root cause:** C89 emits an extra `move` for pointer coercion before function call.

C89 MIR2:
```
%r95 = add %r89, %r94 : ptr
%r96 = move %r95 : ptr             // redundant move
%r97 = call @ld_word(%r96) : u16
```

Nanz MIR2:
```
%r30 = add %r25, %r29 : u16
%r31 = call @ld_word(%r30) : u16   // direct, no extra move
```

**Takeaway:** The C89 lowerer inserts a defensive move for `ptr` arguments. Could be eliminated by a copy-propagation pass or smarter lowering.

---

### `sfn_checksum` — Nanz wins (18 vs 19, -1 instr)

**Root cause:** C integer promotion rules force i16 arithmetic + truncation back to u8.

C89 MIR2 (loop body):
```
%r78 = add %r75, %r77 : i16       // i16 because of C integer promotion
%r82 = add %r78, %r81 : i16       // still i16
%r83 = trunc %r82 : i16 -> u8     // explicit truncation back to BYTE
```

Nanz MIR2 (loop body):
```
%r125 = add %r122, %r124 : u8     // stays u8 throughout
%r129 = add %r125, %r128 : u8     // no promotion, no truncation needed
```

**Takeaway:** Nanz keeps types narrow (u8 arithmetic stays u8). C's mandatory integer promotion to `int` means the C89 frontend must widen then truncate, costing 1 instruction per loop iteration.

---

### `chain_length` vs `follow_chain` — C89 wins (35 vs 36, -1 instr)

**Root cause:** Different loop idioms generate different control flow.

Nanz uses `while running != 0` with a flag variable:
```nanz
var running: u8 = 1
while running != 0 {
    if clst == 0     { running = 0 }
    if clst >= 0xFF8 { running = 0 }
    ...
}
```
This generates 5 `if-then-join` blocks, each threading the `running` flag as a block parameter through the chain. Each check = `br_if` + `const 0` + `jmp @if_join(0)` = 3 instructions.

C89 uses `do { ... break; } while(cond)`:
```c
do {
    if (clst == 0) break;
    if (clst >= 0xFF8) break;
    ...
} while (clst >= 2 && clst < 0xFF0);
```
Each `break` = `br_if` + `ret` = 2 instructions (direct return, no flag threading).

**Takeaway:** `break` in loops generates more efficient MIR2 than flag-variable patterns. Nanz could benefit from a `break` keyword or the loop optimizer could detect the flag-to-exit pattern and simplify it.

---

## Additional Nanz-only Functions

The Nanz library has ~500 more instructions in functions not present in the C89 version:

| Function | MIR2 instr | Purpose |
|----------|-----------|---------|
| `fat12_mount` | 9 | Parse BPB, init globals |
| `parse_bpb` | 78 | Full BPB field extraction |
| `load_sector` | 20 | Cached sector window |
| `ensure_fat` | 24 | FAT cache management |
| `fat12_next` | 26 | FAT chain following with cache |
| `sfn_match` | 19 | Compare 11-byte SFN names |
| `find_file` | 59 | Search root directory |
| `file_read` | 83 | Read file via cluster chain |
| `read_named_file` | 73 | Convenience: find + read |
| `count_dir_entries` | 57 | Count valid directory entries |
| `get_dir_entry` | 81 | Get Nth entry's name + cluster |

## Files

- Nanz source: `stdlib/fs/fat12.minz` (541 LOC)
- C89 source: `examples/c89/fatfs_lowlevel.c` (225 LOC)
- Differential tests: `minzc/pkg/c89/fatfs_differential_test.go`
- VM integration tests: `minzc/pkg/c89/fatfs_vm_test.go`
