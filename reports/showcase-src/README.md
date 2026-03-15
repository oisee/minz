# Showcase source snapshots

Each dated folder contains the `.nanz` source files and corresponding `.a80`
assembly outputs for the examples in the Nanz Real ASM Showcase report (#051).

## Purpose

Track compiler output evolution over time.  When the compiler improves (or
regresses), re-run:

```bash
cd /Users/alice/dev/minz-ts/minzc
go build -o mz ./cmd/minzc/
mkdir -p ../reports/showcase-src/YYYY-MM-DD
for f in ../reports/showcase-src/2026-03-10/*.nanz; do
    name=$(basename $f .nanz)
    cp $f ../reports/showcase-src/YYYY-MM-DD/
    ./mz $f
    cp /tmp/nanz_showcase/$name.a80 ../reports/showcase-src/YYYY-MM-DD/ 2>/dev/null || true
    ./mz ../reports/showcase-src/YYYY-MM-DD/$name.nanz
    cp ../reports/showcase-src/YYYY-MM-DD/$name.a80 . 2>/dev/null || true
done
```

Or simply re-compile the nanz files from the dated folder and diff the .a80
outputs against the previous snapshot.

## Folders

| Folder | Compiler state | Report |
|--------|---------------|--------|
| `2026-03-10/` | post-sprint (29fabaa..811472a): interface params, HL-chain, lowerExprForRet, use-before-init, LSP | #051 rev 2 |
| `2026-03-15/` | Lizp frontend, tetris v2, lanz showcase, cross-lang import | #074–#078 |
