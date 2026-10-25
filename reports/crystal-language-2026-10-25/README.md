# Ruby остаётся прекрасен. А что получилось на Z80?

Standalone Russian article covering the Crystal subset, compile-time meta,
HIR/MIR2, actual Z80 output, host integration, ABAPiti/parser options and the
applicability to Autosiril. Includes the additional scalar Crystal→Go probe.

```sh
python3 reports/crystal-language-2026-10-25/build.py
python3 research/crystal-hir/verify_go.py
```

The build extracts assembly/HIR/MIR2 and Go snippets verbatim from checked-in
artifact snapshots. `manifest.json` records hashes and static listing instruction
counts; these counts are not machine code sizes or timings. The shared stylesheet
comes from the preceding ecosystem report, and is embedded in the final HTML.

Visual QA: Chromium screenshots inspected at desktop and mobile widths; no
horizontal page overflow at 1440, 390 or 320 px. Light/dark modes and native
details/summary interactions checked. No JavaScript or external assets required.

The generated Go listing and receipt are snapshots of one observed build;
register-declaration ordering in the existing emitter may differ on regeneration.
The core compiler was not changed for this article or Go probe.
