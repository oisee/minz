# Cascade replay experiments

This subdirectory has two distinct data formats:

- `seeds.bin` and `che_intro_incbin.asm` use the original 64-layer Fibonacci LFSR portrait. `gen_seeds.py` reproduces the checked-in 192-byte seed table.
- `replay.nanz`, `replay.asm`, `anim_player.nanz` and `anim_player.asm` use the later Galois LFSR (`0xB400`) AND-cascade format. The static table is a 16-bit count followed by seven bytes per seed: little-endian `seed`, `ox`, `oy`, `blk`, `and_n`, `warmup`. The animation form starts with an `ANMZ` header and frame table.

`render_seeds.py`, `gen_seeds_bin.py`, `gen_anim_bin.py` and `bake_replay.py` work with JSON seed data. The JSON used by the original GPU search lives in a neighboring project and is not checked in here. Without it, the Nanz replay and animation programs have no useful payload. `replay.nanz` still requires manual placement of seed bytes at its fixed address. `che_intro.nanz` in this directory currently compiles but fails assembly; use the root `che_intro.nanz` for an assembling Nanz variant.

To verify what builds with the current toolchain, run `./fun/check.sh` from the repository root. For the self-contained, handwritten portrait:

```sh
cd minzc
mkdir -p ../build
go build -o ../build/mza ./cmd/mza
cd ../fun/fun
../../build/mza che_intro_incbin.asm -o ../../build/che_intro.bin
```

`mze --target spectrum` can run the resulting binary if `mze` is installed. This build has not been compared to a golden screen image; assembly success alone does not validate the portrait. The previous README and Makefile, including obsolete `mz_vir` and `--vir` commands, are in [archive](../archive/).
