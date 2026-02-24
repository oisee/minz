# MZE: FUSE Z80 Test Suite — 1335/1335 Tests Pass

**Status:** Complete (v0.18.0)
**Commit:** `0d89e5b`

## Summary

The MZE Z80 emulator achieved 100% pass rate on the FUSE (Free Unix Spectrum Emulator) test suite — 1335 test vectors covering every Z80 opcode including all undocumented instructions. This is the gold standard for Z80 emulator accuracy.

## Test Data

| File | Lines | Content |
|------|-------|---------|
| `fuse_tests.in` | 9,010 | Input: registers + memory setup per test |
| `fuse_tests.expected` | 18,395 | Expected: final registers + memory + events |

**Source:** libz80 FUSE test suite (github.com/ggambetta/libz80)

## Test Format

### Input
```
test_name
AF BC DE HL AF' BC' DE' HL' IX IY SP PC    # 12 hex values
I R IFF1 IFF2 IM halted tstates             # 7 values
addr byte1 byte2... -1                       # memory blocks
-1                                           # end marker
```

### Expected Output
```
test_name
    0 MC 0000                                # events (skipped in our test)
    4 MR 0000 10
AF BC DE HL AF' BC' DE' HL' IX IY SP PC
I R IFF1 IFF2 IM halted tstates
addr byte1 byte2... -1                       # changed memory only
-1
```

### Execution Model
```go
for cpu.Tstates < eventNextEvent {
    cpu.DoOpcode()
}
// Compare all registers + memory against expected
```

## Critical Memory Interface Semantics

Getting these right was the key to passing all 1335 tests:

| Method | T-states Added | Usage |
|--------|---------------|-------|
| `ReadByte(addr)` | **+3** | Normal memory access during instruction execution |
| `WriteByte(addr, val)` | **+3** | Normal memory write |
| `ReadByteInternal(addr)` | **0** | Raw access, no timing (used after ContendRead) |
| `WriteByteInternal(addr, val)` | **0** | Raw access, no timing |
| `ContendRead(addr, time)` | **+time** | Memory contention delay |
| `ReadPort(addr)` | **+4** total | 1 pre + 3 post T-states |
| `WritePort(addr, val)` | **+4** total | 1 pre + 3 post T-states |

**Port read convention:** Returns `byte(address >> 8)` (FUSE floating bus).

## Test Coverage by Category

| Category | Count | Description |
|----------|-------|-------------|
| Main opcodes (00-FF) | 256 | All standard 8-bit instructions |
| CB prefix | ~100 | Bit operations (rotate, shift, test, set, reset) |
| DD prefix (IX) | ~43 | IX-relative addressing and arithmetic |
| DDCB prefix | ~100 | Bit operations on (IX+d) |
| ED prefix | ~39 | Block I/O, block search, 16-bit arithmetic |
| FD prefix (IY) | ~42 | IY-relative addressing and arithmetic |
| FDCB prefix | ~100 | Bit operations on (IY+d) |
| Multi-instruction | varies | DJNZ loops, LDIR repeats (higher tstates) |
| Undocumented | ~50+ | IXH/IXL/IYH/IYL ops, SLL, etc. |

## Key Design Decisions

1. **Event lines are parsed but ignored** — we validate final state only, not intermediate bus events
2. **R register split** — bit 7 in `R7`, bits 0-6 in `R` (careful reconstruction needed)
3. **Most tests have `eventNextEvent=1`** — runs exactly one instruction (all Z80 ops are >=4T)
4. **remogatto/z80's own `z80_test.go` is BROKEN** (doesn't compile) — we use FUSE instead
5. **This suite validates MZX's CPU core** — same remogatto/z80 library, same timing guarantees
