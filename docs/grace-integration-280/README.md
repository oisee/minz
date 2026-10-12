# Literal-call: фактический root training/heldout с экспортом

С required MINZ_CALL_LITERAL_TOKENIZER_EXPORT выполнен TestCallLiteralTokenizerProspective, без SKIP: PASS2,221с. На двух прежних prospectively frozen training входах greedy411820T/5465B и PBQP412303T/5533B. Оба goals выбираютgreedy; тот же trainingASM/BIN исполняется на четырёх отложенных входах:885117T/5465B, fullstate/stdout/RAM/PCSP checks.

Это текущий exported root повтор donor frozen эксперимента, не новые training/heldout данные. Дополнительный выигрыш PBQP не сделал его лучшеgreedy. Полный export1,8MB сохранён на шаре отдельно; immutable donor before/after evidence не заменяется. Ранее jointgate276 пропускал этот тест безenv; его документация исправлена.
