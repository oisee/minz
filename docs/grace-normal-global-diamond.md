# Корректный load forwarding оказался медленнее

Изолированный normal-global diamond helper заменил ровно одну загрузку pos
в cur на Move доминирующего значения. На настоящем исходнике статический
MIR diff подтверждён, rawVM unsignedbounds/barriers/refusals прошли.
Два full30 actual Z80 прогона сохранили память и порядок событий, но speed
вариант оказался хуже. Helper и диагностический hook остаются только в
экспериментальной ветке; production compiler не меняется.

| Whole Self-Lanz30 | T-states | Байты | Native steps |
| --- | ---: | ---: | ---: |
| Current production original | 1135370 | 2000 | 109829 |
| Private reuse prototype | 1137339 | 1998 | 111323 |

Итог: **+1969 T (+0.173%), −2 B, +1494 шагов**. Это отрицательный speed
результат, не основание включать правило по умолчанию. Размер уменьшился,
но нового production size-selector для этого эксперимента не добавляли.

Причина видна в cur ASM/allocation: первое pos остаётся живым в DE через
сравнение; добавляется PUSH DE/POP DE (21 T) перед ветвлением. Удалённая
повторная загрузка стоила16 T только на inbounds пути, а загрузка src_buf
сменилась с DE20 T на HL16 T. Новых per-PC counters здесь нет; algebra
дифференциальных расходов не выдаётся за новый dynamic PC профиль.

Исходный raw oracle считается до experimental MIR rewrite. Native roots,
allocation и assembler получены заново из candidate MIR. Original прямо
сравнивается с текущим production dba34 BIN/ASM/allnativehash/percallTSteps.
Оба candidate raw/source compatibility/functionmaps остаются исходными,
полные target results/globals/resources/3017 ordered events совпадают.
Два независимых saved-file аудита проверили60observations, exactMIRsingle
Load→Move, native SHA bindings и отдельный experimental executable context.
Это диагностический эксперимент, не production optimisation certificate.

Helper допускает только обычный exactu16global без AT/SMC/vtable/string/
malformedstorage, verified acyclic entry→singlepred successor и canonical
pure interval между чтениями. Calls/stores/asm/unknown effects внутри
интервала отклоняются; поздний call после чтения не снимает допустимость.
Исходные CSE/InlineTrivial guards не ослаблялись, defaultpipeline не менялся.

[Пакет](grace-normal-global-diamond-artifacts/native-experiment/README.md) ·
[Независимый аудит](grace-normal-global-diamond-artifacts/independent-oracle/README.md) ·
[Native review](grace-normal-global-diamond-artifacts/native-review/actual-audit/FINAL_REVIEW.md) ·
[SHA256 manifest](grace-normal-global-diamond-artifacts/manifest.json).

Все source snapshots, raw/pre/post MIR, actualASM/BIN, allocation, contexts,
логи и negative comparison сохранены локально. После переноса gzip helper
обновлён для поиска `.gz`, полный saved audit прошёл без CPU.

Следующий шаг — readonly design сравнения u16 с сохранением живых operands,
AF/flags и branch semantics. Новых native результатов или production fix
для этого сравнения пока нет.
