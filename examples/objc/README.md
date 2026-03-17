# ObjC Frontend Examples

MinZ's ObjC frontend compiles a subset of Objective-C through the
HIR -> MIR2 pipeline. Static dispatch by default; dynamic dispatch
via `@protocol` vtables when the receiver is protocol-typed.

## Quick Start (Mac)

```bash
cd minzc

# Compile and run via C99 backend
./mzn --c99 -o /tmp/plasma ../examples/objc/plasma.m && /tmp/plasma
open plasma.ppm   # view the output

# Or via QBE backend
./mzn --qbe -o /tmp/plasma ../examples/objc/plasma.m && /tmp/plasma

# Or via MIR2 VM (no native compiler needed)
./mzv ../examples/objc/plasma.m
```

## Examples

### plasma.m  --  Retro Demoscene Plasma

Renders a 256x192 plasma effect with ZX Spectrum palette. Three
different effects are implemented behind a shared `@protocol Effect`.

**Pipeline:** ObjC -> HIR -> MIR2 -> C99/QBE -> native binary -> plasma.ppm

#### Functions

| Function | What it does |
|----------|-------------|
| `isin(x)` | Integer sine approximation using a triangle wave. Input 0-255 maps to -127..+127 via four quadrants (rising, falling, negative falling, negative rising). No lookup table -- pure arithmetic. |
| `icos(x)` | Cosine via `isin(x + 64)` (quarter-wave phase shift). |
| `setup_plasma_palette()` | Builds a 256-color palette using three phase-offset sine waves for R, G, B. Each channel cycles smoothly: `r = isin(i) + 128`, `g = isin(i+85) + 128`, `b = isin(i+170) + 128`. |

#### Protocol and Classes

```objc
@protocol Effect
-(int)render:(int)t;    // render frame at time t
@end
```

| Class | `render:` algorithm |
|-------|-------------------|
| `Plasma` | Classic plasma: sums four sine/cosine waves at different frequencies and directions. `v1 = isin(x*scale)`, `v2 = icos(y*scale)`, `v3 = isin((x+y)*scale)`, `v4 = icos((x-y)*scale)`. The sum maps to a palette index 0-255. Fields: `scale` (frequency), `speed` (animation rate). |
| `Diamond` | Manhattan distance from screen center: `abs(x - w/2) + abs(y - h/2)`. Animated by adding `t*3`. Produces expanding diamond rings. Field: `spacing`. |
| `XorPattern` | The classic `(x ^ y) & 255` fractal pattern, animated by offsetting x and y by time. Produces the well-known Sierpinski-triangle-like texture. Field: `shift`. |

#### Test assertions (bottom of file)

The `assert-objc` and `assert-objc-dyn` comments are parsed by the
test harness. Static tests call methods directly; dynamic tests go
through the protocol vtable.

---

### canvas_shapes.m  --  Protocol-Based Shape Drawing

Demonstrates `@protocol Shape` with `draw` and `area` methods.

| Class | `draw` | `area` |
|-------|--------|--------|
| `Circle` | Calls `canvas_circle(cx, cy, radius, color)` | `3 * r * r` (integer pi) |
| `Rect` | Calls `canvas_rect(x, y, w, h, color)` | `w * h` |
| `FilledRect` | Calls `canvas_fill_rect(x, y, w, h, color)` | `w * h` |

Static dispatch: `[circle area]` compiles to `Circle_area(self)`.
Dynamic dispatch: `[(id<Shape>)shape area]` goes through the vtable.

---

### dynamic.m  --  Vtable Polymorphism

Minimal example showing how `@protocol` dispatch works.

```objc
@protocol Drawable
-(int)draw;
-(int)area;
@end

@interface Circle <Drawable> { int radius; }
@interface Rect <Drawable>   { int w; int h; }
```

- `Circle.area` returns `radius * radius`
- `Rect.area` returns `w * h`
- Static: `[circle draw]` -> direct `CALL Circle_draw`
- Dynamic: `[(id<Drawable>)obj draw]` -> vtable lookup + indirect call

---

## Canvas Host Functions

These are provided by the runtime (VM or C stubs):

| Function | Description |
|----------|-------------|
| `canvas_init(w, h, mode)` | Create framebuffer. Mode 0 = 256x192 (ZX Spectrum). |
| `canvas_clear(color)` | Fill with palette index. |
| `canvas_pixel(x, y, color)` | Set single pixel. |
| `canvas_line(x0, y0, x1, y1, c)` | Bresenham line. |
| `canvas_rect(x, y, w, h, c)` | Rectangle outline. |
| `canvas_fill_rect(x, y, w, h, c)` | Filled rectangle. |
| `canvas_circle(cx, cy, r, c)` | Circle (midpoint algorithm). |
| `canvas_palette(i, r, g, b)` | Set palette entry (256-color indexed). |
| `canvas_width()` / `canvas_height()` | Query dimensions. |
| `canvas_save_ppm(path)` | Save as PPM image (native only). |

## Other Examples

| File | Description |
|------|-------------|
| `simple.m` | Minimal class with ivar access |
| `counter.m` | Stateful object (increment/get) |
| `message.m` | Method calls with arguments |
| `inherit.m` | Single inheritance |
| `pipeline.m` | Method chaining |
| `foundation.m` | Foundation-like patterns |

## Output

Pre-rendered images in `output/`:
- `plasma.png` -- plasma effect at t=0
- `diamond.png` -- diamond distance pattern at t=0
- `xor.png` -- XOR fractal at t=0
