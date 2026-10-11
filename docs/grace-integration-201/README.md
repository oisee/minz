# Root replay исходных семейств

На интегрированном checkpoint 1ce8f63a повторены независимые VM/Z80 проверки исходных recursion/fold и Lisp/Lanz модулей: PASS за 26,604 с. В эту команду также вошли diagnostic cond_test и исправленный TestMultiFunctionProfileVMZ80. Struct/interface проверен отдельной точной командой: PASS за 8,424 с.

Рекурсивный fib ограничен входами 0–20; прочие домены указаны в prospective freeze каждого пакета. cond_test остаётся пробелом семантики возвращаемого значения: зелёный diagnostic проверяет наблюдение сбоя и не доказывает правильность исходной программы. sum_to(255) в Lanz подтверждён как незавершающийся по исходному byte-counter циклу. Дополнительный выигрыш Grace на этих семействах не обнаружен.

На шаре и в tracked repository проверены SHA256 всех payload: profile reference — 19, recursion/fold — 149, Lisp/Lanz — 70, struct/interface — 121. Полные пакеты находятся в compiler-docs и compiler-reports внутри /mnt/safe/minz/grace/2026-10-11.

Старый полный pipeline cfaa7777 завершился exit 1 с восемью top-level failures. Семь stale reference failures исправлены позже; Manhattan observer проверяется отдельно. Новый полный pipeline ещё не запускался. Последний сохранённый полный CLI PASS относится к 11105fb3; test-only новые семейства не подменяют его checkpoint.
