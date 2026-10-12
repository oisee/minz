# Agon MOS console: two actual CLI programs, three configurations

All six executions PASS: exact contiguous program output, key input and subsequent MOS prompt/echo. hello_agon is 188 B; hello_world imports stdlib.agon.mos and is 266 B. All three products per source have identical BIN hashes: this matrix proves compatibility, not an optimization win.

Configurations are ordinary, --hir-grace --Osize, and --hir-grace (speed policy). CLI compiler is the retained minzc-99ab4b54 executable. Emulator is Fab Agon Emulator 1.2.5 CLI, real eZ80 CPU with Console8 MOS 2.3.3 firmware. Tool hashes and exact commands are in summary.json and each result.json.

The oracle comes directly from the two source programs: their literal messages, fixed input key a, and successful subsequent MOS echo. stdout contains emulator startup diagnostics outside that exact body. The VDP is fake/minimal; no graphics, full memory/stack oracle, arbitrary key stream or real hardware claim. Remaining three graphical Agon sources are still pending suitable runtime proof. Catalog statuses have not been upgraded from this console smoke alone.

Reproduce with scripts/agon_mos_console_smoke.py --compiler <minzc> --emulator <agon-cli-emulator> --mos <mos_console8.bin> --output <new-directory>. The output directory must be new. Commands have compilation and execution timeouts and retain diagnostics on oracle failures. This is research tooling, not a compiler runtime dependency.

Each case retains exact source, CLI ASM/BIN, compile logs, input/output, result and SD-card executable. Imported MOS source is retained at the package root. The external emulator release archive and provenance are on /mnt/safe/minz/grace/2026-10-12/research-tools/fab-agon/.
