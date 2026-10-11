# Исходные factorial/fib/fold: VM и native Z80

Сохранены без изменений шесть оригиналов 2026-03-11: ex9a_factorial_rec,
ex9b_factorial_fold, ex10a_fib_rec, ex10b_fib_iter, ex10c_fib_fold,
ex14_fold_assert. Все соответствующие файлы 2026-03-13 прочитаны отдельно
и имеют тот же SHA; обе датированные копии сохранены. В ex14 проверены
все три функции, включая memory consumer count_in_range.

До выполнения frozen-domain.json фиксирует исходники, SHA, домены и
независимые оракулы. Factorial вычисляется произведением modulo65536,
Fibonacci — независимой итеративной рекуррентностью modulo65536.
Все byte-входы 0..255 проверены для factorial_rec/factorial_fold,
fib_iter/fib_fold, sum_to и factorial из ex14. Exponential fib_rec проверен
на префиксе 0..20, включая заявленную исходником практическую границу.
Нет утверждения о native fib_rec для остальных 235 byte-входов.

Language Book v7 §2.5 показывает explicit `for i in 0..n`; lowerForRange
задаёт возрастающий half-open цикл. Book §6.1 задаёт другую семантику
`range(lo..hi).fold`: countdown hi-lo..1. В этих оригиналах используется
explicit for, поэтому countdown-оракул к ним не применяется.
На n255 выражение n+1 типа u8 заворачивается в0: factorial_fold и ex14
factorial возвращают1 без итераций, sum_to возвращает0. Fib_fold(0)
возвращает1 по начальному b и пустому циклу; fib_iter(0) возвращает0.
Исходный комментарий «12! fits u16» неверен; применено modulo65536,
а не ожидание из комментария.

Для count_in_range: все len0..255, четыре seed0/1/127/255, десять заранее
заданных пар lo/hi, включая обратные границы, все/один байт и промежутки
через 127/128. Массив byte(seed+73*i), i=-1..256, PTR0x4000.
10 240 вызовов на режим; оракул считает lo<=byte<=hi и проверяет ровно len
чтений в возрастающем порядке индекса. Домен конечный, не все массивы или
все 65 536 пары lo/hi. Исходные asserts сохраняются и проверяются обычной
компиляцией; module/entries не заменены специальными helpers.

Режимы ordinary/GraceSize/GraceSpeed с ContractOpt=true. Native выполняет
настоящий CALL и останавливается только на исходном PC возврата с исходным
SP. Проверяются return contract, raw/final VM, значения изменяемых globals,
неизменность прочих globals и массива/guards. Полная RAM и code неизменны
вне точно определённых source globals, типизированных spill-байтов и
наблюдаемого стека; profiler запрещает любые native записи вне этой области.
VM разрешены только исходные global accesses и ordered array reads.

Это локальная проверка перечисленных неизменённых исходников, не общий
pipeline/CLI gate. Native имеет лимит 20 млн инструкций на один вызов;
cumulative VM gas отключён лишь для явно конечного фиксированного домена.

Результаты ordinary → Grace Size/Speed:

| Модуль | BIN, байт | T-states всего домена | Стек, байт (ordinary) |
|---|---:|---:|---:|
| ex9a_factorial_rec | 54 → 54 | 37375924 → 37375924 | 1020 |
| ex9b_factorial_fold | 67 → 67 | 37439733 → 37439733 | 8 |
| ex10a_fib_rec | 38 → 38 | 6510772 → 6510772 | 78 |
| ex10b_fib_iter | 32 → 32 | 2419979 → 2419979 | 4 |
| ex10c_fib_fold | 19 → 19 | 2249669 → 2249669 | 4 |
| ex14_fold_assert | 183 → 183 | 353266714 → 353266714 | 8 |

Все значения и memory/state проверки совпали с независимыми оракулами;
miscompile не найдено. Сохранены source/ASM/BIN/raw-final MIR/ABI/символы,
CALL-драйверы, логи и исполняемый тест с полным SHA manifest.
`first-test.txt` — предварительная проверка до усиления ordered access и
untouched globals. Авторитетный итог — `complete-test.txt`.

Полный retained-executable gate: **PASS, 17,54 s**, 35 391 native CALL
во всех трёх режимах. У factorial_rec все 256 входов, максимум стека
1020 байт при n255; fib_rec 0..20, максимум78 байт. Ни один из этих
результатов не обобщается на другие программы или recursion domains.
Frozen-domain исторически ссылается на chapter4 для explicit for; правильная
ссылка — Book v7 §2.5. Замороженный файл не изменён после измерения.

Retained executable SHA256:
`f68d70f360b32fc0ec3c87d23b0412a4e3d0fdd3dc7cfc4bea81b0ee17414086`.
Воспроизведение из minzc/pkg/pipeline:

```sh
RECURSION_FOLD_EXPORT=/absolute/new-output ../../../reports/original-recursion-fold-corpus/pipeline.test \
  -test.run '^TestOriginalRecursionFoldCorpusComplete$' -test.count=1 -test.v
```
