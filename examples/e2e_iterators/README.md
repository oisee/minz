# Iterator E2E Tests (MZX --console-io)

End-to-end tests that compile MinZ iterator chains and run them in the
ZX Spectrum emulator with bare-metal console I/O (port $23).

## Working E2E

| Test | Chain | Expected | Status |
|------|-------|----------|--------|
| `iter_foreach.minz` | `arr.forEach(console_log)` | `ABCDE\n` | PASS |
| `iter_take.minz` | `arr.iter().take(3).forEach(console_log)` | `ABC\n` | PASS |

## Compile-correct, Z80 codegen WIP

These compile through the full parser+semantic pipeline (our Argument field
fix works) but produce wrong runtime output due to pre-existing Z80 codegen
bugs (register allocator, function calling convention):

| Test | Chain | Issue |
|------|-------|-------|
| `iter_skip.minz` | `.skip(2).forEach()` | Skip pointer offset optimized away |
| `iter_filter_foreach.minz` | `.filter(is_big).forEach()` | Filter predicate value leaks to output |
| `iter_map_foreach.minz` | `.map(double).forEach()` | Register clobbering in chained calls |
| `iter_lambda_map.minz` | `.map(\|x\| x+1).forEach()` | Lambda call convention mismatch |

## How to run

```bash
cd /path/to/minz-ts

# Compile → assemble → run
minzc/mz examples/e2e_iterators/iter_foreach.minz -o /tmp/t.a80
minzc/mza /tmp/t.a80 -o /tmp/t.bin
minzc/mzx --run /tmp/t.bin@8000 --frames DI:HALT --console-io
```

## Array literal workaround

Array literal codegen is WIP — the DJNZ loop reads the array pointer from
SMC address `$F002`. Each example patches this address with an inline asm
block pointing to a `DB` data section:

```minz
asm {
    LD HL, _data
    LD ($F002), HL
}
let arr: [u8; 5] = [65, 66, 67, 68, 69];
arr.forEach(console_log);
asm {
    _data:
        DB 65, 66, 67, 68, 69
}
```
