# Historical `fun/` material

These files are retained for provenance, not as current build instructions.

- `README-previous.md` made unverified claims about every example, generated instruction sequences, performance, a complete SHA-256, and a 64-layer `che_cascade.nanz` portrait.
- `README-cascade-previous.md` mixed two LFSR/data formats and used obsolete `--vir` commands and external JSON paths as a quick start.
- `Makefile-cascade-previous` invokes the removed `mz_vir` executable and `--vir` flag; its Nanz target also currently fails assembly.
- `che_cascade.asm` contains `DJNZ` operands rejected by the current assembler. The active `che_cascade.nanz` does compile and assemble, but its image has not been compared with a reference.

See [the current showcase README](../README.md) and run `../check.sh` for build status.
