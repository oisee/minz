# Main-based compiler integration checkpoint

This branch starts from main 56b8aa09 and transfers the research source delta from fe8ea7ff: HIR/MIR2 frontend/backend fixes, checked tables and contracts, Grace/profile tooling, CP/M runtime and opt-in MZE game clock. LIR retirement is explicit. Existing main documentation remains intact.

Builds of minzc, mza, mze, mzv and import-enrt PASS. Compile-only HIR/MIR2/VIR/MZE PASS; fresh short HIR29.118s/MIR2 71.640s/VIR4.057s/MZE0.496s PASS. Regenerated194-path catalog PASS2.528s. Focused native Tetris, Lizp and PLM showcase gate PASS4.379s. These logs belong to this integration tree; prior research gates remain separate.

The complete short pipeline is running; its result is pending. Fixtures preserve exact frozen sources and inputs at existing paths, plus evidence files required by the reviewed registry. Bulk research exports remain in the research branch and share. A later audit will check every manifest dependency and prune accidental output candidates. This is an integration checkpoint, not a declaration of complete validated merge readiness.
