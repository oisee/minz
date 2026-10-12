# Подготовка текущей таблицы для целого модуля без solver

pipeline.ReadmitCheckedLocalTables компилирует ordinary Z80 OptSize baseline, проходит MIR функции и допускает bounded пул назначений против текущих LocalTableProblem. Выход — обычная точная checked таблица и per-function notes. Внешний solver не вызывается. Неподходящие функции получают причину пропуска; ошибки режима/bound возвращают nil результат.

Source jointSource+checkedTableSource: left_sum/right_sum/caller вне leaf-допуска; add5 получает одно пригодное предложение; main не получает. После сохранения output JSON обычный pipeline выбирает smaller checked-local-table для add5; all256nativeinputs проходят. Invalid mode controls отвергают отсутствие Size, skip emission, внешний backend, input table/path pin. PASS0,630с.

Это API offline подготовки, не новый автоматический production поиск и не расширение допуска calls/CFG. Существующий checked-local-table файл остаётся exact certificate. Сохранённый full pipeline dd100284 предшествует этому API и ещё выполняется; directed test не заменяет общий gate.
