# String repair: final frozen source review

Read `c9c3e1dc` and independently verified all nine entries in `docs/grace-string-representation/source-inventory.json`: five production paths and four focused test files. No CPU execution or tests repeated by this reviewer.

The generated-name hygiene now skips type descriptors, guards visited AST pointers, and reserves Name/Var/Names bindings plus globals and live aliases. The real lowering fixture includes a recursive typed-pointer parameter and verifies MIR, closing the graph-recursion issue. Counted S/L output still uses ordinary loop/load/call allocation and evaluates the argument once. Byte-output builtin errors propagate.

VM representation allocation checks total storage against its configured memory limit before modifying heap or cache. Tests distinguish representable L length 65535 from a 65537-byte encoded object that cannot fit the default 64 KiB heap. Parser and native emitter representation limits and empty syntax remain narrow. Opaque pointers retain CString behavior; arbitrary dynamic string-format inference is not claimed.

The native harness is suitable for one bounded release: twelve automatic-PBQP output cases and three storage/dereference cases, including C/S/L, empty formats, UTF-8 bytes, embedded NUL, S255 and L256. It checks exact output plus a non-tail live-value successor, unchanged colliding-name globals, and actual PC/SP termination. Storage tests independently compare assembled bytes and first-byte loads. The harness uses ordinary native emulator output, not widened closed-host permissions or fabricated allocation.

No source-review blocker remains to the parent-authorized `TestLiteralPrintingNative|TestLiteralStorageNative` run under a preserved test executable. This is not a final successful CP/M main claim: separately corrected CString source and contiguous COM mapping remain required for that smoke artifact. No peephole or default compare policy changes are included.
