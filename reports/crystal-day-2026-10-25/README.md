# Crystal на Z80: память, meta и генераторы

Итоговая HTML-статья за 25 октября 2026: Crystal frontend, macro/meta, arena,
общие borrowed views/buffers/ring, iteration/generators и исправления общего
HIR/MIR2/Z80 compiler. Она отделяет новые результаты от ранее существовавших
Nanz/HIR возможностей и перечисляет границы проверенного профиля.

`python3 reports/crystal-day-2026-10-25/build.py` пересобирает standalone
`index.html` и manifest из сохранённых snapshots. Это сборка статьи, не запуск
runtime tests. Команды проверок находятся в `research/crystal-hir/ITERATION12.md`.
Все snippets извлекаются из артефактов; receipts и их hashes встроены в HTML.

QA: desktop 1440, mobile 390, narrow 320 и dark 1440; без горизонтального
page overflow и page errors, раскрывающиеся evidence blocks работают.
Desktop/mobile/dark и frame section визуально просмотрены. Нет CDN или
обязательного JavaScript. Standalone копия доступна как
`/Users/alice/dev/minz/reports/crystal-day-2026-10-25.html`.
