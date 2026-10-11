# Реальные таблицы и перенос invariant address

Интегрированы 74f4864a (offline CLI generation) и 0ad12214 (MIR Grace hoist).
Root повторил actual solver/native/admission и interning: pipeline 3.298 s,
MIR2 0.008 s, VIR admission PASS; все измерения воспроизведены.

Неизменённый 06_pbqp_weighted.nanz: corpus BIN 36→12 B. Отдельный явный
caller harness сохраняет x через CALL: module44→20 B, helper182→86 T,
caller238→142 T. Все256 byte inputs VM/native с actual ABI и balanced SP.
low_nibble имеет допустимую equal-size запись; baseline сохраняется.
Генерация использует existing global SMT только offline, затем независимый
AddChecked gate. Calls/CFG/effects domain не расширен; optimum не заявляется.

Общее pure address DAG правило на оригинальном grace_intern_checked.nanz:
default1637997 T/661 B, GraceSize1637997 T/661 B, GraceSpeed1611362 T/669 B.
30 independent persistent-state workloads PASS. Pure speed допускает +8 B.
Полная memory fusion не перенесена: partial-trap prefix и byte liveness требуют
дополнительной семантики/allocator support. Отрицательный call-loop замер сохранён.

Исправлен отдельный API gap5226309a: CompileHIRWithOptions раньше игнорировал
UseGrace. Теперь explicitGrace идёт через CompileHIRSteps. Реальный intern
тест обеих API даёт одинаковый BIN, правильный independent RAM/results/traffic,
и соблюдает Size policy; rootPASS0.384s.

Supported module compile-only cmd/pkg/tests PASS. go test ./... -run '^$'
имеет setupFAIL в историческом scripts: mixed template package и relative
imports в old debug snippets. Их нельзя объявлять passed; сохранены оба лога.
Общий fullpipeline cb587fe4 handle2903 ещё работает, terminal результата нет.

[Таблицы](../docs/grace-real-leaf-local-tables/README.md),
[hoist](../docs/grace-invariant-address/REPORT.md),
[root repeat](../docs/grace-real-table-hoist-integrated.txt),
[API](../docs/grace-grace-api-parity.txt).
Пакеты с SHA256 и frozen invariant executable сохранены на шаре.
Общая цель активна: расширение corpus coverage, whole-program selection и
полные gate/таблицы за пределами leaf domain остаются незавершёнными.
