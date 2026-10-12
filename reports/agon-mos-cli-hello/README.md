# Agon Hello: public CLI executable runs through MOS

Current CLI compiled exact hello_agon.nanz with `-b ez80 -o hellocli.bin`: 188 bytes, MOS header plus 119-byte payload. Fab Agon Emulator 1.2.5 CLI/Console8 MOS 2.3.3 loaded it at default &40000, RUN printed the exact expected text (CR/LF normalized by CLI VDP), consumed a key, returned to MOS prompt and executed the following echo. Host exit0 alone was not used as oracle.

Correction to first observation: MOS correctly rejected the headerless research payload, not the public CLI product. pipeline.Assemble already applies generateAgonBin; no backend fix was required. Raw payload was deliberately assembled directly in product audit, and is not itself a MOS executable.

This is a scoped ordinary-mode eZ80/MOS execution smoke. CLI VDP is fake; no graphics, all-modes corpus, full RAM/SP, hardware claim. Source, actual CLI BIN, input, output and hashes retained. Remaining four Agon sources and richer effects still need suitable oracles.
