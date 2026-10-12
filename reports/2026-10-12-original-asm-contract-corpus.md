# Исходный ASM корпус: native ABI и effects

База `7ce75464`. Шесть исходных путей сохранены без изменений, SHA и prospective domains — [freeze.json](original-asm-contract-corpus/freeze.json). Источник ABI: Nanz Language Book v7 §2.14/2.15 (`ret`, `clob`, input registers) и точные Z80 инструкции. **MIR VM OpAsm=no-op не используется как доказательство.** Production-код не менялся; новые helpers не подменяют исходные functions/caller chains.

| Source | Callable entries | Исходных asserts | CALL / режим | Полный BIN | T / режим |
|---|---:|---:|---:|---:|---:|
| examples/tests/asm_caller_callee.nanz | 17 | 9 | 724 694 | 155 B | 24 367 726 |
| examples/tests/asm_calling_convention.nanz | 16 | 44 | 800 496 | 68 B | 27 882 696 |
| examples/tests/asm_register_permutations.nanz | 28 | 35 | 4 207 650 | 107 B | 192 884 679 |
| March11 ex17_asm_block.nanz | 3 | 0 | 258 | 61 B | 11 775 |
| March13 ex17_asm_block.nanz | 3 | 0 | 258 | 61 B | 11 775 |
| March15 ex31_asm_ret_clob.nanz | 3 | 0 | 768 | 13 B | 39 680 |

Ordinary, Grace Size и Grace Speed совпали по значениям/BIN/T. **17 202 372 actual CALL** и 88 source assertions в каждом режиме; [native.txt](original-asm-contract-corpus/native.txt) PASS. Все исходные entries выполнены, wrapper CALL/JP chains настоящие. Return PC=0x7003 и SP=0xf000 проверены вместе; максимум stack6B. Costs включают CALL/return/moves. Эти цифры — scoped verification, не benchmark пользы Grace.

## Независимые модели и домены

Byte unary все256; byte binary все65536; triple всеa/b при c=0/1/127/128/254/255; four registers all a/b при c изнабора и d=c либо255-c (12 pairs). Word unary все65536; word binary14×14 boundaries; mixed word14boundary×all256byte. Pinned A/B/C/D/E/H/L, HL/DE, asymmetric SUB/XOR/weighted sums, register rotation и вызывающие wrappers проверены через actualallocated param homes и actualreturn contract. screen_addr: весь физический экран32col×24row×8line, независимая битовая модель. Full3/4dimensional Cartesian не заявляется.

Memory: direct peek/poke/store_at на0/1/4000/5000/fffe/ffff ×allbyte values; poke_peek originalwrapper FE00. Сравниваются code/64KiBRAM кроме exact explicit testmemory и наблюдённой stack области; profiler отвергает другие записи. Для copy_n1..255 два disjoint256B buffers, проверяется каждый destinationbyte включая untouchedsuffix. Caller-clobbered main registers не объявляются неизменными: A/wordreturn проверяется семантически, A/F/BC/DE/HL outputs хешируются в фиксированном порядке. IX/IY проверяются как неизменные, когда actual code их не использует; comments исключены. Shadow-bank/interrupt arbitrary effects этим byte/word suite не доказаны.

Ports: set_border all256 inputs — точный port=(A<<8)|FE; ex17 main — border2, 4NOP, skipped6502 block и точная строка `asm blocks work!\n` черезOUT23. ex31 port_read all256highbytes с frozen seed, проверяется **16-bit** address highA:FE; main DF:FE→double дляall256seeds; doubleall256. Это injected peripheral oracle, не реальный Spectrum keyboard. Нет предположения о VM ports или CP/M runtime.

## Обнаруженная ошибка исходного описания, не lowering

В неизменённом `asm_register_permutations.nanz:149` `copy_n(dest@HL,src@DE,count@B)` выполняет `LD C,B;LD B,0;LDIR`. Z80 LDIR читает **HL** и пишет **DE**. То есть имена/comment API обещают противоположное направление. Первоначальная comment-derived модель провалилась уже count1 во всех режимах; [before-copy-comment-oracle.txt](original-asm-contract-corpus/before-copy-comment-oracle.txt) и exactbeforetest сохранены. Затем отдельно зафиксирована [коррекция модели](original-asm-contract-corpus/copy-model-correction.json): unchanged source копирует HL→DE и возвращаетadvancedHL, что native подтвердил. Это **не compiler miscompile**; corrected-source вариант не создавался, ошибочное описание не спрятано.

count0 означает BC0 и LDIR выполняет65536 transfers, а не zero-trip. Этот case не запускался в positiveproof, поскольку затрагивает code/stack; freeze ранее называл его отдельным probe, но evidence его не содержит. Не заявляем count0safe или весь memory-domainPASS.

## Сохранение и воспроизведение

Все шесть original source snapshots сохранены; ex17 duplicates сопоставляются по SHA, не по имени. Source assertions отдельно воспроизводятся после независимых sweeps. ASM/BIN/finalMIR/options/counts/regs/allowedmemory, immutablebeforefailure, manifest и compressed exactpipeline executable retained. SHA executable — executable-identity.json. Распаковать pipeline.test.xz, запускать из **minzc/pkg/pipeline**: `ASM_CORPUS_EXPORT=/new/path ../../../reports/original-asm-contract-corpus/pipeline.test -test.run '^TestOriginalASMContractsNative$' -test.count=1 -test.v`.

Категория native ASM закрыта только для этих exactsources/доменов. Не утверждаем VM/nativeэквивалентность ASM, безопасность произвольного clobbercontract, fullperipheral/runtime поддержку или завершение wholecompilergoal. Полныеpipeline/CLI gates не повторялись.
