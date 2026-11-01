# Compiler findings from task Z3 — repros

All repros are run from this directory with the freshly built compiler and
assembler (`minzc/mz`, `minzc/mza`). None of them touches `minzc/pkg/`; they are
recorded here because task Z3 says not to change the compiler.

| # | repro | one-liner | severity for this task |
|---|---|---|---|
| A | `asm_global_main.nanz` (+ `leaf.nanz`, `mid.nanz`) | a hand-written `asm z80` operand cannot name a module global portably, because the compiler prefixes every symbol of an imported module with the mangled name of the *importing* module | **blocker** — hit directly, worked around |
| B | `halfreg_shift.nanz` | `SRL IYL` / `RR IYH` are rejected by the Z80-VALIDATE pass although they are real Z80 instructions the emulator executes | medium — worked around |
| C | `const_array_size.nanz` | an array size must be a literal; a `const` name is a parse error | cosmetic |
| D | `asm_line_comment.nanz` | a `//` comment inside an `asm z80` block is emitted as an instruction line | cosmetic |
| E | `module_dup/` | importing one module through two paths emits its code twice | **blocker** for the 48K memory budget — hit directly, worked around |

## A — asm cannot spell an imported symbol

```sh
$ minzc/mz asm_global_main.nanz -o /tmp/a.a80
$ minzc/mza /tmp/a.a80 -o /tmp/a.bin
Assembly errors:
  line 22: unknown instruction or invalid operands: LD
```

`leaf.nanz` contains `LD HL, leaf__table`. When `mid.nanz` imports `leaf`, the
label is emitted as `mid__leaf__table`, but the literal operand inside `leaf`'s
asm body still says `leaf__table`, so the load can never be encoded. The name a
module must spell for its own global therefore depends on who imports it — which
the module cannot know.

*Workaround used:* `stdlib/zx/narrow_font.nanz` passes the table address into
`nf_build_slots` as a parameter, and `stdlib/zx/text.nanz` caches the font and
shift-table addresses in its own globals (`tx_font`, `tx_shift`) so its asm
bodies only ever name `zx__text__*` symbols. That is still not fully portable —
it holds while `zx.text` is imported by the main module — but it covers this
task and the zvdb port.

## B — half-register shifts fail validation

```sh
$ minzc/mz halfreg_shift.nanz -o /tmp/b.a80
[Z80-VALIDATE] rot_right: 4 invalid instruction(s)
  line 5: >>> SRL IYL <<<
  line 6: >>> RR IYH <<<
```

`SRL IYL` and `RR IYH` are the standard DD/FDCB forms; the emulator runs them
(100% FUSE coverage). The validator's table apparently only knows the
`(IX+d)`/`(IY+d)` memory forms. *Workaround:* rotate in `BC`
(`SRL C` / `RR B`), which is what `tx_blit_rot` does.

## C — array sizes must be literals

```sh
$ minzc/mz const_array_size.nanz -o /tmp/c.a80
Error: Nanz parse: line 10: expected token kind 2, got "NWIN"
```

Same family as the known "a const whose initialiser references another const
reads back 0": derived values have to be spelled out. `stdlib/zx/text.nanz`
therefore declares every window array as `[u8; 4]` with a comment instead of
`[u8; TX_NWIN]`.

## D — `//` comments inside asm blocks

```sh
$ minzc/mz asm_line_comment.nanz -o /tmp/d.a80
$ minzc/mza /tmp/d.a80 -o /tmp/d.bin
Assembly errors:
  line 23: unknown instruction or invalid operands: INCREMENT
```

The `//` marker is stripped and the comment text is re-emitted as its own line,
which mza then tries to assemble. A `;` comment on an instruction line survives
(the whole `stdlib/zx` layer uses those); a standalone `;` comment line loses its
marker too, so all asm comments in `stdlib/zx` are trailing comments.

## E — imported code is duplicated per importer

```sh
$ minzc/mz module_dup/main_direct.nanz      -o /tmp/direct.a80
$ minzc/mz module_dup/main_via_wrapper.nanz -o /tmp/wrapped.a80
$ grep -c 'leaf_work:' /tmp/main_direct.a80      # 1
$ grep -c 'leaf_work:' /tmp/main_via_wrapper.a80 # 2
```

Importing the same module through two paths emits its functions twice. Task Z3
hit this with real numbers: `examples/nanz/zx_narrow_text_bench.nanz` importing
both `zx.text` and `zx.narrow_font` produced a 33013-byte image (which no longer
fits below `$FFFF when loaded at $8000`) against 18261 bytes once it imported
only `zx.text` and reached the builder through `text_build_font()`. The 48K
memory budget is exactly where this bites, since the engine already spends
13.5 KB on font tables.
