# Original Nanz advantage showcase

Exact `examples/tests/nanz_advantage_showcase.nanz`, все13 declared functions, global vals и9 исходных via-mir2 assertions сохранены. Source/domain/host-model зафиксированы до исполнения в `ba591dbf`, base `6d583fc7`. Semantic model: Bookv7 control-flow/8-bit width/valuepipe/fold initial/enum declarations. Результаты не извлекаются из IR.

Численный domain: double/inc/square все256; add_acc и board_idx все65536 пар; pipeline_demo49, fib8=21/fib12=144; fib все0..254 с независимой simultaneous Fibonacci recurrence modulo256; opposite все4 declared enum tags0↔1,2↔3. Invalid tags не названы допустимыми enum inputs. Fold sum_all/sum_plus_offset: original vals[10,20,30,40,50] плюс1280single-lane domains и7776boundary quintuple snapshots =9057 arrays. Каждая из двух функций обязана дать independentbyte sum илиsum+5, читать ровно5bytes в исходном порядке и не менять array.

Всего450651численных entryexecutions втрёхрежимах (изних3main directentry,450648CALL); module156B.

Во всех ordinary/GraceSize/GraceSpeed режимах каждый численный вызов выполняется raw/final VM и actualnative. Native использует реальные ABI homes, проверяет callerPC/SP, physicalPUSH/CALL wordbytes, wholeRAM/codeguards. Mutable globalvals отдельноинициализируется и проверяется каждым fold. Source voidmain выполняется directentry XORA/DI донастоящегоHALT соSPunchanged, не черезCALL/RET.

**fib(255) отдельно является намеренно не возвращающимся исходным случаем.** В source u8i начинается2, whilei<=n, i++wraps; приn255 любоеbytei<=255. Нельзя приписать ему numerical Fibonacci return. Raw/finalVM достигаютgaslimit10000. Native после770loop-head visits повторяет полное exposedregisterstate на том жеPC иSPeffe без dataeffects; RAM/code сохраняется, loop больше не может выйти в данном deterministic context. Explicitnonreturn JSON сохранён в каждомmode, а не зелёная numerical строка. Это не unsupported compiler и не fail oracle.

Пакет полностью исполняет объявленные13entries, отдельные9sourceasserts сохраняютqualifiers (мы дополнительно тестируем native даже там, где source onlymir2). Native pipe calls constantfolded; fold реальныйDJNZloop; fib8/12constantfolded. Исправленийproduction не потребовалось. BINparity всехтрёхрежимов — correctnessproof, не новыйGrace/heldoutgain.

Финальный frozenexecutablePASS1,64s. Сохранены источники/freeze, raw/finalMIR/ABI/allocation/ASM/BIN/results/nonreturnproof/compressedexecutable, SHAmanifest. Шара `/mnt/safe/minz/grace/2026-10-11/advantage-native/REPORT.md`. Полныйpipelineне запускался.
