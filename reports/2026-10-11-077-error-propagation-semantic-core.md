# CY error ABI: explicit MIR outcomes and original source proof

Исправление 692dbcbf сохраняет код и статус ошибки как семантику языка. Это не перенос host hooks. У HIR/MIR функции есть FallibleCY, у вызова отдельные SSA результаты failed:bool/code:u8, у @error — настоящий TermErrorRet(Code), у @propagate/@check — ветвь на эти результаты с возвратом ошибки. Успешный выход очищает CY после return copies/frame cleanup; ошибка возвращает код в A и CY=1.

Нативный вызов снимает AF до result pickup и caller restores, материализует carry в bool и сохраняет A-код. Исправлено пропущенное interference между одновременными ExtraRets. Типы, dump, JSON-идентичность, verifier, liveness и remappers учитывают результаты/терминатор. Значение ошибки вычисляется один раз, код не исчезает как мёртвый.

VM CallWithOutcome возвращает Values/Failed/ErrorCode; Go error остаётся trap/budget/unknown symbol. Обычный Call оставлен совместимым: ошибка даёт byte code среди значений, поэтому отличать её должен CallWithOutcome. Существующие value-only controlled/memory-host профили явно отклоняют CY программы до запуска — иначе выбор мог принять ошибку за успешный результат.

Fallible ABI выводится из ? имени, @error/@check/@propagate и транзитивных прямых вызовов. Поддерживаются defined u8/void и немедленная проверка standalone direct call. Word/tuple/extern, вложенные fallible expressions и eZ80 явно отклоняются. Status-unaware constant-call folding, LUT, trivial inline, tail call/recursion, conditional-return sink, conditional-call folding и pressure split не применяются к fallible ABI; остальные правила Grace продолжают работать. Это первый корректный поддерживаемый slice, не универсальная обработка ошибок.

Замороженный 00553e9d исходник examples/nanz/14_error_propagation.nanz не изменён и не урезан. Oracle Book v7 Appendix K CY+A и исходные branches: safe_div/add/sub/compute/chain все 256×256 bytepairs, array_get все256 indices. 327936 вызовов/режим, 983808 total. Каждый вызов проверяет raw/final VM value+status/code, реальный assembled Z80 A/CY, исходный PC возврата+SP и полный RAM/code кроме ограниченного использованного stack и точных spill bytes. Нет источниковых asserts (комментарии исходника сохранены).

Весь original main проверен raw/final VM с явным print-u8 host только для десятичного вывода; native использует штатный порт0x23 и отказывает другим портам. При входном CY=0 и CY=1 stdout равен 5250, исходная @check останавливает последний print, A=1/CY=1, SP/RAM/code сохранены. Это emulator application image ORG8000 с отдельно отмеченными CALL/RET drivers, не linked COM и не реальная CP/M OS.

Terminal original fullsource3modes PASS49.812s. Отдельно512 native+VM проверок двух локальных frames, живого A и callerAF/BC restores; full MIR2/HIR/Nanz PASS16.580/25.529/14.827s. Negative verifier/status-remapping, suffix?, unsupported compiler APIs/value-only profiles PASS. Исторические отрицательные ASM/BIN и лог в baseline/ остаются без изменений.

Повтор: cd minzc; MINZ_CARRY_ERROR_ARTIFACTS=<dir> GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^Test(OriginalCarryErrorNative|ErrorABIRejects|FallibleSuffix|ValueOnlyProfiles)' -v; go test ./pkg/mir2 ./pkg/hir ./pkg/nanz. Итоговые ASM/BIN и raw/final MIR в after/. Все файлы манифеста tracked, SHA проверены. Не утверждаем all-corpus correctness, оптимальность или поддержку CY профилирования.
