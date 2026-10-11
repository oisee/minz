# String representation repair: frozen bounded implementation

Five production paths preserve SString/LString/CString storage and dispatch direct literal printing to ordinary counted HIR loops. Arbitrary pointers retain the existing CString intrinsic. Processed UTF8 byte bounds are checked in the parser and native emitter. Empty storage has no invalid DB trailing comma. VM print.char writes one byte rather than reencoding it as a Unicode rune.

Generated bindings are reserved against all source names (including tuple and loop binders), current environment, aliases and globals. A visited pointer guard and skipped type descriptors keep recursive public HIR types from causing scanner recursion. The VM refuses string storage beyond its memory limit before mutating heap/cache; an explicitly larger VM can represent a65535-byte LString, which does not claim a fitting64K executable.

Raw and optimized VM tests verify first-byte loads, exact builtinUTF8/embeddedNUL output, live successor values and sourceglobal/local collision preservation. Storage/cache tests cover all three kinds, empty values,255S/256L and representablelength refusals. Native pipeline/PBQP fixtures are compiled but have not executed: exact storage bytes, printoutput/terminatingSPPC, neighboring CString and live global successor across ordinary counted loops are ready for review. Output equality is not claimed as an independent proof that every memory read stayed inside a literal.

No SelfLanz source, CP/M wrapper, PGO selector or production default has changed. Full native execution waits for frozen independent reviews and root release.
