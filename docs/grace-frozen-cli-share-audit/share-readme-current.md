# Grace: сайт, отчёты и доказательства

Рабочая ветка `feat/grace-hir-vm`, checkpoint `6ec387d6`.

- [Последняя сводка компилятора](compiler-docs/grace-current-state.md)
- [Полный опубликованный сайт офлайн](site-and-artifacts/site/index.html)
- [Все отчёты репозитория](compiler-reports/)
- [Документация и пакеты доказательств](compiler-docs/)
- [Зафиксированные исполняемые файлы проверок](compiler-checkpoints/)
- [План](compiler-reports/2026-10-11-017-unified-backend-roadmap.md)

Sites 61 опубликован: свежая сводка и девять архивов доказательств.
Полная копия обновления и deployment archive сохранены и проверены SHA256.
Предыдущие Sites 60 артефакты сохраняются; их полный исторический манифест
охватывал 24 755 файлов, 9 224 782 963 байта. Новые repo отчёты доступны отдельно.

Последняя полная сверка SHA256 документации и отчётов выполнена на checkpoint
11760b2b: 14 167 файлов документации и 427 отчётов. Последующие пакеты проверены
отдельными integration-037…042 манифестами; они не объявляются частью старого
полного манифеста. verification.json сохраняет точные границы проверок.

Приватная публикация:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/current-state.html

Полная текущая сверка docs/reports: 6ec387d6 (14563 файлов документации, 437 отчётов). Исторические файлы сохраняются; это проверка копии, не PASS всего компилятора.

## Frozen CLI и последняя отдельная проверка

[Точные executable, контексты и команда replay](FROZEN-CLI.md). Checkpoint-манифест
теперь включает все 11 executable, в том числе оба replay-компилятора. Его прежняя
версия сохранена в `checkpoint-manifests/`. Это отдельная текущая проверка, а не
обновление исторических полных docs/reports манифестов.

Канонические пакеты CLI: [replay с training-проверкой](compiler-docs/grace-compiler-mode-replay-trust/REPORT.md),
[обычный replay](compiler-docs/grace-compiler-mode-replay/REPORT.md),
[memory-host BIN/ASM](compiler-docs/grace-memory-host-selected-output/REPORT.md).
Одноимённые дополнительные каталоги в `compiler-reports/` сохранены исторически;
для этих пакетов используйте ссылки в `compiler-docs/`.
