# Corrected legacy fifteen-case saved audit

All fifteen cases pass the independent expected-source, output/pool-byte and saved state oracle. The retained executable SHA is e041856bb88371a25adf6c957785b7d9731599eb09318c81cc024ec4fa7ec128. All twelve print cases preserve the generated-name collision globals (123 and 456), produce answer 4661 and exact expected byte output, including embedded NUL and UTF-8. S255 and L256 are admitted byte-count boundaries. The three dereference cases read first bytes S=2, C=65 and L=2 and have no stdout. Every case has empty error, positive actual cycles, exact terminal HALT PC and SP=0xF000.

All fifteen assembled.a80 files were independently reassembled with the actual assembler and matched the saved BIN byte-for-byte. No CPU was executed. Exact per-case source, binary and assembly SHA and costs are in legacy-corrected-saved-audit.json.

This export explicitly identifies compiler_route=legacy. Allocation diagnostics are JSON null and final.mir2 is empty in all fifteen cases. They are not fabricated Steps/PBQP dumps or a post-MIR proof. The assembled/native byte binding is checked independently from these unavailable diagnostics. Initial fifteen-case legacy failure history remains untouched, with four first-byte output failures retained separately; the corrected results do not relabel that history.
