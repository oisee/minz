# Интеграция 082: full core, four pointers, PFCCO arena, Sites 63

Полный core на 9b1637ef terminal PASS: HIR23,824s, MIR2 9,536s, Nanz13,332s, VIR2,097s, CLI395,601s. Это включает reserve coalescing и предшествует расширению native harness IX/IY. Полный pipeline22e426de отдельно PASS2805,380s.

Exact05 four_ptrs интегрирован dcaf6e30:1792calls,7aliaslayouts×256patterns, raw/finalVM+assemblednative с настоящим HL/DE/BC/IX ABI. Ordered reads,259RAM,guards,SP/code PASS. Изменён только invocation harness для IX/IY;12boundarytests проверяют сохранение остальных registers/flags/PC/SP и unknown отказ. Actual complete workload182801T/22B; это не gain от оптимизации.

Обычный opt-in PFCCO PBQP на exact08 arena allocator:1193→1184B;test_split3896→3808T,11actualsource/sandboxcalls13100→13012T. Direct arena_split100boundarypairs на каждой стратегии220→190T,4writtenbytes/raw-widthoracle/actualpointer+wordABI/SP PASS. Root добавил guards ptr-1/ptr+4 и immutable compiled image проверку с явным исключением mutable module globals. Этот focused test PASS0,139s. Production/admission не менялись; стоимость включает actualassembledcalls/moves/saves. Это отдельное PFCCO evidence, не новый Grace prospective search.

Каталог194:35 ограниченно проверенных independent VM/native paths,1nativeCPM,101pending,32ASM,19externalruntime,5Agon,1host. Новое PFCCO evidence на08 не увеличивает счетчик: он уже имел алгоритмический оракул.

Sites63 опубликован с owner-private audience; точный deployment/source/archive в site-v63.json.74HTML pages без missing local links,18новых архивов SHAverified.9733currentdist files246247235B SHAverified copied to dated site-and-artifacts, исторические extra files сохранены. Полный archive203311064B также скопирован и SHAverified. Сайт snapshot e6cb41ef предшествует terminalcore и последним oracle/PFCCO packets; эти результаты доступны в repo/share, не объявлены опубликованными на сайте.
