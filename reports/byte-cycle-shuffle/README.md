# Сохранённая пара вместо shadow-A snapshot в byte cycle

Production base **1f2365be**. Freeze7d0b2fdd опубликован до правок, emitter fix **a445e292**, независимый замер старого/нового ASM **3cfaec07**. Меняется только общий `emitParallelCopy`, не PFCCO, interference, source или runtime permissions.

Когда закрытый byte-cycle использует только основные A/B/C/D/E/H/L и A занят перестановкой, emitter ищет BC/DE/HL, полностью отсутствующую среди **всех** источников/назначений copy-list. Учитываются identities, завершённые копии, пары/половинки и immediate destinations. Найденная пара сохраняется PUSH/POP: её внешний живой владелец не теряется. Старые значения цикла перемещаются в обратном порядке после сохранения одного байта в scratch. LD и PUSH/POP пары не меняют F, AF' не используется. При отсутствии полностью свободной пары или при index/spill цикле сохраняется прежний snapshot fallback. Это не предположение, что невидимые в copy-list регистры мертвы.

Реальный A↔C теперь: `PUSH DE; LD D,A; LD A,C; LD C,D; POP DE`. Авторитетный emulator измерил **33 T / 5 B** против точного сохранённого прежнего snapshot **95 T / 14 B**. Раннее предложение113 T было предварительной оценкой; здесь используются фактические95 T. Stack growth —2 байта, вместо6 байт прежних одновременно сохранённых AF; runtime instrumentation учитывает обычный emitted PUSH.

| Исходный tokenizer, PBQP | До | После |
|---|---:|---:|
| ordinary BIN | 5683 B | **5593 B** |
| адаптированный исполняемый BIN | 5687 B | **5597 B** |
| init_keywords | 17488 T | **16868 T** |
| последовательность двух контрольных входов | 327906 T | **327286 T** |

Десять A/C cycles в init дают ровно10×62 T и10×9 B. Tokenize138371/131639 T и render21896/18512 T не изменились. Greedy ASM/BIN и все его замеры побайтно/численно прежние:5465 B, init15790 T, sequence326208 T. **PBQP всё ещё хуже greedy по обоим критериям**. Этот patch улучшает общий emitter и конкретный измеренный PBQP код, не доказывает новый полезный автоматический выбор PBQP.

Повторяет прежние два фиксированных tokenizer входа из TestPFCCOOriginalTokenizerRenderingConsumer: lexer/interner oracle, весь string arena8192/token4096/source4096 buffers, globals, stdout, code/literal guards и SP. Source неизменён. Ordinary code содержит extern peek/poke; одинаковые явные native adapters добавляют4 байта, ORG0400. Эти входы были ранее отложены от исторического cost scan, но уже измерялись: **не новые prospective heldout для этого patch**. Более свежий существующий prospective PFCCO packet повторен отдельно: frozen training/heldout, выбор только training, same selectedBIN на heldout; Speed2320 training/5568 heldout T, Size2605 training/6252 heldout T,52 B. Patch не меняет этот packet и не превращает parity в дополнительный выигрыш.

Native тесты: шесть обменов A с каждым другим основным байтом ×65536 пар (393216 executions), каждый F byte покрыт; untouched primary/index регистры и SP. Дополнительно24 перестановки A/B/C/D и4 mixed/fallback cases ×256 входов, identities, live completed destinations, word cycles и immediates, независимый original snapshot oracle. Отдельный AF' sentinel6dd3 доказывает, что optimized path не заимствует shadow AF. Existing pair/half tests выполняют оба production entrypoints — block edge и call arguments — в обоих порядках, включая IX/IY, mixed word/byte и liveA. Existing dword bank tests тоже прошли. Standalone before/after measured check PASS.

Реальные calls/saves в tokenizer включены в BIN/T. Original guarded SelfLanz wholeCOM:135 stdout bytes,419878 T/3987 B; guarded tokenizer wholeCOM:146 stdout bytes,411987 T/7391 B. Оба независимых whole-memory/OS-return/SP/stack-guard оракула прошли; runtime guards не отключались. Stateful packet17 вызовов/режим сохраняет прежние8857→7888 T/52 B. Это emulator runtime contract, не физический CP/M hardware proof.

Scoped byte/pair/dword native PASS2.333s; whole runtime+tokenizer+packet PASS3.724s; существующий prospective packet PASS1.099s. Полный MIR2 пакет **PASS87.192s**, без нового fullpipeline/CLI gate. Отдельное добавление measured snapshot test после старта fullMIR проверено focusedPASS0.009s; production во время fullMIR не менялся.

Артефакты before/after: исходник, options, finalMIR, allocation, ordinary/native ASM/BIN, full state/stdout captures, source SHA и frozen test executable identity. Reproduce: из minzc/pkg/pipeline executable в build.json с `-test.v -test.run '^(TestPFCCOOriginalTokenizerRenderingConsumer|TestCPMRuntimeGuardedTokenizerMain|TestCPMRuntimeOriginalSelfLanzMain|TestPFCCOMeasuredNativePacket|TestPFCCOProspectiveHeldout)$'`; PFCCO_TOKENIZER_EXPORT и MINZ_PFCCO_HELDOUT_EVIDENCE — отдельные абсолютные output dirs. Audit `python3 reports/byte-cycle-shuffle/audit.py` не импортирует compiler. Share `/mnt/safe/minz/grace/2026-10-12/byte-cycle-shuffle/`.
