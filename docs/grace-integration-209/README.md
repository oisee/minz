# PL/M module entry: root интеграция и границы

Интегрированы prospective freeze81b21303 и исправление5726a168 как d32d20d4. Root focused PL/M/pipeline PASS0,011/0,599 с; полный PL/M PASS0,025 с. Новые executable module statements, case-preserving lexer/LITERALLY и exact-byte addressed literals проверены на исходном Hello и отдельном length-bounded process fixture. Все50 manifest payload tracked/local/share SHA совпали.

Hello сохраняет пустую исходную PRINTCHAR, поэтому не обещает вывод. PRINTMSG читает до нуля; addressed literal не получает выдуманный implicitNUL. Тест явно устанавливает trailingzero в безопасном месте и проверяет полное состояние. Это bounded memory convention, не portable termination или CP/M runtime. Поэтому pending/runtime граница каталога остаётся; ошибка отсутствующего main закрыта, но не вся строка поведения.

Полный pipeline29df08bb предшествует этому frontend исправлению и ещё выполняется. Новый полный CLI/compileassemble gate требуется после завершения current production fixes. Предыдущий terminalCLI11105fb3 не подменяется focusedPASS.
