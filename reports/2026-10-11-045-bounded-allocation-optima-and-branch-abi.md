# Ограниченные доказательства allocation и branch ABI

Интегрированы `8effe44b` и `ebc7ea5e`. Exact source is_digit_b и caller,
сохраняющий исходный аргумент, прошли VM и Z80 для всех256 входов. Пять блоков,
оба compare и пути возврата отражены в MIR/ASM. CFG table admission пока
не расширяется: нужны отдельные доказательства per-edge inputs/copies/flags.

PFCCO сравнен с независимым полным перебором на600 детерминированных forests,
до6 nodes с3 вариантами. Матрица adapter cost измерена на реально собранном
Z80 и включает CALL/RET и сохранение BC. Negative control без21 T save cost
меняет выбор на реально более дорогой вариант. Это bounded model proof,
не доказательство качества любого production call graph.

Для PBQP задачи (x+3)+x перебраны81 назначения A/C/E;36 проходят независимые
source-liveness constraints, и все36×256 inputs проверены native. Текущий
PBQP достигает model optimum12 и actual native minimum57 T/5 B; худший
вариант80 T/11 B. Пересылки/сохранения исполняются фактически. Domain малый;
spills/CFG/flags/широкие типы здесь не исчерпываются.

Root повторил все три проверки: PASS. Пакет и точный source/ASM/BIN/MIR
скопированы на шару и сверены SHA256.
[Отчёт](../docs/grace-branch-allocation-proof/README.md),
[root repeat](../docs/grace-branch-allocation-proof-integrated.txt).
Полный pipeline cb587fe4 ещё работает. Общая цель остаётся активной.
