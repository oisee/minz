# PFCCO: реальная accumulator demand закрывает два residual примера

Разбор полного ASM уточнил природу residual `offset(x)=3+x`: это не различие числа PUSH/POP. Обе комбинации сохраняют два register pairs. General parameter leaf получает D (C занят константой 3), и codegen выполняет `LD A,C; ADD A,D`; caller дополнительно переносит C→D. При parameter A leaf выполняет `ADD A,C`, caller избегает второго adapter. Условный путь General также требует сохранения живого accumulator вокруг CMP. Предыдущий first-use heuristic ошибочно требовал General для rhs коммутативного сложения с literal lhs.

Исправление ограничено PFCCO/Z80 unary demand: first-use ADD/AND/OR/XOR с byte literal lhs и rhs parameter предпочитает A, как уже умеет backend. Вычитание/CMP, runtime lhs, u16 и другие CostTable сохраняют прежнюю модель. Backward CFG liveness запрещает считать A reuse бесплатным, если rhs жив после операции; copy recursion cycle-safe. Greedy и emitted semantics не менялись. Это конкретная body demand поправка, не общий call-save cost model и не heuristic по частоте.

## Проверки реального кода

| Пример | До / greedy | После PFCCO | Независимый native перебор |
|---|---:|---:|---:|
| 3+x, прямой | 44 288 T / 22 B | 43 264 T / 21 B | 43 264 T / 21 B |
| 3+x, условный | 49 585 T / 27 B | 46 026 T / 26 B | 46 026 T / 26 B |
| 2*x+3, прямой | 47 360 T / 25 B | 46 336 T / 24 B | 46 336 T / 24 B |
| 2*x+3, условный | 52 645 T / 30 B | 49 086 T / 29 B | 49 086 T / 29 B |

Реальный MIR module сохраняет C ABI первого callee и A returns. Независимо перечислены все девять комбинаций A/B/General двух свободных параметров; **каждая** собирается и исполняется на всех 256 входах с арифметическим oracle и восстановлением SP. Контракты применяются до allocation; учитываются все moves/saves. Первоначальные классы не меняют совместный выбор. Предыдущий residual test превращён в проверку реально доказанного bounded успеха, исходные before ASM сохранены.

Обычный Nanz source с реальным ASM C pin, условным вторым CALL и byte global trace проходит `Options.ContractStrategy="pbqp"` до LUT/preallocation: 84 327 T / 60 B greedy → 82 283 T / 58 B PFCCO. VM/native независимо согласны с `3*x+3` (x=0:0) и persistent state на всех 256 входах. Точный source/ASM/BIN сохранён в пакете.

Старый `2*x+3` full-source результат сохраняется: 87 387/63 → 85 343/61. Weighted packet остаётся 7 888 T / 52 B для greedy и PBQP; prospective heldout frozen test PASS. Нового profile choice/режима поиска нет.

## Границы

Активные отрицательные контроли: SUB/CMP, u16, runtime lhs, rhs с поздним использованием в том же блоке и successor block, циклы копий. Hard pins и byte adapter matrix продолжают проходить.

Общая цена сохранений, конкретное распределение General по C/D/E и реальная частота ветвей всё ещё не представлены полностью в class model. Совпадение с native optimum доказано только для перечисленного bounded domain. Full pipeline gate этой работой не запускался и PASS всего корпуса не заявляется. Проверки и SHA256 — `reports/grace-pfcco-literal-demand-artifacts/`.

Полный MIR2: PASS 8.253 s. Production pkg/cmd compile-only: PASS. Native bounded focused 1.864 s, ordinary source/admission 0.113 s; frozen prospective heldout 0.98 s.
