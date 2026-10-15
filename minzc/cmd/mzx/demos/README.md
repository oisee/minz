# Embedded release candidates

These are native Spectrum snapshots. The Tetris candidate was refreshed on
15 October 2026; the other snapshots retain their archived 13 October builds.
Exact sources, binaries and SHA256 receipts are preserved under
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-14/tetris-direct/`
for the current Tetris; other archived receipts retain their prior locations.

* tetris.sna: examples/zx/tetris_playable.nanz, scripts/build_zx_tetris_demo.sh.
  Refreshed 15 October: direct paired baked sprites, no bitmap shadow.
  Select 1 (one pixel per two frames) or 2 (one pixel per frame).
  Geometry, address patching and attributes are prepared before HALT.
  Native main, independent physics and fetched ULA frames are checked;
  see reports/advanced-demos/tetris/direct-renderer.md. Code starts at
  8000; the launcher reserves FC60..FCE0 for stack. Input, hold and lock
  can still take multiple frames. This is a playable release candidate,
  not a constant-50-FPS claim for every action. Source and image hashes
  are in tetris.provenance.json; new detailed receipts stay local.
* zork1.sna: zork1-current-128/zork-spectrum128.sna, builder a4b9ac24.
* zork2.sna and zork2-bank10.bin: zork2-late-save/play; Scorpion memory,
  persistent disk SAVE, verified full game and fresh-machine RESTORE.
* rotozoomer.sna: reports/zx-rotozoomer-library/native/ordinary/running.sna.
* sprites.sna: reports/advanced-demos/typed-sprite-scene/source.nanz.
  Typed whole sprite, paired erase/draw, pre-HALT placement; native main/ULA
  and successive raw/final VM calls verified. SHA256 is in sprites.provenance.json.
* sprites-push.sna: reports/hardware-sprite-chain/demo (low-level PUSH kernel).

Zork stories are compiled from the pinned licensed source identified in
reports/zmachine-spectrum128/release/plan.md. MIT notices are included here.

Release 0.25.1 was published from main a284fafc with six native packages.
The typed-sprite snapshot is a later candidate and does not replace those
published archives. See the typed-sprite report for its own proof and limits.
