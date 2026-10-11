# Закрытый memory-host baseline для исходного Self-Lanz

Исходный рекурсивный parser теперь исполняется через отдельный публичный
`RunZ80MemoryHostBaseline(factory, workload)`: verified raw MIR VM и собранный
Z80 сравниваются по возвращаемым значениям, всем globals, полным RAM-регионам
и порядку фактических byte events. Все30 вызовов проходят:1221494 T /2091 B,
BIN SHA256 fed98ba78becf9431a5301f31330363cc3005f1f6692fd1f9e9183d80b9d0a51.
Независимая Python host-модель подтверждает весь workload. Это основание
для следующего parser PGO; оптимизация parser и выигрыш ещё не заявляются.

## Контракт исполнения

Отдельная схема `minz.memory-host-workload/v1` содержит только закрытые builtin
read-u8-at-u16/write-u8-at-u16 с точными сигнатурами. Workload владеет байтами
регионов, initial globals, reset/patch, literal calls и layout. Настройка вне
замера; память сохраняется между вызовами согласно descriptor. Старые no-host
profile API продолжают отклонять extern, ASM, ports и чужую схему.

Весь исходник сохраняется, включая неиспользуемую печать. Она разрешена только
в недостижимых функциях полного direct-call closure; исполняемые инструкции
ограничены этим closure. Все extern declarations обязаны совпасть с builtin set.
Factory выдаёт независимые HIR; совместимость, source units и точные origins
сверяются для новых и ранее сохранённых объектов до/после компиляции и исполнения.

VM разрешает обычные globals только по происхождению выполняемой инструкции
от symbol+bounded constant offset. Неизвестные числовые указатели проверяются
по объявленным регионам. В Z80 каждая разрешённая операция global/spill связана
с финальным PC, исходной MIR-операцией или compiler-owned spill, symbol,
смещением, шириной и направлением. Actual opcode декодируется независимо;
перед каждым байтом проверяется ограниченный budget операции. Inline markers
не изменили измеренный BIN. Orphan TSMC-store переносит identity только после
точной замены на обычный spill-store; настоящая code patch не разрешается.

Стек проверяется по actual PUSH/POP/CALL/RET, pre/post SP, границам всех байтов,
инициализированным байтам, CALL return bytes и RET frame. Доступ проверяется
до изменения памяти; same-value stores наблюдаются. Numeric globals/stack/spill
без подтверждённой операции отклоняются. Границы кода, данных и stack не пересекаются.

Типизированный отказ builtin содержит причину, адрес и направление; совпадает
и успешный префикс эффектов. Инфраструктурные ошибки, нехватка gas/steps,
неподдержанные native операции и прямые permission traps отклоняют attestation.
Первый baseline поддерживает неуспешное исполнение только по closed host denials.
Универсальная эквивалентность ошибок и volatile/port/hardware ABI не заявляются.

## Проверки и сохранённые данные

Независимые проверки обнаружили и сохранили numeric global/stack/spill обходы,
а также PUSH за нижнюю границу stack с записью в соседний ресурс. После
исправления проверяются до-effect отказ, повторный budget при посещении loop,
partial word prefix, forged metadata/origins/sidecar, точная причина отказа,
поддельный workload, stale contexts, ABI, integer bounds и255/256/257-byte atoms.

Сохраняются исходник, workload, source compatibility, raw VM snapshots,
actual ASM/BIN, resolved sidecar, allocation JSON и SHA, финальный MIR dump и SHA.
Postallocation MIR допускает coalesced регистры: strict SSA Verify для него
не отключён; verified postallocation VM certificate не заявляется. Dump SHA
идентифицирует диагностическую сериализацию, а независимый oracle проверяет
raw source и настоящий target. Global final bytes проверены; независимый exact
traffic globals не заявляется. Exact ordered events относятся к RAM-регионам.

Интегрированные HIR/MIR/Nanz/C89 и CLI проходят. Showcase31 прежнее падение,
0 новых. Fresh native CLI audits прошли для всех16 кандидатов; их ASM/BIN и
измеренные затраты совпали с предыдущим HL-этапом. Hashmap123708 T /965 B;
tokenizer speed2174363 T /4820 B, size2240918 T /4305 B. Интегрированный
public baseline exporter прошёл1.377 s. Полный pipeline прошёл289.268 s.
Бюджетная компиляция четырёх вариантов прошла236.478 s; свежий assembler
подтвердил точные прежние ASM/BIN344/344/232/344 B. Машинный gate722.080 s
сохранён по идентичности байтов. Независимый typed-аудитор проверил90 записей
sidecar, allocation, MIR и ASM/BIN; независимая host-модель — все30 вызовов. [Артефакты](grace-memory-host-artifacts/manifest.json).

Следующий шаг — отдельно доказать host-aware arena-first сравнение строк,
измерить полный parser workload и выбирать speed/size по actual binaries.
PC baseline указывает intern_region334605 T (27.39%); существующие typed
copy/region/RW recognizers исходный extern-based parser ещё не принимают.
