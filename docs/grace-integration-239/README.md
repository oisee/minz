# Новый core после условного возврата и Pascal CP/M

Полный HIR/MIR2/Nanz/VIR начат на `76945771`, `go test ./pkg/hir ./pkg/mir2 ./pkg/nanz ./pkg/vir -count=1 -timeout=30m`; session31401, журнал `core-76945771-full.txt`. Включает TermCondRet, exact Pascal output adapters, public .pas CLI validation и обновлённый каталог. Пока результата нет, PASS не заявлен.

Отдельный неизменный retained pipeline `29df08bb` остаётся live process2392127, примерно 70 минут. Он имеет исходный 90-минутный timeout, не перезапускался из-за продолжительности; scope предшествует новым исправлениям.

Self-parser classifiers — другой live executable: `self-parser-classifiers-running.test`, SHA256 `6dc1363719c5cde1c862ebd247100a7c8ab2dd597ca457eb5605135686b954bf`, исходная база31161121/test c382bf49. Ordinary и Grace Speed дали положительные per-mode результаты, общий terminal ещё неизвестен. Полный original module сохранён; файловые/печатающие externs не исполняются.

Точные source delta b711c8cf→76945771 и self-parser executable сохранены с SHA проверкой на `/mnt/safe/minz/grace/2026-10-12/`. Это запись конкретных проверок, а не общий статус завершения цели.
