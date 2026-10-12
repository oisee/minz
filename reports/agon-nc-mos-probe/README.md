# Independent Agon TUI stream check

The model scripts/agon_tui_stream_model.py is written from source calls, literal strings, dimensions and library decimal formatting, not copied from generated output. TUI Hello produces 313 bytes, TUI Demo 707 bytes; ordinary/HIR-Grace Size/HIR-Grace Speed all match exactly and return cleanly to MOS. Mini NC initial listing plus immediate ESC produces 976 bytes, currently checked in ordinary only. No navigation claim.

Fab CLI VDP reports each ESC as `Unknown packet VDU 0x1b` followed by a newline. The decoder inverts only this known diagnostic inside the run/return boundaries. It rejects any other unsupported packet and repeated MOS boot. It preserves every other byte; expected and observed streams are retained. This proves emitted ANSI stream and clean MOS continuation, not rendering by real VDP, RAM/SP or hardware.

Compiler: minzc-agon-entry, build/hash in ../agon-entry-fix/compiler-build.json. Emulator/firmware pins in ../agon-entry-console-replay/summary.json. Exact inputs and public CLI ASM/BIN retained; source files unmodified. All artifacts saved on share.
