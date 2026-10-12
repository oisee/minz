# Current transferred-engine gate and alias expectation

Fresh transferred rewrite/label/canonical-spec packages exposed an outdated test expectation: VIR pairAliases now includes IX and IY, while TestVIRPairAliases still required their omission. It now requires no missing canonical pair aliases; existing checks independently validate every declared pair against z80spec and reject duplicates. No production alias behavior changed; alias implementation/native proofs were committed earlier. Final transferred-engine packages PASS.

Read-only tracked-source inventory finds no live minzc/pkg/lir files and no Go import of that package. Production CLI dependency list retained: VIR remains reachable through checked table utilities, so absence of LIR does not imply absence of VIR. Historical archived source/evidence is retained. No universal native semantics or complete objective claim.

Fresh seven-package core at a7ffb886 remains a separate live run, as do full CLI/production/research gates.
