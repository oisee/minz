# Удаление перезаписанных byte literals

Z80 post-emit pass удаляет ранний LD r,n, если в непрерывной последовательности непосредственных byte-register loads тот же регистр перезаписывается до использования. Допущены A/B/C/D/E/H/L и числовые literals 0..255; comments/blank lines прозрачны, labels и любые другие инструкции являются barriers. Pair/half aliases, memory, CALL, control flow и symbolic immediates не пересекаются. ABI и MIR не меняются. LD r,n не читает регистры и сохраняет flags.

Native сравнение до/после проверило 768 сочетаний входного состояния и flags: все основные registers, IX/IY, flags, исходный PC возврата и SP совпадают. Отдельно проверены 11 видов barriers. Существующие literal-cycle и peephole тесты прошли. Полный MIR2 PASS; точное время в сохранённом логе. Арифметическая/PFCCO prospective heldout проверки PASS; PFCCO heldout 6252 → 5568 T / 52 B сохраняется, Size выбирает baseline при равенстве. Исходный tokenizer main прошёл независимый full-state oracle через port23 и CP/M BDOS2.

На tokenizer ContractOpt/PBQP Size полный BIN уменьшился **5533 → 5513 B**: init_keywords 430 → 410 B. Greedy остаётся 5465 B. Это уменьшение 20 байт; runtime T-states данной конфигурации не измерены. Данные до изменения находятся в `tokenizer-contract-attribution-current`, новые ASM/BIN/MIR и источник рядом. Правило не устраняет весь PBQP проигрыш; остаётся 48 байт против greedy.

Основной полный pipeline 5d90b7aa и CLI 85fd7531 собраны до этой production правки. CLI завершился PASS в отдельном пакете, pipeline ещё работает; их результаты не переносятся на новый backend автоматически.
