# Lexical HIR lowering: восстановление bindings до Z80

После полного tokenizer обнаружена самостоятельная ошибка HIR → MIR: корректный вложенный Block с собственной `x` терял внешнюю `x`. Исходный probe возвращал 999 вместо 1. Это мешает доверять source/VM oracle для профильных преобразований, поэтому исправление precedes следующий PGO kernel.

## Исправление и поведение

Lowerer сохраняет внешний register/type/callable-alias/local-array binding в момент объявления локальной переменной и восстанавливает его при выходе. Присваивания внешним переменным до shadow declaration и без shadow сохраняются. Инициализатор оценивается во внешнем окружении; scalar/tuple/fold binding скрывает старую alias metadata, callable binding скрывает старый scalar. Вложенные массивы получают отдельное static storage при коллизии имени.

`break` и `continue` вычисляют аргументы ребра через покидаемые lexical frames на копиях maps. Это предотвращает перенос внутренней shadow переменной в loop-carried состояние и сохраняет окружение для lowering соседней ветви. Range и ForEach возвращают внешнее значение своей переменной после цикла; range end вычисляется до введения loop binder. Отдельный nested Block вокруг range body не позволяет пользовательскому shadow перенаправить increment.

## Проверка

21 handcrafted HIR case проходит HIR Verify, MIR VM и assembled Z80: разные типы shadow, вложенные blocks/branches/loops, mutation before declaration, return/break/continue, range end referring to outer binder, byte/word ForEach, callable aliases и их initializers, tuple bindings, array/scalar shadows. Машинный bootstrap использует actual PBQP parameter home, без принудительного назначения регистра.

Сквозной Nanz caller проверяет пять последовательных вызовов с результатами 8/10/10/12/16, обычные function calls, continue/shadow и u16 ForEach над resource. Обе настройки Grace проходят обычный pipeline/allocator/assembler/target harness: 4296 T и 100 bytes, exact одинаковый binary SHA256 e51976d5abffe55eaa5fe2c05268b1f9eba09e889b382086e8300241bf541467. Каждый resource byte прочитан один раз, записей нет. Это стоимость correctness workload, не новый выигрыш оптимизации. Этот source test не доказывает поддержку косвенного callable ABI: known callable alias cases отдельно построены в HIR.

HIR/MIR/Nanz проходят за 7,921/4,043/1,527 с; CLI за 18,442 с. Собранный CLI с новым lowerer повторно проверяет полный tokenizer: оба objective выбирают прежние exact binaries (speed 2259865T/4475B, size 2270286T/4346B). Все candidate state/resource-access сопоставлены независимым host, typed evidence SHA256 проверены.

Showcase сохраняет 31 прежнее падение, новых нет. Общий pipeline проходит за 238,234 с (только budget fixture вынесен в отдельный binary comparison). Compile-only budget comparison подтверждает точное равенство четырёх исходников ASM и собранных бинарников предыдущему этапу: 344/344/232/344 bytes. Это позволяет сохранить прежний 722,080-секундный machine gate для неизменных бинарников; новое исполнение budget workload не заявляется.

## Следующий профильный участок

[PC audit](grace-hir-scope-artifacts/tokenizer-pc-audit/README.md) исполняет прежний exact tokenizer baseline SHA256 `048b95267b15d26a4a71d0a7051a16dd12f5f66c36e9ce1c922b45d109babc33`. Сумма 2 270 286 T, runtime state/access/results и bootstrap совпадают с сохранёнными данными. Temporary Go overlay добавляет PC/cycle counters в harness и имеет отдельную host compatibility identity; machine binary не меняется.

| Физическая функция | Собственные T-states | Доля |
| --- | ---: | ---: |
| intern | 524 924 | 23,12% |
| scan | 458 306 | 20,19% |
| peek_char | 301 086 | 13,26% |
| region_eq | 224 008 | 9,87% |

Instruction-start PC относит CALL/RET к месту исполнения; время вложенных вызовов не включается повторно, generated split считается отдельно. Это измерение горячих участков, не результат новой оптимизации. Pascal variable-stride search требует отдельного legality proof с malformed lengths/wrap/traps; одного guard по str_bump недостаточно.

## Оставшиеся ограничения и артефакты

Отдельно воспроизведён прежний backward goto из shadow block: HIR Verify принимает его, но VM возвращает 99 вместо 7. Labels/goto через lexical scopes требуют собственного SSA edge/ancestry контракта и не исправлены этим structured-control этапом. Исторические probes сохранены, следующее исправление должно проверяться отдельно.

[Артефакты](grace-hir-scope-artifacts/README.md) сохраняют source, workload, raw/target state и traffic, HIR/MIR, assembly/binary, costs, patch, исходные failing probes, PC audit и журналы. Hardware и исходный extern tokenizer ABI остаются дальнейшими этапами.
