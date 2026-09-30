# Small Z3 conformance fixtures

These original MIT-licensed stories exercise Z3 behavior missing from CZECH v0.8. Run them with `./smoke.sh /path/to/mzv` from any directory. The expected text was checked against `dfrotz` 2.44; the save and command-stream fixtures use `mzv`'s fixed local filenames instead of Frotz's filename prompts.

| Fixture | Behavior |
|---|---|
| `stream3` | ZSCII newline in a memory table and nested memory streams |
| `transcript` | Output stream 2 selected and deselected |
| `unicode` | Default Z3 ZSCII extra characters rendered as UTF-8 on screen and in transcript |
| `command_stream` | Output stream 4 records a command; input stream 1 replays it and falls back to keyboard at EOF |
| `save_restore` | Dynamic memory and PC resume at the save branch; another story release cannot restore the save |
| `high_memory.py` | Generates a 131,070-byte story whose routine and Z-text lie near the 128 KiB limit |

The `.inf` files and committed `.z3` binaries were built with Inform 6.45. To rebuild one story, run `inform -v3 stream3.inf stream3.z3` (substitute its basename). `high_memory.py` generates its story at test time, so no large binary is committed.
