# PFCCO arena: исправление интегрировано и повторено

Root `4e70ff9e`: arena consumer, прежний prospective PFCCO heldout и local-array pointer VM/Z80 regression совместно **PASS 1,565 с**. Полный неизменённый arena BIN1130→1122B; сумма11caller13040→12900T; arena_split100boundarypairs220→166T. Строгий size-gain assertion сохранён, caller moves/saves входят в native замер.

Изменение PFCCO разрешает softword preferences как реальные альтернативы вместо ложных hardconflicts. Одноразовые word literals размещаются под выбранный класс доallocation; edgecost и transformation связаны в pipelinehelper. FixedABI/ASM/extern/hard/multiuse отказ сохранён. Greedy не менялся. [Аудит и сохранённые первоначальные отказы](../../reports/pfcco-arena-current/RESULT.md).

Исторический report069 объяснял выигрыш успешнымsolver, но там фактически сработал fallback. Положительные старые native числа реальны, объяснение было неверным. Новый результат получен после успешного выбора, с typedcallliterals, а не восстановлениемстарогоfallback.

Прежний PFCCO heldout Speed5568T/52B, Size6252T/52B сохранён. Новый полный rootMIR запущен, donorfullMIRPASS79,744с относится к отдельномуcheckpoint. NC ещё имеет unresolvednativeoutput mismatch и не получает corpusadmission.
