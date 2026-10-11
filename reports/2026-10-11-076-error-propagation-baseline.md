# Original error propagation: frozen negative evidence

Исходный `examples/nanz/14_error_propagation.nanz` сохранён целиком. Workload заморожен коммитом 00553e9d до запусков. Oracle: Nanz Language Book v7, Appendix K (3752–3774): ошибка возвращает CY=1 и код в A; @propagate/@check прекращают продолжение. Успех возвращает CY=0. Старые главы книги называют возможность будущей; проверяем явный текущий контракт Appendix K и комментарии исходного примера.

На base 2b8910d4, default, GraceSize и GraceSpeed одинаково неверны:

| Вызов | Ожидание A/CY | Факт A/CY |
|---|---|---|
| safe_div(10,0) | 1/1 | 10/1 |
| safe_add(200,200) | 2/1 | 200/1 |
| safe_sub(0,1) | 3/1 | 0/1 |
| compute(10,0) | 1/1 | 10/1 |
| chain(255,1) | 2/1 | 255/1 |

У main начальный CY=0 даёт stdout `5250`, CY=1 даёт пустой вывод: оптимизация заменила успешные вызовы константами, а RET C остался читать входной carry. print binding — существующий порт 0x23, измеряется его точный байтовый вывод; это не CP/M OS. Raw/final VM сообщает unknown function @error или @propagate, а не результат языка. Никакие noop host hooks не устанавливались.

Все 8 диагностических вызовов на режим сохраняют SP и проверенный RAM/code, main отдельно проверяет возврат/вывод/SP без полного RAM-оракула. Это отрицательный пакет, не полная проверка замороженной матрицы. Нативные вычисления compute(255,1) и array_get(4) также сохранены; не утверждаем, что вся success ABI неверна по этим пробам.

Причины: @error кодогенератор делает SCF/RET без загрузки аргумента в A; intrinsics — обычные MIR Call без управляющей семантики; raw RET обходит cleanup локального frame; ConstantCallElim моделирует значение, но не CY.

Повтор: скопировать baseline-probe.go.txt в minzc/pkg/pipeline/carry_error_baseline_test.go, выполнить go test ./pkg/pipeline -run '^TestOriginalCarryErrorBaseline$' -v. Ожидается FAIL на сохранённом compiler context. Исходный probe вынесен из production tests, чтобы исторический отрицательный эксперимент не ломал suite после исправления. BIN содержит исходный модуль плюс отдельно маркированные CALL/RET invocation drivers, ORG 8000.

Следующий шаг: TermErrorRet(Code), функция с явной fallible CY ABI, SSA call-status/code, VM language outcome отдельно от Go traps, native capture до caller restores и общий frame cleanup. До status-aware реализации запрещается сворачивать/inline fallible calls только в значение. Покрытие исходного примера пока не green.
