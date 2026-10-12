# Full HIR/MIR и новый native NC контрпример

Root `0597fc9b`: полный HIR **PASS 25,602 с**, полный MIR2 **PASS 78,620 с**, включая pointer-width и print intrinsic fixes. Это package gates; они не заменяют whole-main поведение корпуса.

NC raw/final VM уже проходит независимый1453-event сценарий. Native тот же unchanged source/image/key workload **FAIL**: первый mismatch event188 (неверное выделение строки), далее зацикливание и невосстановленный возврат. `ordinary`, Grace Size и Grace Speed отказывают; statusNC не повышается. Source/disk/events frozen доexecution. Driver CALL main, actual extern formal homes, disk host пишет только точный512-byte win, unknownwrites/callsrefused. Native тест и raw diagnostic ASM/BIN/MIR/alloc/events сохранены; расследование продолжается.

Первоначальный native probe не находил sanitizer-имя local global `format_size$tmp`; observer corrected to emitted identifier, historical log retained. Следующий FAIL — actual execution mismatch, не mask test. Текущая native проверка guard исключает mutable globals как область, поэтому полная независимая finalRAM proof ещё не заявляется. Браузер не считается native-correct.

PFCCO arena кандидат отдельно прошёл actual222nativeinvocations с независимыми assertions:1130→1122B, split220→166T; root интеграция и полный package gate пока впереди. Исторический report ошибочно приписывал прежний выигрыш успешному solver: там работал fallback. Новый кандидат должен проверяться как настоящее решение и сохранять typed literals.
