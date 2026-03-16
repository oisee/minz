# Report #089 — ObjC Canvas Demoscene, Multi-Frontend, CLI Standardization

**Date:** 2026-03-16
**Status:** Working features, shipped

---

## Summary

- **ObjC dynamic dispatch** via protocol vtables completed in prior session (commit `49247a8`)
- **Cross-language canvas library**: host functions in MIR2 VM (Go), C runtime for native compilation via QBE
- **Three demoscene effects**: plasma (sine interference), diamond (Manhattan distance), XOR fractal -- all rendering correct PNGs
- **All 8 frontends** wired into mzv (VM runner) and mzn (native compiler) via shared `parseSource()` dispatch
- **CLI standardized** to `--long-flag` / `-s` short convention via `github.com/spf13/pflag`
- **Auto-harness**: mzn generates `main()` for programs without one (runs static tests, saves canvas as PPM)
- **Native compilation verified**: ObjC `plasma.m` -> QBE -> x86-64 -> `plasma.ppm`

---

## Visual Showcase

| Effect | Description |
|--------|-------------|
| ![Plasma](../../examples/objc/output/plasma.png) | **Plasma** -- Sine wave interference, 256x192, rainbow palette |
| ![Diamond](../../examples/objc/output/diamond.png) | **Diamond** -- Manhattan distance animation |
| ![XOR](../../examples/objc/output/xor.png) | **XOR Fractal** -- Classic x^y demoscene pattern |
| ![Sphere](../../media/mzv_sphere_minz.png) | **Sphere** -- Raymarcher (Nanz, via mzv) |

---

## Canvas Architecture

```
Any frontend (.nanz/.c/.m/.lizp/.lanz/.plm/.pas/.abap)
  | parseSource() -- frontend dispatch by file extension
 HIR -> MIR2 (optimized)
  |
 +-----------------------------+
 | mzv (VM)                    |  canvas_* -> Go host functions -> PNG
 | mzn (QBE native)            |  canvas_* -> C runtime stubs -> PPM
 | go test (TestPlasmaRender)  |  canvas_* -> Go host -> PNG
 +-----------------------------+
```

The canvas API is uniform across all three execution modes:

| Function | Purpose |
|----------|---------|
| `canvas_init(w, h, mode)` | Initialize framebuffer |
| `canvas_clear(color)` | Fill with indexed color |
| `canvas_pixel(x, y, color)` | Set pixel |
| `canvas_palette(idx, r, g, b)` | Define palette entry |
| `canvas_line(x0, y0, x1, y1, c)` | Bresenham line |
| `canvas_rect(x, y, w, h, c)` | Rectangle outline |
| `canvas_circle(cx, cy, r, c)` | Midpoint circle |
| `canvas_save_ppm(path)` | Write PPM (native only) |
| `canvas_width()` / `canvas_height()` | Query dimensions |

---

## ObjC Effects Source

The plasma effect uses integer sine approximation (triangle wave) and ObjC protocol dispatch:

```objc
// Integer sine: triangle wave approximation, range -127..+127
int isin(int x) {
    int ix = x & 255;
    int quarter = ix & 63;
    int half_val;

    half_val = quarter * 2;

    if (ix < 64)  return half_val;
    if (ix < 128) return 127 - half_val;
    if (ix < 192) return 0 - half_val;
    return half_val - 127;
}

// Plasma class conforming to Effect protocol
@interface Plasma <Effect> {
    int scale;
    int speed;
}
-(int)render:(int)t;
@end

@implementation Plasma
-(int)render:(int)t {
    int w = canvas_width();
    int h = canvas_height();
    int y = 0;
    while (y < h) {
        int x = 0;
        while (x < w) {
            int v1 = isin(x * self->scale / 8 + t * self->speed);
            int v2 = icos(y * self->scale / 8 + t * self->speed / 2);
            int v3 = isin((x + y) * self->scale / 16 + t);
            int v4 = icos((x - y + 256) * self->scale / 16 + t / 2);

            int color = (v1 + v2 + v3 + v4 + 512) / 4;
            if (color < 0) color = 0;
            if (color > 255) color = 255;

            canvas_pixel(x, y, color);
            x = x + 1;
        }
        y = y + 1;
    }
    return 0;
}
@end
```

Dynamic dispatch is tested via `assert-objc-dyn` comments that exercise the protocol vtable path.

---

## CLI Standardization

Both mzv and mzn migrated from Go `flag` to `github.com/spf13/pflag`:

### mzv

| Before | After | Purpose |
|--------|-------|---------|
| `-trace` | `--trace` / `-t` | Print each VM call |
| `-headless` | `--headless` / `-H` | Run without terminal |
| `-max-frames N` | `--max-frames N` | Stop after N frames |
| *(new)* | `--dump-frames DIR` | Dump .scr files |

### mzn

| Before | After | Purpose |
|--------|-------|---------|
| `-c99` | `--c99` | C99 backend only |
| `-qbe` | `--qbe` | QBE backend only |
| `-emit-c` | `--emit-c` | Print generated C99 |
| `-emit-qbe` | `--emit-qbe` | Print QBE IL |
| `-o PATH` | `--output` / `-o` | Output binary path |
| `-run` | `--run` | Compile and run (default true) |
| *(new)* | `--disasm` / `-d` | Show disassembly |

---

## Multi-Frontend Support

All 8 frontends are now available in both mzv and mzn via a shared `parseSource()` function:

| Extension | Language | Parser |
|-----------|----------|--------|
| `.nanz` | Nanz | Native Participle |
| `.c` | C89 | modernc.org/cc/v4 |
| `.m` | ObjC | modernc.org/cc/v4 + ObjC lowerer |
| `.lanz` | Lanz | Native Participle |
| `.lizp` | Lizp | S-expression parser |
| `.plm` | PL/M | Custom PL/M parser |
| `.pas` | Pascal | Custom Pascal parser |
| `.abap` | ABAP | abaplint (Node.js bridge) |

Usage is identical across tools -- the file extension selects the frontend:

```bash
mzv program.nanz       # Nanz via VM
mzv program.c          # C89 via VM
mzv program.m          # ObjC via VM
mzn -o demo program.m  # ObjC -> QBE -> native binary
```

---

## Test Results

| Test | Result | Notes |
|------|--------|-------|
| `TestPlasmaRender` | PASS | 3 PNGs generated (plasma, diamond, xor) |
| `TestCanvas_HostFunctions` | PASS | pixel/line/rect/circle verified |
| Pre-existing test suite | No regressions | 26/26 Go test packages pass |
| Native compilation | PASS | `mzn -o plasma plasma.m` -> runs, produces PPM |

---

## Files Changed

### New files

| File | Purpose |
|------|---------|
| `minzc/pkg/mir2/canvas.go` | Canvas host functions for MIR2 VM (Go, PNG output) |
| `minzc/pkg/mir2/canvas_test.go` | TestCanvas_HostFunctions |
| `minzc/pkg/c89/plasma_render_test.go` | TestPlasmaRender (3 effects, E2E) |
| `examples/objc/plasma.m` | Plasma/Diamond/XOR demoscene effects |
| `examples/objc/canvas_shapes.m` | Canvas API shape tests |
| `examples/objc/dynamic.m` | ObjC dynamic dispatch via protocol vtable |
| `examples/objc/output/plasma.png` | Plasma effect output |
| `examples/objc/output/diamond.png` | Diamond pattern output |
| `examples/objc/output/xor.png` | XOR fractal output |

### Modified files

| File | Changes |
|------|---------|
| `minzc/cmd/mzv/main.go` | Multi-frontend `parseSource()`, pflag migration, canvas host wiring |
| `minzc/cmd/mzn/main.go` | Multi-frontend, pflag, canvas C runtime, auto-harness for mainless programs |
| `minzc/pkg/c89/objc_lower.go` | ObjC protocol vtable dispatch, canvas function declarations |
| `minzc/pkg/mir2/vm.go` | Canvas host function registration in VM |
| `minzc/pkg/mir2/module.go` | Canvas module linkage |
| `minzc/pkg/mir2/z80codegen.go` | Canvas extern stubs for Z80 backend |
| `minzc/pkg/pipeline/pipeline.go` | Frontend dispatch plumbing |

---

## What's Next

- **Live animated canvas** in mzv TUI: bridge canvas framebuffer to ANSI/sixel renderer for real-time demo playback
- **Dynamic dispatch vtables for native**: currently VM-only; mzn needs C codegen for vtable struct + indirect calls
- **SDL/Ebitengine window** for canvas programs: interactive mode with keyboard input and frame timing
