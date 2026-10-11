# Повтор по полной цели: heldout, назначения и удаление LIR

На checkpoint 3c553d2b повторены четыре существующих frozen training/heldout семейства: Self-Lanz parser, HashMap, packet PFCCO и Pascal SORT. Вместе с native checked-table admission/tamper и отказом старого LIR configuration — **PASS 37,756 с**. Это replay ранее замороженных экспериментов после исправлений; нового prospective freeze или всеобщего ускорения здесь нет.

Независимые bounded exhaustive native-cost проверки PBQP, PFCCO forests и multiple calls — **PASS 1,064 с**. В измерение входят boundary moves и saves; конечные малые домены не доказывают глобальную оптимальность произвольной функции.

Текущий git tree не содержит minzc/pkg/lir. Go исходники не импортируют pkg/lir; две ссылки в комментариях shared rewrite engines описывают историческое происхождение, не dependency. Инвентарь переноса сохраняется в docs/grace-lir-removal-inventory, архив и mapping старых тестов — в docs/grace-lir-final-removal и reports/2026-10-11-lir-test-transfer-completion.md. Полноту всех pattern/physical переносов нельзя объявить только по отсутствию каталога.

Готовность общей цели всё ещё не доказана: языковые/runtime пробелы корпуса, свежий полный pipeline, сохранение и публикация поздних пакетов остаются предметом дальнейшей работы. В главной docs/grace-current-state.md добавлена актуальная сводка перед историческими checkpoints.
