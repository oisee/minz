# Saved legacy-route native fixture review

PASS for all fifteen corrected fixtures from the actual `CompileHIR` route.
This review performs no CPU replay. Independent MZA assembly reproduces every
saved full binary and symbol table. The independent byte oracle verifies C/S/L,
empty strings, embedded NUL, UTF-8 bytes, S255 and L256 output; all append `!`.
Three storage fixtures have exact C/S/L byte prefixes and typed first-byte
global values. Actual HALT PC, balanced SP F000, no execution error and expected
globals (answer 4661, collision sentinels 123/456) agree for every fixture.

The allocation/MIR metadata are empty/null for this legacy compiler route;
they are not claimed as a Steps-derived proof. Preserved executable identity is
recorded in `bytes-state.txt`. The earlier legacy S-string output failures remain
in the permanent checkpoint history. This closes these ordinary native fixture
regressions; it does not attest CP/M execution or the closed memory-host frame
decoder.

Inputs: `/home/alice/dev/minz-artifacts/grace-checkpoints/string-representation-evidence/legacy-corrected`.
Reproducers and terminal outputs are adjacent. The auditor reads saved output,
globals, registers, assembled source, symbols and binary only.
