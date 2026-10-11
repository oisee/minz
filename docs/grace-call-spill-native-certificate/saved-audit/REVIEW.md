# Saved call-spill certificate audit

PASS: policy collect/replay eight images and search collect/paired replay sixteen
images. No CPU replay was performed. Exact preserved test executable SHA is
`4d818aab4c5bbae1e5e794a7d6c1f9c8d22e1421f651c174fdc908a3f1b3d653`;
every target context binds that executable. Each run contains two real workload
calls, and raw/native results, globals, regions and ordered resource events
agree after removing only diagnostic instruction-step counts.

The typed Go audit verifies sidecar, call-frame, call-spill, allocation, MIR and
native binary hashes against the saved files. Every image contains ten finite
call-spill stages paired by callsite, home register, symbol and width. Their
exact bytes reproduce the save/restore grammar; the marked direct CALL, stage
PC ordering, named spill access sidecar, and embedded binary extents agree.
All precompile proof choices, source sites, source SHA, original/actual
compatibility and function maps bind the respective association and candidate.

Policy collect/replay selects the same speed image and stores identical
selection; its external assembly equals the selected image. Search collection
uses actual Speed and selects policy1/mask1; paired replay uses actual Size and
selects policy0/mask2 (796T/2224B). The sixteen candidate envelopes are identical
across these goal changes; only selection options/winners differ. Each decision
hashes the exact saved search/association bytes and selects the matching saved
ASM/BIN. External `-o` assembly equals the actual Size-selected image.

Reproducers and terminal logs are adjacent. The typed verifier was built from
the owner’s current pipeline types; it only decodes saved evidence. These are
bounded collect/replay regression fixtures, not the full thirty-call benchmark
or physical CP/M execution. Runtime provenance admission was separately tested
by the owner; this audit confirms its saved evidence and byte bindings.
