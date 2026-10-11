# MinZ / Grace: сохранённые исходники, сайт и доказательства

Актуальная рабочая ветка: `feat/grace-hir-vm`. Сводка и пакеты на этой шаре относятся к своим указанным checkpoints; общий полный PASS из наличия файлов не следует.

- [Актуальная сводка](compiler-docs/grace-current-state.md)
- [Полный обзор языков и компилятора](compiler-reports/2026-10-11-092-grace-vm-z80-status.md)
- [Каталог 194 точных исходных путей](compiler-docs/grace-integration-204/catalog.json)
- [Последний heldout и allocation replay](compiler-docs/grace-integration-205/README.md)
- [Опубликованный приватный сайт v68 целиком офлайн](site-and-artifacts/site/index.html)
- [Новые отчёты и доказательства](compiler-reports/)
- [Документация и integration packets](compiler-docs/)
- [Зафиксированные исполняемые файлы](compiler-checkpoints/)
- [Копии исходников, тестов и библиотек по checkpoint](compiler-source-evidence/)

Каталог204: 133 independent/bounded VM/native paths, два отдельно native SMC, два pending языка; остальные сохраняют ASM/runtime/Agon/CPM/host требования. Это пути, включая архивные копии, не количество независимых приложений.

Полный pipeline29df08bb выполняется; binary SHA и команда в compiler-docs/grace-integration-202. Terminal PASS пока не заявлен. Сайт v68 опубликован на более раннем snapshot047f0681: 9890 файлов/253494209 bytes проверены по SHA256; поздние compiler packets сохраняются отдельно и не объявляются частью старой публикации.

Общий отчёт и README feature отправлены в main через PR74: https://github.com/oisee/minz/pull/74 . Прямой push защищён; auto-merge ждёт обязательную assert matrix. Это docs-only change, не перенос всей исследовательской ветки.

Исторические manifests и verification.json сохраняют свои прежние границы. Поздние пакеты проверяются собственными manifests. Предыдущая главная страница сохранена как README-history-through-6ec387d6.md.
