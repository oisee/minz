# Повтор рекомбинации и переноса исторических назначений

Четыре opt-in проверки полного gate повторены. Composition hash3 использует сохранённую proposal-таблицу, текущий exact binding и 65536 независимых tuples на каждый путь: равный результат 6 B / 30 T, модуль 45 B. Это ограниченный домен, не все 2^24 входов.

Source clone reuse: rename, constants 3/17/255 и новые vreg IDs сохраняют historical shape, но получают разные checked keys и повторный допуск. PASS.

Quotient weighted: 13 текущих vregs свёрнуты в 2 исторические переменные для 10x, 12 в 2 для 9x. Все256 входов helper и реального caller: 10x модуль44→20B caller238→142T; 9x модуль40→19B caller222→138T. PASS.

Старый TestIndexedHistoricalTripleNative требовал table gain для canonical indirect entry. Текущий bridge явно отказывает таким функциям, и уже существующий practical test проверяет этот контракт. Старый тест обновлён: требует точный отказ, пустую таблицу, неизменный ASM; затем выполняет весь прежний VM/direct/indirect домен. Triple66→66T; apply123→123T;34B. Никакого table gain или поддержки indirect bridge здесь нет. Производственный отказ не ослаблен; расширение bridge остаётся отдельной задачей.

Composition PASS0.176s; остальные три PASS3.450s. Исходный диагностический FAIL сохранён в before.txt.
