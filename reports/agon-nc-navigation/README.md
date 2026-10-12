# Mini NC: boundary Up, Enter and ESC under MOS

All three CLI configurations match the independent 1024-byte ANSI stream: initial eight-file listing, Up at selection 0 (no underflow/redraw), Enter displaying HELLO.BIN, then ESC cleanup and clean MOS continuation. Source and library unchanged. The fake CLI sends CR after each input line, so stdin VT then ESC corresponds to keys 11,13,27; the CR after ESC is cleared by MOS. Exact streams and transcripts retained.

This does not prove down/navigation across the whole list. CLI line input cannot directly inject LF as a key without its newline translation. Real VDP rendering and full RAM/SP are separate. Compiler and firmware hashes are recorded in reports/agon-entry-console-replay/summary.json. Independent model: scripts/agon_tui_stream_model.py.
