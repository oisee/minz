# Fun / Frontend / Runtime Status

**Дата:** 2026-04-06  
**Цель:** не showcase-маркетинг, а честный operational snapshot:

- что реально сейчас собирается на живых representative примерах
- что проходит `--asserts mir2`
- что реально запускается под `mze` / `mzv` / `mzx`
- где на выходе уже красивый asm

## Scope

Это **не полный sweep всего репозитория**.  
Это curated matrix по 6 high-signal файлам:

- `fun/bit_intent.nanz`
- `fun/pointer_threading.nanz`
- `fun/frill_showcase.frl`
- `examples/lizp/showcase.lizp`
- `examples/pascal/assert_test.pas`
- `examples/objc/simple.m`

И отдельные runtime probes:

- `examples/nanz/tetris_tui.nanz` via `mzv -H`
- `tetris_cpm` via `mze`
- `che_cascade` via `mzx --headless`

## Build / Assert Matrix

Команды запускались из `minzc/` через `go run ./cmd/minzc ...`.

| File | Frontend | Command | Result | Notes |
|------|----------|---------|--------|-------|
| `fun/bit_intent.nanz` | Nanz | `go run ./cmd/minzc ../fun/bit_intent.nanz --asserts mir2` | PASS | `vir: all 5 functions compiled via Z3 unified solver` |
| `fun/pointer_threading.nanz` | Nanz | `go run ./cmd/minzc ../fun/pointer_threading.nanz --asserts mir2` | PASS | `sum_neighbors` падает в PBQP fallback, но file overall compiles and verifies |
| `fun/frill_showcase.frl` | Frill | `go run ./cmd/minzc ../fun/frill_showcase.frl --asserts mir2` | PASS | несколько recursive/call-heavy функций идут через PBQP fallback, но итоговый file OK |
| `examples/lizp/showcase.lizp` | Lizp | `go run ./cmd/minzc ../examples/lizp/showcase.lizp --asserts mir2` | FAIL | label audit: `inc_`, `zx_poke` undefined |
| `examples/pascal/assert_test.pas` | Pascal | `go run ./cmd/minzc ../examples/pascal/assert_test.pas --asserts mir2` | PASS | `vir: all 3 functions compiled via Z3 unified solver` |
| `examples/objc/simple.m` | ObjC | `go run ./cmd/minzc ../examples/objc/simple.m --asserts mir2` | PASS | `vir: all 6 functions compiled via Z3 unified solver` |

### Snapshot

- PASS: `5 / 6`
- FAIL: `1 / 6`
- current obvious frontend/runtime integration hole in this sample: **Lizp showcase**

## Runtime Probes

### 1. `mzv -H examples/nanz/tetris_tui.nanz`

Command:

```bash
timeout 2 mzv -H examples/nanz/tetris_tui.nanz
```

Result:

- PASS
- headless output shows:
  - full board frame
  - score/lines/next/hold UI
  - colored cells rendered
  - terminal eventually reaches `GAME OVER!`

Interpretation:

- `mzv` path for the TUI Tetris is alive
- this is already a real visual/runtime demo, not just compile-only proof

### 2. `mze` + current `tetris_cpm`

Command:

```bash
printf 'qq' | go run ./cmd/mze /tmp/tetris_cpm_regress.com -t cpm | xxd -g 1
```

Output:

```text
00000000: 30 30 30 30 60                                   0000`
```

Result:

- mixed / still not visually correct
- the binary now compiles and runs, but CP/M-side visible output is still not in a good state

Interpretation:

- this is progress compared to earlier compile blockers
- but `tetris_cpm` under `mze` is still a backend/runtime correctness target, not a finished demo

### 3. `mzx --headless` + `che_cascade`

Command:

```bash
timeout 3 mzx --headless --load /home/alice/dev/minz-vir/build/che_cas.bin@8000 \
  --set PC=81E4,SP=FFFF,DI,IM=1 \
  --frames DI:HALT \
  --dump-keyframes /home/alice/dev/minz-vir/build/che_status_kf
```

Result:

- FAIL on current installed path
- current error:

```text
glfw: X11: The DISPLAY environment variable is missing
panic: glfw: The GLFW library is not initialized
```

Interpretation:

- the current `mzx` binary/path is still not honestly headless-stable here
- for `che_cascade`, runtime status via `mzx` remains inconclusive on this machine/path

## Good ASM Worth Showing

### 1. `bit_intent.nanz`

Source:

- [fun/bit_intent.nanz](/home/alice/dev/minz-vir/fun/bit_intent.nanz)

Representative source shape:

```nanz
var p: ptr
p = &flags_g
p^.4 = 1
if p^.7 != 0 { return 42 }
```

This one already shows the expected direct bit ops:

```asm
SET 1, H
RES 1, H
SET 4, H
SET 7, B
BIT 7, C
JR NZ, .branch_on_ptr_bit_if_then1
```

What this proves:

- bit intent survives far enough to become `BIT/SET/RES`
- register-backed bit intent is one of the cleanest current “backend did the right thing” showcases
- memory-backed bit intent still needs a separate quality audit and is not claimed here as showcase-quality

### 2. `pointer_threading.nanz`

Source:

- [fun/pointer_threading.nanz](/home/alice/dev/minz-vir/fun/pointer_threading.nanz)

Representative source shape:

```nanz
while i < n {
    let a: u8 = data_g[i]
    let b: u8 = data_g[i + 1]
    acc = acc + a + b
    i = i + 1
}
```

The interesting thing here is not that the whole asm is perfect; it is that the trace shows the transform actually firing on real source:

```asm
; [trace] backend=VIR passes=[dead-block-arg=1,split-join-ret=1,condret-sink=1,ptr-threading=1,cond-rets=1]
```

That appears on both functions in the generated output.

What this proves:

- `ptr-threading` is no longer just unit-test fiction
- it really triggers on source code

### 3. `examples/objc/simple.m`

Source:

- [examples/objc/simple.m](/home/alice/dev/minz-vir/examples/objc/simple.m)

Representative source shape:

```objc
@interface Box { int value; }
- (int)get;
- (int)addN:(int)n;
@end
```

What is still worth showing here is the source-level intent:

- ObjC methods lower to direct functions over plain data
- there is no “runtime theater” requirement for simple field access

But the current emitted asm is not clean enough to use as a showcase snippet yet:

- top-of-file ABI summary comments in the emitted `.a80` are stale (`BC/HL`) and contradict the actual function-local comments (`HL/DE`)
- `Box_get` still carries an extra `DE` copy, so the snippet is correct-looking but not elegant enough to feature as “good asm”

Current defensible verdict:

- ObjC here is genuinely “syntax over direct calls and struct access”
- but this particular asm output should be treated as “interesting and mostly direct”, not as a polished showcase of backend quality

## Main Takeaways

1. The current strongest compile/showcase combo is:
- `bit_intent.nanz`
- `pointer_threading.nanz`
- `frill_showcase.frl`
- `examples/pascal/assert_test.pas`
- `examples/objc/simple.m`

2. The current most obvious broken representative in this small matrix is:
- `examples/lizp/showcase.lizp`

3. The current best runtime-visible success is:
- `mzv -H examples/nanz/tetris_tui.nanz`

4. The current still-not-finished runtime targets are:
- `tetris_cpm` under `mze`
- `che_cascade` under current `mzx --headless` path

## What Next

The next useful operational moves are clear:

1. fix `examples/lizp/showcase.lizp` label/runtime integration (`inc_`, `zx_poke`)
2. keep using `tetris_cpm` as backend/runtime regression target for `mze`
3. either fix current `mzx` headless path or stop pretending `che_cascade` is already a stable emulator demo
4. if desired later, run a separate **full corpus sweep script** instead of pretending this small matrix already covers everything
