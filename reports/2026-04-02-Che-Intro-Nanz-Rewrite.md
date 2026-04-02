# Che Intro Nanz Rewrite

Date: 2026-04-02

## Goal

Rewrite `fun/che_intro.nanz` into a more idiomatic Nanz shape that matches the active MIR2/VIR backend better:

- static global data instead of runtime `store_layer(...)` population
- direct array indexing instead of manual byte-pointer wiring in source
- a small mask LUT instead of a per-pixel shift loop

## Why

The previous version expressed the layer table as:

- `global data_addr: u16 = 0xC000`
- `store_layer(idx, seed, points)`
- `init_layers()` with 64 calls
- `load_seed(idx)` / `load_points(idx)` via manual `^u8` pointer math

That is workable, but it is not the cleanest source form for Nanz now that the current backend can handle typed static global data more directly.

The more idiomatic source form is:

- `global layer_seeds: [u16; 64] = [...]`
- `global layer_points: [u8; 64] = [...]`
- `global bit_masks: [u8; 8] = [128,64,32,16,8,4,2,1]`

Then the hot-path source becomes:

- `seed = layer_seeds[layer_idx]`
- `points = layer_points[layer_idx]`
- `mask = bit_masks[x % 8]`

This is simpler, more declarative, and lines up with the backend's data-section strengths.

## Address-Of Note

In the active HIR/MIR2 pipeline, indexed addresses are supported as addresses, not only loads.
So `p = &data[i]` is part of the intended model, not just `p = &data[2]`.

## Rewrite Scope

The rewrite should:

- remove `data_addr`, `store_layer`, `load_seed`, `load_points`, and `init_layers`
- replace them with typed global arrays
- replace the bit-mask loop in `xor_pixel` with LUT indexing
- keep the rest of the algorithm structurally the same

## Expected Result

Source should become smaller and clearer, while generated code gets a better chance to map onto:

- `DW` streams for seeds
- `DB` streams for points
- a small `DB` mask LUT

without a runtime table-construction phase in user code.
