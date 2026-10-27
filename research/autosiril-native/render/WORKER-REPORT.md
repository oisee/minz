# Native renderer worker checkpoint — 2026-10-27

## Result

Completed the first meaningful native-renderer checkpoint: the original Autosiril
three-channel row renderer executes as Nanz/HIR/MIR2 target code and is compared
against the existing actual-Ruby planning corpus on raw MIR2, final MIR2, and an
assembled Z80 image. Both ASCII rows and a complete post-row row-state packet
must match for every engine.

Exact proof command:

```sh
python3 research/autosiril-native/render/prove_kernel.py \
  --planning /tmp/minz-render-planning-v4 \
  --autosiril /Users/alice/dev/autosiril \
  --out /tmp/minz-z-workers/render-results/kernel-20261027-clean
```

Output is preserved in `/tmp/minz-z-workers/render-results/kernel-20261027-clean`.
The clean log is adjacent as `kernel-20261027-clean.log`. The exact measured
receipt is also copied to `evidence/native-kernel.json`.

Commit status: a local commit was requested, but the managed workspace could not
write the worktree index:
`fatal: Unable to create '/Users/alice/dev/minz/.git/worktrees/minz-autosiril-render-z/index.lock': Operation not permitted`.
No SHA is claimed. The intended commit consists solely of the five new owned
render files shown by `git status`; the checkout is otherwise clean.

## Coverage

- 203/203 cases and 10,320/10,320 rows on each engine.
- 609 case/engine results: 203 raw MIR2, 203 final MIR2, 203 assembled Z80.
- 515,988 Z80 text bytes and 794,640 Z80 post-row state bytes matched Ruby.
- Observed line widths are exactly 46, 47, 48, and 49.
- Witnessed transitions: envelope demotion, octave raise, pforms-before-mutation,
  and empty-envelope reset.
- Z80 execution checks passed for return contract, PC trap/image bounds, SP floor,
  IRQ state, memory writes only to globals/backend spills/stack, no executed
  declared data, no unexpected port I/O, unchanged config, and canaries.
- Image: 7,839 bytes total; 790 global bytes, 88 spill bytes, 6,961 code bytes;
  peak observed stack usage 34 bytes.
- Source/MIR/ASM/binary SHA-256:
  - Nanz: `686eb8e722ec5b53c8719047b4e1bde5be1a921d8c42ed69d5b3cbba829a0594`
  - raw MIR2: `01cc0c3ab26f8c1276567ad6cb8a5cd53c4268486deebc047125149aa889316c`
  - final MIR2: `ebdc3efbadb8f0bd9456f630a45a26e77c5a30e6b1bd3cc5a803c1cdaf90ccef`
  - assembly: `e42e240c58e9183ce512ad8fb14d70604de30122cda71726fdbdbc6cc3a128df`
  - COM/binary: `e8158b59563f3357878eb4b13fbe268c4890eaa390f9a5d7e64528d8857c72a0`

## Scope and limitations

- This is a row-kernel proof only. Source records and sink packets use explicit
  fixture traps (`RST 8` and `RST 0x10`); it is not the later independently
  linked CP/M/TRD file program or six-COM chain.
- The host proof generator serializes already-generated actual-Ruby fixtures and
  compares collected target bytes. During target execution it performs no musical
  decision, envelope selection, formatting, row scheduling, setter recalculation,
  or result lookup.
- Target storage is bounded to three 16-byte channel heads, one current row of
  three mutable notes, two retained/sentinel note slots, previous forms/penote,
  one 52-byte line packet, and one 82-byte state packet. Whole-song arrays are
  not loaded into Z80 RAM. The kernel admits 9,216 source cells total and rejects
  unsorted or non-increasing lane rows.
- `render_rows` is a target-side `u32`, including zero and tails beyond the
  render interval; this corpus's maximum is 1,352 rows. Source rows are admitted
  through 65,535, matching the fifth-stage fixture profile.
- Malformed/failure-injection, capacity overflow, cleanup, CP/M, TRD, >64K,
  segmentation, bank-restore, and six-COM gates remain future milestone work.
  No eventual-milestone completion is claimed.

No wakeup daemon was created, legacy STOP markers were not modified, and files
under the future patterns/VT2 worker's ownership were not touched.
