# Табличное назначение: проверка фактически выбранного pattern

База `47127a27`. Независимая проверка complete assignments/live interference
не защищала от scratch-регистров внутри выбранного шаблона. Конкретный native
контрпример: A содержит 37 и нужен после `H → IXH` move. Pattern
`ld_ixh_hl8` выполняет `LD A,H; LD IXH,A`, не объявляя A в Clobbers. Старый
emitFromTable принимает назначение и возвращает **5 вместо 37**. После peephole
остаётся BIN `3e253e05c9`; исходный failing runtime log сохранён в before.txt.

Изменения:

- Table fast path принимает только прямой pattern, удовлетворяющий назначению.
  Не вставляет несертифицированные pre/post repair moves: они могли затирать
  живой A или второй operand. Отклонённый hit возвращается обычному solver.
- Для каждого выбранного pattern проверяются writes расширенных инструкций и
  declared Clobbers против независимо вычисленных live-through values.
  Физические overlap/половины учитываются через descriptor и canonical z80spec.
- Неизвестные instruction effects отвергают fast path. Проверка консервативна:
  временный write с последующим restore тоже может отклоняться; восстановление
  через PUSH/POP пока не является сертификатом этого пути.
- Итоговые инструкции проходят shared MZA validator; one-block Ops должны точно
  совпадать с allOps. Multi-block table emission отказывается вместо потери edges.

Проверки:

- unsafe hidden-A fixture отклоняется до возврата ASM;
- implicit accumulator repair отклоняется;
- live flags pattern clobber отклоняется;
- допустимый scratch fixture на собранном Z80 сохраняет A=37, пишет memory9000=5,
  возвращает SP=f000: BIN `06253e05dd6732009078c9`, 74 total fixture T-states;
- прежние direct table native cases проходят, binary/enriched admission тоже.

Go 1.26.8, GOCACHE=/tmp/minz-grace-gocache. Focused suite PASS, after.txt.
Full pkg/vir FAIL только на прежнем TestLocSets8IXDefinitions (ожидает 11,
фактически 9). Тот же failure подтверждён с неизменённым baseline pipeline.go,
логи full-vir.txt и baseline-failure.txt. GPU executable отсутствует, его test
explicit SKIP; это не подтверждение исполнения GPU solver.

Это исправление подтверждённого miscompile в табличном emitter. Versioned
constraint/cost keys, поиск альтернативного безопасного pattern, корректность
всех прочих peepholes и полный useful table coverage остаются отдельными gates.
