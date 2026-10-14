# Embedded release candidates

These are native Spectrum snapshots. The Tetris candidate was refreshed on
14 October 2026; the other snapshots retain their archived 13 October builds.
Exact sources, binaries and SHA256 receipts are preserved under
`/mnt/safe/minz/grace/`.

* tetris.sna: examples/zx/tetris_playable.nanz, scripts/build_zx_tetris_demo.sh.
  Pixel-step falling actor uses baked Nanz sprite instances and shadow dirty
  bands. Native main is checked against independent physics and fetched ULA
  frames; see reports/advanced-demos/tetris/smooth-gameplay.md. Code starts at
  6000, shadow bitmap occupies E000..F7FF, ordinary launcher stack is FCE0.
  Preparation uses typed Nanz row walking and a baked eight-store fill kernel.
  Full-field restoration is 54.6% faster than the first pixel candidate; see
  reports/advanced-demos/tetris/native-preparation.md. Preparation remains slower
  than one frame; this is not a 50-FPS claim.
* zork1.sna: zork1-current-128/zork-spectrum128.sna, builder a4b9ac24.
* zork2.sna and zork2-bank10.bin: zork2-late-save/play; Scorpion memory,
  persistent disk SAVE, verified full game and fresh-machine RESTORE.
* rotozoomer.sna: reports/zx-rotozoomer-library/native/ordinary/running.sna.
* sprites.sna and sprites-push.sna: reports/hardware-sprite-chain/demo.

Zork stories are compiled from the pinned licensed source identified in
reports/zmachine-spectrum128/release/plan.md. MIT notices are included here.

Release work is in progress: Linux graphical menu and Windows Tetris startup
have been exercised. Final packaged-binary verification, main merge,
publication and the release audit are still required.
