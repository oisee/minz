# ABI таблиц v4 и публикация сводки

Интегрирован `c185aebb`. Проверяются exact typed entry/contract mapping,
arity, physical storage kind, return width/class и изменение функции после
подготовки. Два native контрпримера сохранены: LocMem с именем A читал
accumulator вместо memory ABI; отсутствующий tied operand читал старый A.
Оба теперь отвергаются. Контекст v4 сохраняет старые записи как misses.
Root repeat VIR и pipeline PASS; benefit weighted с caller сохраняется.
Calls/CFG/islands в допуск не добавлены.

Опубликована приватная сводка:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/current-state.html
Site source commit `7aca483b1939348d48f01f1bb5d888b7651ee9e1`, deployment
`appgdep_6acbba0ae2408191953ed789c6156aac`, terminal succeeded.
Публикация содержит compiler checkpoint c3782f21; более поздний v4 отражён
в repo/shared current state, не объявляется частью опубликованного checkpoint.

Прежняя сводка сохранена в current-state-legacy-v60.html. Девять tar.xz
пакетов включают source/workload, ASM/BIN, наблюдения, оракулы и внутренние
SHA256. Новые страницы/ссылки и архивы проверены локально. Browser QA не выполнялся;
успешный native Sites result подтверждает публикацию.

[ABI report](../docs/grace-local-table-abi-binding/README.md),
[root repeat](../docs/grace-table-v4-integrated.txt).
Обновлённые страницы, пакеты и полный deployment archive сохраняются на
/mnt/safe/minz/grace/2026-10-11/ с отдельным integration-042 manifest.
Полный pipeline cb587fe4 ещё работает. Цель остаётся активной.
