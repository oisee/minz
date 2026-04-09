# 2026-04-09 Wisdom: IRC, MZV, TUI, PBQP baseline

## Working baseline

Repo for the next session:
- `/home/alice/dev/minz`

Compiler/runtime baseline:
- `PBQP` is now the real default path in pipeline, not just a design goal.
- `Z3/VIR` is parked by default; interpret new behavior on this PBQP-first baseline.

Important recent progress across sessions:
- QBE `u8` fix landed.
- PBQP-PFCCO solver work landed.
- conditional `CALL` landed.
- Port I/O ops landed: `OpIn8/OpOut8/OpIn16/OpOut16`.
- `mzv`/port infrastructure exists and is good enough for real network apps.
- `loop {}` keyword exists, but still treat it as suspicious in app/runtime debugging.

## IRC status

Confirmed working:
- `examples/nanz/test_irc_minimal.nanz`
- Libera Chat plain TCP connect works and receives 001 welcome.
- DarkScience TLS connect works and receives 001 welcome.
- `PING/PONG` now works in `test_irc_minimal.nanz`.

This proves:
- network transport works,
- TLS works,
- line accumulation works,
- basic IRC protocol is alive on MIR2 VM.

So the remaining IRC issue is NOT transport.

## MZV / TUI runtime lessons

Key fixes already identified/applied in the shared `minz` tree:
- `mzv` needed a single shared stdin reader.
- `tui_read_key()` must use the shared stdin channel and be non-blocking.
- EOF on stdin must NOT kill a headless or networked TUI program.
- raw terminal cleanup should happen on explicit exit, not on any stdin read error.

Strong lesson:
- Do not mix runtime debugging with TUI architecture redesign.
- First stabilize raw `mzv` TUI behavior.
- Then build better UI layers on top.

## Full IRC client status

File:
- `examples/nanz/irc_client.nanz`

Current state:
- Client now clearly sends `NICK`/`USER` and reads server bytes under `mzv`.
- Earlier "clear screen and immediately exits" diagnosis was too coarse; on the current baseline the program is alive enough to read network.
- Remaining blocker is visible TUI behavior / meaningful screen output.

Suspicious areas still worth checking:
- `loop {}` in `main()` is still suspicious; use `while 1 == 1` if needed to de-risk.
- Helper-heavy TUI path may still hit bad call-arg/codegen shapes.
- In the mock TUI compile log, there was a concrete warning-level symptom:
  - `CALL print_from without arg setup` in `show_raw`
- This suggests simplifying critical render helpers before blaming the whole runtime.

## Mock/stub direction

Good idea from this session:
- do TUI debugging without live IRC server,
- but make the mock reusable later for the real IRC UI.

Desired mock shape:
- deterministic line source,
- same screen/update path as IRC,
- easy switch later between mock feed and real network feed.

Do not build a separate throwaway demo if it diverges too far from IRC.

## TUI architecture lessons

Important reports:
- `reports/2026-04-09-Bruto-IDE-Lessons-For-MinZ-TUI-and-Screen-RU.md`
- `reports/2026-04-09-Claude-IRC-TUI-Handoff-RU.md`
- `reports/2026-04-04-Native-Screen-DSL-Proposal-RU.md`
- `reports/2026-04-09-TUI-View-DSL-Proposal-RU.md`
- `reports/2026-04-09-Claude-TUI-View-DSL-Review.md`

Main takeaway:
- current `@screen` is good for forms,
- but live apps like IRC need a middle layer above raw `tui_*`.

Recommended widget set for future live-TUI DSL/runtime:
- `status`
- `log`
- `list`
- `input`
- plus `separator`

Important refinements from Claude review:
- named bind args
- split weights
- compile-time geometry resolution for v1
- dirty flags
- ring buffer for log
- full redraw on Z80 is too expensive

But again: do NOT implement this before raw `mzv` TUI path is stable.

## Collaboration pattern that worked

- Use Claude for bounded diagnosis/review, not open-ended redesign.
- Give exact report path and exact done criteria.
- Keep him on hold after each bounded result.
- Codex does the actual integration and chooses what is real vs overclaimed.

## Next-session guidance

Do not start by designing a new DSL.
Start with:
1. make `irc_client.nanz` render something real and stable under `mzv`
2. prefer simplifying risky helper/call shapes over inventing more abstraction
3. only after that, refine the TUI View DSL spec and maybe prototype `StatusBar` + `InputLine`
