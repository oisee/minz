# Оригинальный rotate sled: native proof без source rewrite

База432a3950; exact examples/nanz/16_rotate_sled.nanz и legacy-import copy сохранены с независимыми SHA в freeze.json. Пять source helpers проверены на всех256байтах в ordinary/GraceSize/GraceSpeed. Модели независимы от ASM: byte rol1/rol3/rol4, high=x>>4, low=x&15. Все3840helperCALL прошли с returnPC7003/SPf000; main3раза выдал точный stdout1861011 черезOUT23 (проверены оба байта portaddress). ПолнаяRAM/profilerwrites разрешают только observedstack и testCALLdriver, IX/IY не изменены. Main заканчивается compiler-generated XOR A/DI/HALT, SPeffe: первоначальный harness ожидал CALL-return и был исправлен; это не дефект исходника/компилятора.

Никакого source исправления не делали: фактические input/return homes helpers — A, поэтому ASM-модификация A оказывается правильным возвращённым значением. Это положительный результат для исходного context, не универсальное доказательство стиля `asm(clob all); return x` при другой ABI/allocator. Parser поддерживает явный (ret REG)/(out REG) в pkg/nanz/parse.go:4112; изменение source только ради гипотезы не требуется. VM OpAsm=no-op не используется как rotate oracle.

Все режимы: module192B, main1142T доHALT, maxstack6B. Helper total включает testCALL: swap70T,rol1 31T,rol3 66T,high77T,low34T. MIR ASM peephole заменяет RLCA3/4 на CALL shared sled независимо от OptSize/Speed; overhead27T относительно inline. Это существующая политика, не заявленный выигрыш скорости. Реальный runtime7RLCA+RET —8B, старый source comment9B неточен. Его не меняли.

Focused nativePASS0.031s; широких gate повторов нет. Exact source/ASM/BIN/MIR/results и retained executable в manifest; coverage только declared sixentries, а не все ASM/library allocationcontexts или физическое устройство.
