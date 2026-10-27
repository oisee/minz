# Frozen fifth-stage fixtures

`replay.json.gz` contains portable actual-fresh matrices, original Ruby reference
results, source hashes and expected target output/IO/image signatures. It is
compressed to avoid repeating large native song byte strings in the repository.
The ordinary text source and generation/verification scripts remain reviewable.

`freeze.py` creates it only after the complete fresh fifth-stage gate has passed.
`replay.py` checks the producer source hashes, resolves `@echo/` paths against the
actual preceding CI artifact, reconstructs deliberate catalog mutations, rebuilds
all target programs and compares their results. Ruby/Crystal are not executed by
frozen replay; reuse of their actual-fresh references is recorded explicitly.

Raw source/native/MIR/assembly/disks/files/operation traces and mutant rejection
logs are retained in each run's output tree and uploaded by CI. Signatures in
this archive do not replace those inspectable artifacts or independent audits.
