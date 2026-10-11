# PFCCO × checked table × Grace: 12 сочетаний

На исходном 06_pbqp_weighted.nanz с явным weighted_caller(x)=weighted(x)+x измерены off/greedy/PBQP × HIR+MIR Grace off/on × table off/on. Все 12 вариантов прошли raw/final VM и assembled native для всех256byteinputs обеихфункций (6144nativecalls), включая actual ABI,SP и неизменность compiledimage. Точное table consumption проверено по trace. Generation uses explicit Z3; table generated separately under each compile mode, без обхода signature/configuration admission.

| Table | Full BIN | weighted call | weighted_caller call |
|---|---:|---:|---:|
| off | 44 B | 182 T | 238 T |
| on | 20 B | 86 T | 142 T |

Эффект одинаков во всех шести PFCCO/Grace сочетаниях. Нового дополнительного выигрыша от их совместного включения здесь нет. Стоимость caller включает реальные argument moves, CALL, preservation и bootstrap. Exhaustive validation — не новый prospective PGO выбор. Grace переключён одновременно на HIR и MIR; отдельное разложение этих двух флагов не измерено.

Все actualcandidate ASM/BIN,таблицы,исходныйsource иcalleradapter,512case timingaggregate иdiagnostics сохранены в artifacts. Сохранённый BIN не подаётся в существующий ProfileCompilationID: внешний tablepath по-прежнему запрещён профильным API до отдельного bindingdomain. Productiondefaults не менялись, таблицы не стали hint PBQP.

Astra была запущена для ревью комбинаций, но turn завершился token rate limit eastus без заключения. Вывод основан на коде и independentnative expectations. Focused PASS5,070s; общий pipeline3e143f9f выполняется отдельно и не включает новыйtest.
