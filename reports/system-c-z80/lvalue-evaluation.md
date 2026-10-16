# C: одно вычисление адреса при обновлении объекта

16 октября 2026 года. Ветка `feat/system-c-z80`; полный системный этап
и интеграция в main ещё не завершены.

## Ошибка и исправление

```c
unsigned char *pick(void) { calls++; return &bytes[0]; }
*pick() += 2;
```

По C99 §6.5.16.2 compound assignment вычисляет lvalue один раз. Старый HIR
дублировал дерево адреса для чтения и записи: pick вызывался дважды.
Pre/post increment и значение assignment expression добавляли третье
вычисление или повторное чтение объекта. Raw VM и native Z80 воспроизвели
расхождения независимо от оптимизаций.

Lowerer теперь отдельно сохраняет вычисленный адрес объекта во временном
значении. Чтение и запись используют этот адрес. Increment/decrement также
сохраняет исходное значение: postfix возвращает его, prefix — результат
обновления. Assignment expression возвращает converted assigned value,
не перечитывая destination. Это важно для volatile: `result=(device=3)`
должен записать device, но не читать его обратно.

Сохранённые адреса и значения — обычный типизированный HIR; отдельный runtime
helper, дополнительные DI/EI или изменение публичного ABI не вводятся.

## Проверка

12 семейств × 15 входов × default/Grace/Size: 540 native и 1080 raw/final VM
запусков. Оракул независимо проверяет число вызовов, итоговую запись,
преобразование byte/word и результат выражения. Покрыты compound assignment,
pre/post increment/decrement, pointer-returning call, struct field, index call,
pointer walk с postfix, nested assignment и aggregate assignment expression.
Native проверяет четыре байта результата, PC/SP и 32 защитных байта;
VM проверяет сохранность input. Aggregate expression дополнительно проверяет
скопированные head/tail и оба значения поля.

В существующую volatile-матрицу добавлены шесть сценариев: assignment,
compound assignment, pre/post increment и decrement как выражения.
18 native и 36 VM запусков сверяют точное число и порядок байтовых событий:
assignment — только write; остальные — один read и один write.
Существующие volatile-сценарии также повторно прошли.

```sh
go -C minzc test ./pkg/c89 -run '^TestC(LvalueSingleEvaluationNativeVM|VolatileAccessNativeVM)$' -count=1
go -C minzc test ./pkg/pipeline -run '^TestSystemCFAT12ImageNativeVM$' -count=1
```

FAT12 повторно прошёл все восемь workload в трёх режимах: 24 native и
48 raw/final VM исполнения, включая независимый FatFS oracle и trace диска.
Короткий core HIR/MIR2/C89/cparse/Nanz/interop прошёл: 38.281/86.889/
33.270/0.004/34.299/0.002 секунды. Все 12 новых семейств и volatile-матрица
повторно прошли совместный итоговый directed-прогон. Полный fast pipeline
(432 проверки, три процесса) ещё выполняется; его результат сюда не включён.

36 комплектов source/HIR/ASM/BIN экспортируются через `SYSTEM_C_LVALUE_EXPORT`.
Артефакты:
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/system-c-z80/lvalue-evaluation/`.
