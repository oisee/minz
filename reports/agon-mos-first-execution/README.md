# Agon MOS first executable observation

Fab Agon Emulator 1.2.5 CLI boots real Console8 MOS 2.3.3 on its eZ80 CPU. CLI VDP is explicitly fake; graphics and hardware unproven. Upstream https://github.com/tomm/fab-agon-emulator; downloaded archive SHA256 9308da450c9e6532ec4070342d859bacb3996f14a9c31327e49e857c3ce32bbe (see share research-tools/fab-agon).

Exact frozen hello_agon BIN (119 bytes, origin/main 0x40045) loads, but MOS RUN &40045 returns **Invalid executable**. Host exit0 is not program PASS: subsequent echo proves only MOS remains responsive. No Hello output occurred. This is a new product/runtime gap; earlier assembler acceptance cannot establish MOS executable admission. Investigate MOS header and entry wrapper before changing code or claiming execution.

Input/output/error and exact BIN retained. No corpus status upgraded. Full pipeline-b6b4e0c7 continues independently.
