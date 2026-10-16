# Старые обещания MinZ: что можно подтвердить и перенести

16 октября 2026 года. Рабочий журнал, не заявление о завершённом переносе VIR.
Обновление: описанный abs_diff теперь получается автоматически из C и Nanz;
полная матрица и основные пакеты прошли; проход интегрирован через PR #178
(71119f5b).

## Первый проверенный приём: вычитание вместо сравнения

Старый [VIR draft](../research/abi-paper/vir-solver-draft.md) описывал:

```asm
; x в A, y в C, результат в A
SUB C
RET NC
NEG
RET
```

SUB одновременно вычисляет d = x−y по модулю 256 и устанавливает carry при x<y.
Без заёма d — нужная абсолютная разность. При заёме NEG даёт y−x по модулю 256.
Равенство тоже корректно: SUB даёт ноль без carry. Исходные x и y беззнаковые;
это не правило для произвольного signed abs, где есть отдельная граница переполнения.

Старый текст называл это четырьмя байтами и утверждал доказанную глобальную
оптимальность. Это **четыре инструкции, пять байтов**: NEG имеет двухбайтовый
код ED 44. Доказательство эквивалентности не доказывает глобальную оптимальность.

Проверено на новом native-стенде: все 65 536 пары байтов, четыре начальных
значения F (00, 01, 80, FF), по 262 144 запуска каждого варианта. Оракул —
целочисленная абсолютная разность, отдельно проверены неизменность входов,
16-битный результат в памяти и PC/SP возврата.

Меняется только тело функции в сохранённом ASM. Вызывающая функция, данные,
ABI и подмножество 2048 входов совпадают с полным текущим бенчмарком.

| Вариант полного модуля | Байты | T-states на 2048 входах |
|---|---:|---:|
| C/Nanz, byte narrowing + load fusion | 36 | 289 366 |
| Тот же модуль, ручное тело abs_diff | **30** | **253 672** |
| SDCC 4.5 speed, замороженный C | 33 | 258 142 |

Ручная версия меньше SDCC на 3 байта и быстрее на 4470 T (около 1,73%).
Это результат ручной замены, **не результат автоматического MinZ C/Nanz**.
Функция уменьшилась 11 → 5 байт; выигрыш измерен вместе с прежней ценой caller.
Полный ручной домен проверен native; raw/final MIR не выражает эту ручную замену.

[Измерения](old-claims/hand-abs-summary.json), [ручной ASM](old-claims/hand-abs.a80),
[исходный ASM](c89-byte-load-fusion/program.a80).
Воспроизведение из корня репозитория:

```sh
go -C minzc run ../research/c89-benchmark/hand-abs/main.go \
  --asm ../reports/c89-byte-load-fusion/program.a80 --out /tmp/minz-hand-abs
```

## Почему обычный проход не сделал это сам

В [VIR](../minzc/pkg/vir/pipeline.go) есть fuseAbsDiffASM: сопоставление готового
ASM с шаблоном. В [MIR](../minzc/pkg/mir2/absdiff.go) есть FuseAbsDiff, но он
ожидает соседние compare/sub в одном блоке. Нынешний CFG содержит compare
в заголовке и два sub в разных возвращающих блоках.

CondRetSink не переносит такое вычитание через carry-based comparison без
доказательства сохранности flags. Этот барьер сохранён. Вместо его снятия
добавлен [FuseByteAbsDiffCFG](../minzc/pkg/mir2/absdiff_cfg.go), который доказывает
целый маленький CFG: два скалярных возврата, противоположные u8-вычитания,
одинаковые исходные значения, единственные определения, отсутствие эффектов
в возвращающих ветках и наблюдений flags, допустимые рёбра и происхождение.
Имя функции не участвует в правиле.

Проход запускается после DSE, когда становятся видны два оставшихся вычитания,
в обычном и Grace-пути. Использует существующие SUB, CmpSubCarry, TermCondRet
и NEG; новых обязательных зависимостей нет. Равенство допускает как строгие,
так и нестрогие unsigned-предикаты, поскольку обе исходные ветки возвращают ноль.
Signed, word, hard constraints, shared-блоки, неоднозначные определения,
дополнительные эффекты, несовместимое происхождение, multi-return и fallible/flag ABI
отвергаются. Для Nanz без SrcTy байтовый тип выводится из определений операндов;
синтетическому carry-предикату ширина задаётся явно.

## Автоматический результат: оба режима SDCC

Повторены все **69 сборок**, **66 PASS**, те же три неподдерживаемых случая.
На Z80 выполнены 75 350 вызовов всего набора. Изменилась только строка abs_diff
у MinZ C и Nanz во всех трёх профилях; остальные строки не изменились.
Исходники и нагрузки остаются замороженными.

| Полный модуль abs_diff | Режим | Байты | T-states |
|---|---|---:|---:|
| MinZ, одинаковый C | default / Size / Grace | **30** | **253 672** |
| MinZ, отдельный Nanz | default / Size / Grace | **30** | **253 672** |
| SDCC 4.2 | Size / Speed | 34 | 264 286 |
| SDCC 4.5 | Size / Speed | 33 | 258 142 |
| sccz80 | O3 | 87 | 696 320 |

Size и Speed SDCC запускались отдельно через --opt-code-size/--opt-code-speed.
Они совпали на abs_diff; это не означает совпадения на других программах.
Все отдельные профили и оставшиеся проигрыши сохранены в
[новой полной матрице](old-claims/automatic-summary.json) и
[версиях инструментов](old-claims/automatic-toolchains.json).
[Автоматический ASM](old-claims/automatic.a80),
[raw MIR](old-claims/automatic-raw.mir), [final MIR](old-claims/automatic-final.mir).

Проверки: четыре unsigned-предиката × полный байтовый домен до/после на MIR VM;
18 отказов с проверкой отсутствия частичной мутации. Два frontend-исходника
с новым именем distance × три режима × 65 536 пары: **393 216 native Z80-запусков
и 786 432 raw/final MIR VM-вызовов**, с памятью и PC/SP. Это дополнительные
source-проверки, не новое независимое семейство алгоритмов.
Полные MIR2/HIR/C89/Nanz пакеты прошли (90,895 / 36,646 / 61,227 / 33,394 s).
Все 433 fast pipeline-проверки прошли в трёх независимых процессах:
[статусы](old-claims/automatic-fast-statuses.json). Deep/release PASS не заявлен.

На этом примере старую идею удалось перенести доказательно в обычный compiler.
Глобальная оптимальность по всем Z80-программам по-прежнему не заявляется.

## Остальные старые обещания: очередь проверки

| Обещание | Что сейчас известно | Что ещё нужно доказать |
|---|---|---|
| Константные fib10/sum10/fact6 и demo | Старый [C89 отчёт](2026-03-20-104-C89-vs-SDCC-Extended-Benchmark.md) использует фиксированные аргументы и считает инструкции | Пересобрать тот C, проверить native результат, размеры всего модуля и действительную цену caller |
| Условные RET | CondRetSink существует; защита flags влияет на допуск | Переносить только при доказанной эквивалентности условия после вычислений |
| Tail call CALL/RET → JP | Приём реален при совместимом ABI | Проверить стек, живые значения, fallible/indirect вызовы и перестановки аргументов |
| Обмен без инструкций | PFCCO может согласовать назначения caller/callee | Считать весь call site; переименование виртуальных значений не гарантирует отсутствие физических пересылок |
| VIR/Z3 выигрывает все примеры | Старый исследовательский путь отличается от обычного MIR2 → Z80 | Воспроизвести исходники, версии и измерения; переносить доказанные идеи без обязательной зависимости от Z3 |

Ранее были и отчёты, где SDCC выигрывал: [январский разбор](../docs/2026-01-02-260-MinZ_vs_SDCC_Benchmark.md)
показывает слабые простые функции MinZ. Его сравнение строк ASM тоже не заменяет
измерение байтов и тактов. История не подтверждает единого тезиса «раньше всегда побеждали».

В текущей [полной матрице](c89-byte-load-fusion/summary.json) Nanz + Grace gcd
уже быстрее и компактнее SDCC 4.5 speed: 54 байта / 3 099 781 T против
56 / 4 243 840. Это отдельные эквивалентные исходники; язык, ABI и оптимизатор
здесь действуют совместно. Сравнение одинакового C остаётся отдельной категорией.


## Повтор старого C-бенчмарка: что действительно работает сегодня

Повтор выполнен на checkpoint 78e10200. Все функции исторического
[sdcc_benchmark.c](../examples/c89/sdcc_benchmark.c) оставлены без изменений.
К каждой копии добавлена одинаковая для всех C-компиляторов обвязка execute():
глобальные входы, вызов нужной функции и запись результата. Оригинальный SHA,
полные исходники с обвязкой, входы и ожидаемые значения заморожены до измерений
[в manifest](../research/c89-benchmark/old-claims/cases/manifest.json).
Это повтор приёмов старого текста на сегодняшнем коде, а не восстановление старого binary.

**88 / 88 сборок PASS**: 11 примеров × восемь режимов, включая оба SDCC 4.2/4.5,
sccz80 O3 и MinZ default/size/Grace. Выполнено **89 456 native Z80 вызовов**,
**67 092 raw/final MIR VM вызова** для MinZ. Проверены результаты, заданные входы
и PC/SP возврата. Независимый GCC C89-host подтвердил **11 182 ожидания**:
[протокол](old-claims/replay/host-oracle.json). Входы word-арифметики избегают signed
переполнения int16; это не полный 16-битный домен. Для byte min/abs используются
все x и восемь граничных y; clamp покрывает все x и 16 пар lo/hi, включая lo>hi.
Случай lo>hi проверяется по порядку исходных if, а не по догадке о «правильном clamp».

### Успехи и цена полного модуля

Ячейка — **байты полного модуля / T всей нагрузки**. Между строками нагрузки
различаются: четыре константных примера содержат по одному вызову, остальные —
замороженные наборы входов. Все generic-функции исторического файла сохранены;
неиспользуемые generic-функции занимают место, хотя не участвуют в вызове execute().
MinZ Size/Grace совпали с default во всех этих строках и сохранены в JSON.

| Вызов | MinZ default | SDCC 4.2 Size | SDCC 4.2 Speed | SDCC 4.5 Size | SDCC 4.5 Speed | sccz80 O3 |
|---|---:|---:|---:|---:|---:|---:|
| [fib10](../research/c89-benchmark/old-claims/cases/old_fib10.c) | [249 / 56](old-claims/replay/minz/default/old_fib10/program.a80) | [225 / 1 327](old-claims/replay/sdcc/size/old_fib10/program.asm) | [230 / 1 307](old-claims/replay/sdcc/speed/old_fib10/program.asm) | [214 / 1 327](old-claims/replay/sdcc45/size/old_fib10/program.asm) | [219 / 1 307](old-claims/replay/sdcc45/speed/old_fib10/program.asm) | [618 / 7 351](old-claims/replay/sccz80/o3/old_fib10/program.asm) |
| [sum10](../research/c89-benchmark/old-claims/cases/old_sum10.c) | [249 / 56](old-claims/replay/minz/default/old_sum10/program.a80) | [225 / 791](old-claims/replay/sdcc/size/old_sum10/program.asm) | [230 / 771](old-claims/replay/sdcc/speed/old_sum10/program.asm) | [214 / 789](old-claims/replay/sdcc45/size/old_sum10/program.asm) | [219 / 769](old-claims/replay/sdcc45/speed/old_sum10/program.asm) | [618 / 5 868](old-claims/replay/sccz80/o3/old_sum10/program.asm) |
| [fact6](../research/c89-benchmark/old-claims/cases/old_fact6.c) | [249 / 56](old-claims/replay/minz/default/old_fact6/program.a80) | [225 / 3 338](old-claims/replay/sdcc/size/old_fact6/program.asm) | [230 / 3 328](old-claims/replay/sdcc/speed/old_fact6/program.asm) | [214 / 3 294](old-claims/replay/sdcc45/size/old_fact6/program.asm) | [219 / 3 284](old-claims/replay/sdcc45/speed/old_fact6/program.asm) | [618 / 5 482](old-claims/replay/sccz80/o3/old_fact6/program.asm) |
| [demo](../research/c89-benchmark/old-claims/cases/old_demo.c) | [249 / 56](old-claims/replay/minz/default/old_demo/program.a80) | [225 / 638](old-claims/replay/sdcc/size/old_demo/program.asm) | [230 / 638](old-claims/replay/sdcc/speed/old_demo/program.asm) | [214 / 638](old-claims/replay/sdcc45/size/old_demo/program.asm) | [219 / 638](old-claims/replay/sdcc45/speed/old_demo/program.asm) | [618 / 862](old-claims/replay/sccz80/o3/old_demo/program.asm) |
| [twice](../research/c89-benchmark/old-claims/cases/old_twice.c) | [283 / 10 854](old-claims/replay/minz/default/old_twice/program.a80) | [230 / 11 792](old-claims/replay/sdcc/size/old_twice/program.asm) | [235 / 11 792](old-claims/replay/sdcc/speed/old_twice/program.asm) | [219 / 11 792](old-claims/replay/sdcc45/size/old_twice/program.asm) | [224 / 11 792](old-claims/replay/sdcc45/speed/old_twice/program.asm) | [625 / 20 234](old-claims/replay/sccz80/o3/old_twice/program.asm) |
| [add](../research/c89-benchmark/old-claims/cases/old_add.c) | [292 / 67 670](old-claims/replay/minz/default/old_add/program.a80) | [236 / 72 360](old-claims/replay/sdcc/size/old_add/program.asm) | [241 / 72 360](old-claims/replay/sdcc/speed/old_add/program.asm) | [225 / 72 360](old-claims/replay/sdcc45/size/old_add/program.asm) | [230 / 72 360](old-claims/replay/sdcc45/speed/old_add/program.asm) | [632 / 212 390](old-claims/replay/sccz80/o3/old_add/program.asm) |
| [abs_diff](../research/c89-benchmark/old-claims/cases/old_abs_diff.c) | [278 / 253 672](old-claims/replay/minz/default/old_abs_diff/program.a80) | [238 / 264 286](old-claims/replay/sdcc/size/old_abs_diff/program.asm) | [243 / 264 286](old-claims/replay/sdcc/speed/old_abs_diff/program.asm) | [226 / 258 142](old-claims/replay/sdcc45/size/old_abs_diff/program.asm) | [231 / 258 142](old-claims/replay/sdcc45/speed/old_abs_diff/program.asm) | [634 / 696 320](old-claims/replay/sccz80/o3/old_abs_diff/program.asm) |
| [clamp](../research/c89-benchmark/old-claims/cases/old_clamp.c) | [288 / 770 122](old-claims/replay/minz/default/old_clamp/program.a80) | [244 / 860 027](old-claims/replay/sdcc/size/old_clamp/program.asm) | [249 / 853 513](old-claims/replay/sdcc/speed/old_clamp/program.asm) | [232 / 851 370](old-claims/replay/sdcc45/size/old_clamp/program.asm) | [237 / 844 856](old-claims/replay/sdcc45/speed/old_clamp/program.asm) | [642 / 1 550 708](old-claims/replay/sccz80/o3/old_clamp/program.asm) |
| [min8](../research/c89-benchmark/old-claims/cases/old_min8.c) | [279 / 261 648](old-claims/replay/minz/default/old_min8/program.a80) | [238 / 249 802](old-claims/replay/sdcc/size/old_min8/program.asm) | [243 / 249 802](old-claims/replay/sdcc/speed/old_min8/program.asm) | [226 / 218 640](old-claims/replay/sdcc45/size/old_min8/program.asm) | [231 / 218 640](old-claims/replay/sdcc45/speed/old_min8/program.asm) | [634 / 571 392](old-claims/replay/sccz80/o3/old_min8/program.asm) |
| [wrap_min](../research/c89-benchmark/old-claims/cases/old_wrap_min.c) | [278 / 282 128](old-claims/replay/minz/default/old_wrap_min/program.a80) | [238 / 278 474](old-claims/replay/sdcc/size/old_wrap_min/program.asm) | [243 / 278 474](old-claims/replay/sdcc/speed/old_wrap_min/program.asm) | [226 / 239 120](old-claims/replay/sdcc45/size/old_wrap_min/program.asm) | [231 / 239 120](old-claims/replay/sdcc45/speed/old_wrap_min/program.asm) | [634 / 870 400](old-claims/replay/sccz80/o3/old_wrap_min/program.asm) |
| [wrap_twice](../research/c89-benchmark/old-claims/cases/old_wrap_twice.c) | [285 / 14 472](old-claims/replay/minz/default/old_wrap_twice/program.a80) | [230 / 13 132](old-claims/replay/sdcc/size/old_wrap_twice/program.asm) | [235 / 13 132](old-claims/replay/sdcc/speed/old_wrap_twice/program.asm) | [219 / 13 132](old-claims/replay/sdcc45/size/old_wrap_twice/program.asm) | [224 / 13 132](old-claims/replay/sdcc45/speed/old_wrap_twice/program.asm) | [625 / 32 294](old-claims/replay/sccz80/o3/old_wrap_twice/program.asm) |

Полная [матрица](old-claims/replay/summary.json), [версии и SHA runner](old-claims/replay/toolchains.json),
[манифест опубликованных артефактов](old-claims/replay/manifest.json).
Рядом с каждым ASM сохранены costs.json, команды; рядом с MinZ — raw/final MIR.
Подробные наблюдения и binary сохранены локально в
/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/c89-nanz-sdcc45/old-claims-replay/.

**Свёртка констант подтверждена**, но старая фраза «выигрывает каждый пример»
неверна как современный вывод. Все полные модули MinZ в этой матрице больше SDCC
4.5; вызовы min8, wrap_min и wrap_twice тоже медленнее его. Рекламное сравнение
инструкций отдельных функций пропускало caller, сохранённые generic-функции и helpers.
Отдельный маленький abs_diff-модуль из новой основной матрицы действительно выигрывает
у SDCC; здесь тот же abs_diff окружён всем старым исходником и размер уже другой.

### Какие старые приёмы воспроизвелись

```asm
fib10:           ; 4 байта, 20T самой функции
    LD HL, 55
    RET
sum10:
    LD HL, 45
    RET
fact6:
    LD HL, 720
    RET
demo:
    LD HL, 26
    RET

twice:           ; вход/выход HL, 2 байта, 21T
    ADD HL, HL
    RET
add:             ; вход HL/DE, выход HL
    ADD HL, DE
    RET

min8:
    CP B
    RET C
    LD A, B
    RET

wrap_min:
    LD B, C      ; контракты wrapper/callee ещё различаются
    JP min8      ; настоящий tailcall, без нового адреса возврата

wrap_twice:
    ADD HL, HL   ; после inlining вызова уже нет
    RET
```

20T загрузки-константы/RET — цена самой функции; обвязка execute() в четырёх
измеренных случаях сворачивает ещё и её вызов, и весь execute() занимает 56T.
Обмен регистрами между caller/callee не обязателен: twice/add уже возвращают в HL,
но это не доказательство «нулевой цены любого ABI». Wrap_min показывает реальный
оставшийся LD B,C. EX DE,HL тоже не бесплатен: 4T.

Clamping использует короткие условные возвраты, однако современный ASM отличается
от старого обещания девяти инструкций. Безопасность flags проверяется совместным
исполнением, барьер CondRetSink не снят. Tailcall в wrap_min подтверждён PC/SP;
wrap_twice достиг короткого тела другим способом — inlining.

### Следующее выжимание

Первый конкретный резерв — глобальные stores и согласование call site. У min8 тело
уже компактно, но execute() расширяет байт через DE и пишет word через HL побайтно.
У wrap_twice лишняя цена вызова остаётся в execute(), хотя тело wrapper всего из
двух инструкций. Нужно измерять caller вместе с callee, иначе короткое тело
превратится в очередную рекламную цифру.

Общий word store уже умеет LD (symbol),HL; рассмотрим безопасный допуск других
16-битных пар Z80 и подавление одноразового AddrOf. Отдельно проверим согласование
PFCCO-контрактов для tailcalls: смена назначения может убрать LD B,C, но должна
сохранять стек, живые значения и внешнее ABI. Это следующие кандидаты, ещё не новые
включённые оптимизации данного checkpoint.

Воспроизведение:

```sh
python3 research/c89-benchmark/old-claims/check_host.py --out /tmp/old-host.json
python3 research/c89-benchmark/old-claims/run.py \
  --out /tmp/minz-old-claims --toolchains /path/to/toolchains --go /path/to/go
```
