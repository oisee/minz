# Wide native, контракты и проверенные локальные таблицы

Интегрированы 6be9d4d4 checkedlocaltables, e66b02a7widecaller/memory,cb587fe4nativePFCCO workload. Полные пакеты объединённой ветки: MIR2 PASS 8.423s, VIR PASS 0.058s, z80spec PASS 0.006s; u32production, APIoption иHashMapheldout PASS3.224s. Supportedcmd/pkg/internal/tests compile-onlyPASS.

Широкая память:672 VM/native случаев с u24/u32, pointer/valuehomes, shadowbanks иwrapffff;32 CALLcases сдвумяclobberedbanks иmodulo2^32oracle. Реальный Nanz productionglobal+call+add/store проходит. Snapshotcertificates memory-host ещё ограничены1/2байтами; не объявлены широкой сертификацией.

PFCCO workload: два реальныхCALL, первый результат жив через второй, независимая order-sensitive модель иglobalwrap. Без контрактной оптимизации8857T/52B;greedyиPBQP7888T/52B. Сохранён также первый no-benefit кандидат6474T/68B. Независимый Pythonаудит повторноPASS.

VIRcheckedlocalkeys связывают descriptor, operands, hints, clobbers, ABI,physicaloverlaps иcostmodel; assignment повторно допускается,claimedcost пересчитывается. Исправлена потеря wordreturn передRET. Fixture95→57T,10→7B срезультатомHL1234/SPf000. **Обычный production pipeline пока не вызывает VIR/table lookup**; практическая интеграция отдельным локальным кандидатом впереди. Это не globaloptimality и не wholecorpusbenefit.

Новый showcase имеет193case (добавленpfccoweightedpacket):192PASS,1FAIL,0SKIP. ZVM теперь отвергается ясной nativecapacity причиной:story131072B плюс18377B предыдущихglobals. Исходник сам заявляет запускmzv-H; nativeработаZork не объявляется. Полныйpipeline cb587fe4 запущен отдельно; пока идёт, результат неизвестен.

Все77новых файлов скопированы на шару и провереныSHA256; пакетыdocs/grace-vir-checked-local-tables, grace-z80-u32-spills, grace-pfcco-native.
