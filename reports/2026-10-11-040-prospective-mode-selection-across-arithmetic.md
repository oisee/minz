# Выбор режимов: отрицательный tail и положительный PFCCO heldout

`ac29ad82` добавляет переиспользуемый measured API: четыре сочетания
ContractOpt и MIR Grace, отдельные цели Speed/Size, выбор по строгому улучшению,
фиксированный порядок при равенстве. Каждый кандидат исполняется на Z80;
ошибка любого кандидата отменяет весь выбор. Связываются source/HIR/ABI,
параметры компиляции, фактические ASM/BIN и executable. Аудит защиты от поздней
мутации возвращаемого MIR ещё продолжается, поэтому общий proof gate не завершён.

Первый prospective эксперимент на исходном tail-recursion честно отрицательный:
training original 25492 T / 174 B, Grace 26808 T / 214 B. Оба goals выбирают
original; тот же BIN на 24 heldout calls даёт 470109 T и проходит независимые
Fibonacci/factorial/triangle/power модели modulo256. Root repeat всех API и
tail тестов PASS 2.413 s; portable Python audit PASS.

Второй исходник — pfcco_weighted_packet. Новые пары зафиксированы до измерений
и исключают все 17 прежних известных пар. `131b32f8`: пять training pairs
2605→2320 T при 52 B; Speed выбирает ContractOpt on, MIR Grace off.
Тот же BIN на 12 heldout pairs: 6252→5568 T, −10.94%, 52 B.
Size сохраняет original по равенству размера. Независимая модель проверяет
результаты, state counter и byte accesses. Root repeat и Python audit PASS.
Это третье семейство — арифметика с реальными вызовами, но ограниченный workload
двух функций; весь набор приложений не объявляется проверенным.

[Tail](../docs/grace-arithmetic-heldout/REPORT.md),
[PFCCO](../docs/grace-pfcco-prospective-heldout/REPORT.md),
[root API repeat](../docs/grace-compiler-modes-integrated.txt),
[root PFCCO repeat](../docs/grace-pfcco-heldout-integrated.txt).
Исторические frozen executables и все пакеты сохранены на шаре с SHA256.
Полный pipeline checkpoint cb587fe4 всё ещё работает. Цель активна.
