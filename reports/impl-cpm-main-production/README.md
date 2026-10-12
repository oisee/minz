# Original impl demo: production CP/M COM

Исходный `examples/nanz/51_impl_cpm_demo.nanz` и BDOS ASM неизменны. Workload frozen c83f75ec; исторический отказ и тестовый HALT-loader сохранены отдельно в ../impl-cpm-main. Production donor 6635e4c6.

Новый допуск: только полный однобайтовый void ASM-адаптер `LD E,A; LD C,2; CALL 5` с входом в A и AllClasses clobber. Точная машинная позиция адаптера проверяется повторно после генерации; любой иной ASM остается запрещён. Локальные Alloca допускаются только в entry, положительный фиксированный размер ≤256 суммарно; адреса выводятся через Move/Field/константный PtrAdd и проверяются по размеру объекта. Сохранение локального адреса/возврат/неизвестный вызов запрещены; разрешён только закрытый read-only leaf pointer consumer с проверенными смещениями. Обычные extern peek/poke прежнего runtime сохраняют свой ABI.

Стековый verifier отслеживает каждый DEC/INC SP и все CFG пути, consumes guard reserve для роста; эмиттер ставит проверки и внутри доверенного ASM перед CALL. Частные 1024 байта стека linked внутри COM. Сохранение всех основных регистров/флагов остаётся обязанностью обычного call-clobber контракта; BDOS адаптер не переписан. Внешние внутренние потребности BDOS по стеку — условие среды ОС.

Три режима ordinary/GraceSize/GraceSpeed, два независимых BDOS2 host режима (preserving и A/F/BC/DE/HL/IX/IY clobber) прошли исходный whole main: `C:75\r\nR:50\r\n`, ORG0100, OS RET PC0000/SPfe00, исходное входное слово стека сохранено. Полные 64KiB/code сохраняются вне точных named runtime/spill ranges; guard A5 и status0 проверены. Каждый PUSH/CALL записывает ожидаемое слово. Actual stack highwater19B (включая guard helper). COM1419B, SHA199d968289be627478633709f5130b9113cea6f7a1244d4062e16746b8b1b46d одинаков во всех режимах; 7442 CPU T без стоимости синтетического BDOS. Выигрыш Grace здесь не заявляется.

10 near-miss admission mutation controls, mutated named machine adapter/stack/unguarded growth controls PASS. Дополнительный явно test-owned caller проверяет живой байт и comparison predicate через all-clobber BDOS на256 входах ×2режима (512 native). OriginalSelfLanz + tokenizer unguarded/guarded focused suites PASS9.364s; CPM MIR checker suites PASS.036s; final focused PASS.171s. Предыдущая независимая print_u8 proof всех256 сохранена в baseline packet. VM ASMstdout не подменён host stub. Это эмуляторное evidence runtime ABI, а не физическая CP/M машина.

Public command из корня репозитория:
```
/mnt/safe/minz/grace/2026-10-12/impl-cpm-main-production/executables/minzc examples/nanz/51_impl_cpm_demo.nanz -t cpm --runtime cpm --runtime-stack 1024 -o demo.com
```
Size добавить `--grace --hir-grace --Osize`; speed `--grace --hir-grace`. Все три CLI COM побайтно равны emulator-tested production/program.com. ASM/MIR/options/allocation/symbols/stdout/BDOS ledgers сохранены. Frozen executables SHA и точные пути в frozen-executables.json; frozen pipeline test запускается из minzc/pkg/pipeline с `-test.run '^(TestOriginalImplCPMProductionMain|TestOriginalImplCPMRejectsUntrustedVariants|TestCPMSourceBDOSCallerLiveState)$' -test.v -test.count=1`.

Допуск не является произвольной поддержкой ASM/динамической памяти: mutable/dynamic pointer escapes, loop allocations, unsupported extern ABI и SMC отказаны. Полная цель корпуса не объявляется завершённой этим приложением.
