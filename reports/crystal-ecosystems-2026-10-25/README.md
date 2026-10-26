# Crystal / HIR ecosystem report

`index.html` is a self-contained Russian HTML report with inline CSS/SVG, no
external assets or JavaScript. It covers verified Crystal experiments and
separately identifies proposed ideas from JVM and BEAM ecosystems. Official
sources are linked next to the relevant claims.

Rebuild from the checked-in experiment receipts:

```sh
python3 reports/crystal-ecosystems-2026-10-25/build.py
```

Visual QA: Chromium, desktop 1440 px and mobile 390 px screenshots inspected;
layout widths also checked at 320 and 768 px. No document-level horizontal
overflow, no JavaScript errors; details/summary interactions work without JS.
Light and dark rendering checked. Core arena results come from the recorded
2372-case/four-engine run, not from report rendering.
