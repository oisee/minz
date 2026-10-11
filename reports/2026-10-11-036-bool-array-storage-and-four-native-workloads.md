# Bool array storage и дополнительные native workloads

Интегрирован `ee21cae8`: ширина массива теперь учитывает отдельно адресуемый
byte каждого элемента. Значащая ширина scalar bool остаётся один bit. Реальный
Pascal sieve прежде выходил за границы VM heap; исправление устраняет причину,
не расширяет heap в тесте и не меняет алгоритм.

Root повторил direct NOT/layout и все native showcase models: MIR2 0.622 s,
pipeline 1.373 s, Nanz 0.049 s; exit 0. Новые нагрузки: все 256 popcount,
36 FSM state/event/property calls, 250 Lizp crypto/checksum, Pascal sieve
101 флаг и count=45 по независимому trial division. 18 live-A regressions
потребляют значение после NOT. Catalog193 теперь связывает 11 ограниченных
workloads с независимыми оракулами; остальные не объявляются исполненными.

Свежий compile/assemble gate на ee21cae8: 192 PASS, один SKIP, exit 0.
Отдельный MINZ_SHOWCASE_CHECK_KNOWN=1 для ZVM: exit 1, явный отказ по
story[131072] после 18377 bytes предыдущих globals. Исправленная u32 поддержка
не устраняет физический лимит адресного пространства. Устаревшая skip reason
про u32 spill заменена на актуальную причину host memory model.

[Пакет](../docs/grace-showcase-algorithm-oracles.md),
[root repeat](../docs/grace-showcase-algorithms-integrated.txt),
[полный compile gate](../docs/grace-showcase-ee21cae8-all.jsonl),
[ZVM refusal](../docs/grace-showcase-zvm-capacity-current.txt).
Пакет и логи скопированы на /mnt/safe/minz/grace/2026-10-11/;
13 файлов исходного integration packet сверены SHA256.
Общий pipeline handle2903 ещё жив; terminal результата пока нет.
Следом сверка каталога со всеми существующими native tests, реальная польза
таблиц на корпусе и effect-safe invariant address hoist. Цель активна.
