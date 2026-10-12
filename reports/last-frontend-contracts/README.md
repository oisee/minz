# Last frontend contracts, 12 October 2026

The two previously pending corpus paths now have independent native evidence: Lizp cond_test uses the documented last-form return contract; PL/M Hello uses explicit NUL and the CP/M console runtime. See ../lizp-last-form and ../plm-hello-runtime for actual source, MIR, ASM/BIN and execution evidence.

Catalog regeneration PASS: 194 exact paths, zero pending-independent-oracle. Categories remain explicit: 82 bounded-independent-vm-native, 70 executed-independent-vm-z80, 2 executed-independent-bounded, 21 native ASM, 11 CP/M runtime, 5 Agon MOS, 2 SMC, 1 host-only. These are separate proof scopes, not universal compiler or hardware correctness.

The fresh focused native gate and retained historical adapter gate PASS. An earlier broad directed run used a pre-fix historical Hello test and failed its source freeze; the historical test now reads the retained historical source and passed again. Its earlier failure is not included as a passing gate.

The previous complete production/CLI/research gates predate these frontend changes. Fresh core and complete pipeline results must be assessed separately before compiler integration into main. Compiler changes remain on feat/grace-hir-vm; published main reports do not imply code integration.
