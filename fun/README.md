# `fun/`: compiler showcases and graphics experiments

This directory mixes small language examples with ZX Spectrum graphics experiments. Use `./fun/check.sh` from the repository root to compile every Nanz and Frill source, run source assertions requested through `--asserts mir2`, and assemble the output. It also assembles the handwritten sources. The script records current assembler blockers and fails on new failures or when a recorded blocker is fixed and needs removal from its list.

The check is **a build check**, not a proof that every program behaves correctly on a Spectrum. Assertions cover only cases present in the source. In particular, the graphics examples have no golden screen or frame comparison. As of 2026-10-01, 28 Nanz and 3 Frill sources compile; 27 of those assemble, while four have known blockers. The six active handwritten assembly files assemble. Re-run the script after compiler changes rather than relying on these counts.

## Where to start

| Track | Files | What they demonstrate | Current evidence |
| --- | --- | --- | --- |
| Values and control flow | `adt_option.nanz`, `state_machine.nanz`, `tuple_return.nanz`, `triple_return_skip.nanz`, `tail_recursion.nanz` | ADTs, matching, tuples, state transitions, recursion | Compile and assemble; source assertions where present |
| Memory and data layout | `addr_*.nanz`, `bit_intent.nanz`, `pointer_threading.nanz`, `oop_shapes.nanz`, `vectors.nanz` | Addresses, bit access, interfaces and structured values | `vectors.nanz` currently fails assembly; the others assemble |
| Arithmetic and composition | `widemath.nanz`, `pipes_nanz.nanz`, `iterator_fusion.nanz`, `pipes.frl`, `frill_showcase.frl` | Widths, overloaded operators, pipes and iteration | Compile and assemble; performance claims need measurements |
| Graphics | `che_cascade.nanz`, `che_intro.nanz`, `che_nanz.nanz`, `lfsr_decoder.nanz`, `frill_graphics.frl`, `raymarcher.nanz`, `fun/` | LFSR layers, screen writes, fixed-point SDFs and pattern functions | Most assemble; `raymarcher.nanz` does not. No golden image check |
| Other experiments | `sha256.nanz`, `irc_client.nanz`, `import_demo/` | 32-bit helpers, IRC client, imports | `sha256.nanz` contains SHA-256 building blocks, **not a hash implementation**. `irc_client.nanz` fails assembly |

`fun/fun/` is a separate Che/cascade replay experiment with its own [README](fun/README.md). Its Nanz replay and animation sources assemble, but require seed or animation payloads for a meaningful run. `fun/fun/che_intro.nanz` currently generates invalid Z80 assembly. The handwritten `fun/fun/*.asm` variants do assemble.

The old showcase README, old cascade README and Makefile, and the non-assembling handwritten `che_cascade.asm` are in [archive](archive/). Their performance, image and runtime claims are historical and have not been revalidated.

## Next improvements

1. **Quick win:** Fix the four assembly blockers listed by `check.sh`; keep each source as a regression case. The generated failures include invalid `LD` operands and duplicate spill labels in `vectors.nanz`.
2. **Foundation:** Add a deterministic golden bitmap for one fixed `che_cascade.nanz` input, compare it against an independent CPU reference, and only then claim image equivalence.
3. **Foundation:** Make Nanz replay and animation payload embedding reproducible from checked-in data. Today their source compiles but does not bake external JSON into the binary.
4. **Later:** Measure generated code size and cycles before asserting that an example is optimal or zero-cost. `raymarcher.nanz` also needs an image oracle after its backend blocker is fixed.

Music examples are outside this cleanup.
