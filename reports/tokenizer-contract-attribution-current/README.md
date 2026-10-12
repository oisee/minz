# Self-tokenizer: где PBQP Size проигрывает greedy

Оба варианта собираются из точного `examples/nanz/self_tokenizer.nanz`: ContractOpt=true, OptSize=true, AssertMode=none; отличается только greedy/PBQP contract strategy. Тестовый экспорт расширен: теперь сохраняются источник, final MIR, точный ASM и собранный BIN, а не только таблица функций. Production код не изменён.

Полный BIN greedy **5465 B**, PBQP **5533 B**: +68 B. Разница по неперекрывающимся изменённым участкам: `init_keywords` 370 → 430 B (+60), `str_alloc_3..6` по +2 B (+8 суммарно). `str_alloc_2` меняет входные homes A/C на C/A без изменения собственных 70 B; `region_eq` меняет третий home B на C при тех же 131 B. В `init_keywords` видны повторные загрузки аргументов перед CALL под изменённый ABI. Это конкретный признак стоимости адаптации callers, не доказательство глобальной причины всех решений PBQP.

Сумма всех function spans не равна размеру модуля: исходный diagnostic определяет границы по символам, а aliases/extern и хвост данных могут влиять на сумму. Для цели Size используется фактическая длина BIN, а не сумма этих строк.

Проверка завершилась PASS 1,084 s: `MINZ_TOKENIZER_OBJECTIVE_EXPORT=<output> go test ./pkg/pipeline -run '^TestTokenizerContractObjectiveAttribution$' -count=1 -timeout=10m -v`. Она проверяет compile/assemble и сохраняет attribution; не запускает VM/native workload, не даёт T-states и не подтверждает корректность новой ABI этим результатом. Более широкое исполнение проверяется отдельно текущим полным pipeline. Следующее улучшение должно учитывать фактические caller moves и повторное использование уже выбранных homes, сохраняя независимые оракулы.
