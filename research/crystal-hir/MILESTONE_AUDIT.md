# Проверка завершённой вехи 25.10.2026

Последнее уточнение пользователя: «добегаем до ближайшей вехи и пишем статью»,
«и в кристале и вообще». Ближайшая активная веха — generator frame liveness;
итоговый материал охватывает Crystal и общую compiler ecosystem. Дальнейшие
расширения из roadmap не считаются уже реализованными.

| Требование | Прямое доказательство |
|---|---|
| Crystal iteration подключена к общему HIR | record_export.cr + cmd/crystal-record-hir: Slice→ForEach, yield→GenFor/Yield; native generator corpus 2048 и ring corpus 525 |
| Общие buffer/slice API для Crystal и Nanz | stdlib/mem/bytes.nanz, bytes.cr, explicit library bindings; один SHA реально прочитанной библиотеки в receipts |
| Semantics/effects/early stop | iteration12-generators-results: четыре engine, 5 отказов; iteration12-rings-results: четыре engine, 6 отказов, 7 legacy cases, counter при stop до второго сегмента |
| Memory/lifetime contracts | Caller-owned storage, snapshots sharing bytes, clear/rebind/alias controls, FIFO backing/guards; borrowed lifetime не продлевается, borrow checker не заявлен |
| Ближайшая frame-веха | Core commit 09ba28f96, iteration12-results: Ring 53→13, counter 6→3; Nanz ANF 8→3; before regression сохранён |
| Сохранение Nanz semantics | iteration12-regression.txt: generator completion/derived frame/definition liveness и G1 negative/runtime gates, buffer/ring tests |
| Native Crystal / raw MIR2 / optimized MIR2 / Z80 | Свежие iteration12-generators-results.json и iteration12-rings-results.json; layout receipts содержат фактические поля/размеры |
| Итоговая HTML-статья по Crystal и общему compiler | reports/crystal-day-2026-10-25/index.html, template, build.py, manifest, artifact snapshots; standalone копия в основном reports |
| Красивые проверенные материалы | Infographics memory/FIFO/frame; verbatim ASM, corpus table, explicit boundaries; desktop/mobile/320/dark QA и visual inspection |
| Отдельная ветка и публикация | spike/crystal-hir; code commit 09ba28f96 уже опубликован; финальный docs push проверяется HEAD == origin/spike/crystal-hir |
| i7 только читать | В заключительных этапах все compile/test/edits выполнены локально в /tmp/minz-crystal-hir; SSH/i7 вызовов не было |

Числа в статье относятся к отдельным корпусам, а не к проценту поддержки языка.
Исторические корпуса помечены как historical receipts; после нового liveness
заново выполнены именно generator/ring и указанные общие gates. Byte-stream
33 B и RLE 47 B остаются conservative fallback; их уменьшение не заявляется.
Frame size не является полной stack/RAM или speed метрикой.

Остаются явно документированными границы: full Crystal runtime/stdlib, generic
owning containers, автоматическая lifetime проверка, full exception unwinding,
channels/scheduler, произвольные stdlib Iterator objects, universal semantic
pointee linker и идеальность ASM. Это roadmap, а не скрытые completed features.
