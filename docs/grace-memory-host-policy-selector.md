# Policy-aware memory-host association and selector v2

Selector v2 связывает одну явную backend-политику с исходным loop profile
и всеми четырьмя arena/append кандидатами. На неизменённом Self-Lanz
PreserveForSpeed выбирает оба преобразования: **1076715 T / 2100 B**.
Legacy выбирает тот же mask 3: **1079636 T / 2099 B**.
Старые v1 API и CLI продолжают использовать Legacy.

## Данные и replay

Association v2 содержит compile_options, original HIR loop profile и полный
baseline v2. Профиль описывает исходные VM-циклы; это не счётчики native
преобразованных циклов. Loader ограничивает вход 32 MiB, проверяет точный
JSON shape, дубликаты, case aliases, полноту, policy, исходник/workload/context,
суммы вызовов и native hash bindings. Некорректный schema/policy отклоняется
до source factory. Сохранённые costs остаются недоверенными до полного fresh
replay; совпадение whole envelope обязательно.

Каждый из четырёх кандидатов сохраняет baseline v2 с той же policy и точное
aggregate precompile proof по исходным source sites, functions, origins и SHA.
Исходные графы удерживаются сильными ссылками, включая удалённые подготовкой
узлы; проверяются 24 factory-вызова, отсутствие shared graphs и поздних мутаций.
MIR после allocation остаётся диагностикой. Memory-host/ISA permissions не менялись.

GoalSize сравнивает измеренные байты при фиксированной policy и не означает
pipeline OptSize. Поиск сразу между двумя backend-политиками не добавлен.

## Измерения

Два actual Speed selector запуска, восемь кандидатных бинарников и два
отдельных collection baseline: десять полноразмерных native запусков
по 30 вызовов. Тест PASS 36.20 s. Size-победители ниже вычислены из этих же
сохранённых данных; второй full30 GoalSize selector не запускался.
Короткие actual проверки обеих целей выполнены отдельно.

| Mask | Legacy T / B | PreserveForSpeed T / B |
| --- | ---: | ---: |
| Original | 1135370 / 2000 | 1132449 / 2001 |
| Arena | 1135211 / 2121 | 1132290 / 2122 |
| Append | 1079795 / 1978 | 1076874 / 1979 |
| Both | 1079636 / 2099 | 1076715 / 2100 |

Actual speed выбирает mask 3 в обеих политиках. Расчёт по сохранённым
бинарникам для size выбирает mask 2. Для этого workload каждый same-mask
переход policy 0→1 дал −2921 T и +1 B. Универсальная аддитивность или
победитель для любого workload не утверждаются.

Два независимых saved-only аудита проверили 300 native наблюдений:
240 кандидатных и 60 исходных, полные результаты/10 globals/3 ресурса,
3017 упорядоченных host events на каждый baseline, 11 original loop counts
с per-call/cumulative суммами, v2 envelopes, precompile proofs и native SHA.
Все четыре Legacy-кандидата точно совпали с текущими CLI-бинарниками.
Actual executable сохранён:
`502796375629b6422fdecc06b93be33c3ca56e2d9b3037e956e0df2ed1b16418`.

## Проверки и артефакты

Bounded selector тесты PASS 114.497 s: две policy, две цели, observed zero-body,
24-call lifecycle и forced GC, поздние мутации на 6/12/18/24, shared source,
подменённые counts/observations/proofs/envelopes. Preflight и unobserved/tie
проверки также прошли. Association тесты, включая indented JSON round trip,
PASS 3.649 s.

Первый full30 replay отказал из-за whitespace AllocationJSON после pretty
serialization. Сохранены вход, лог и executable отказавшего запуска.
Исправление нормализует только представление AllocationJSON после проверки
полного typed shape и compact SHA. Fresh replay equality не ослаблен.

[Манифест артефактов](grace-memory-host-policy-selector-artifacts/manifest.json):
source snapshots, whole profiles/selection, восемь ASM/BIN, MIR/allocation,
оба executable, логи и независимые аудиты. После переноса локальный audit
повторно PASS300. Код перенесён в рабочую ветку0dceafab и pushed.
HIR/MIR/Nanz/Lanz/C89/CLI прошли; showcase31same0new. Новый общий pipeline
ещё выполняется, успешный итог для него пока не заявляется.

Следующий этап — отдельный CLI v2 режим с явной named policy, whole
association для collect/replay и хешами сохранённых association/selection.
Старый CLI остаётся Legacy/v1. Настоящее Z80-железо пока не проверялось.
