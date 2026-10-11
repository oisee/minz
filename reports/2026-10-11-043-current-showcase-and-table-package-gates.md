# Свежий compile gate и полные проверки таблиц

На `d11aa0d2` выполнен showcase compile/assemble с
MINZ_SHOWCASE_CHECK_KNOWN=1: 193 точных случая, 192 PASS и один FAIL.
Единственный отказ — host ZVM: story[131072] превышает native 16-bit адресное
пространство. Ни один случай не пропущен. Это проверка сборки, не доказательство
исполнения всех приложений; независимые workload оракулы отражены отдельно.

Полный VIR PASS 2.102 s, Z80spec PASS 0.008 s. Таблицы v4 допускаются с exact
ABI/type/storage/arity checks; старые production контексты дают fallback.

[Showcase JSONL](../docs/grace-showcase-d11aa0d2-all.jsonl),
[полные package gates](../docs/grace-table-v4-full-packages.txt).
Оба лога сохранены на шаре и сверены SHA256. Full pipeline cb587fe4 handle2903
ещё жив; его terminal результат не заявляется.

Следующая работа: CLI measured modes, whole-main self-Lanz и independent
exhaustive сравнения allocator. Общая цель остаётся активной.
