# promoted_arithmetic: все исходниковые функции VM/Z80

База `561dab36`. Проверен **неизменённый целиком**
`examples/nanz/promoted_arithmetic.nanz`, SHA256
`cb0dea99eeda35be09f3fe09a015d2e827daeba98597322c4082d3d66d6b4d81`.
Все 9 callable entries и все 24 исходных assertions оставлены в parsed module;
исследовательский прогон не сводится к их подмножеству. Production compiler edits
в этом шаге отсутствуют.

Оракулы написаны независимо по исходным формулам и Book v7 §§2.9,3.1,3.2:
unsigned→i16 cast сохраняет 0..255; final u8 усекает; i16 промежуточные суммы и
разности заворачиваются; division truncates toward zero и remainder сохраняет
знак dividend. Для `pressure` каждый промежуточный x/y/z/w сначала приведён к
int16, затем host int32 division/remainder, затем final uint16 truncation.

| Исходниковая entry | Область независимой проверки на каждый режим |
|---|---|
| add, byte_add, add8, add_bytes | Каждая: все 65 536 byte pairs; результат uint8(a+b) |
| test_ld_word | Все 65 536 byte pairs; lo OR hi<<8 |
| swap_nibbles | Все 256 bytes; uint8(a<<4 OR a>>4) |
| popcount4 | Все 256 bytes; host bits.OnesCount8(a AND 15) |
| promoted_mod | Все 256 bytes; unsigned исходник расширяется в положительный i16, a%2 |
| pressure | 13^4 signed-word boundary Cartesian: 25 116 допустимых cases; 3 445 cases с нулевым y или w исключены явно |

Word values: 0,1,-1,2,-2,127,128,255,256,32767,-32768,-32767,0x1234.
Это не полный 2^64 word Cartesian и не проверка zero-divisor/error semantics.
Per mode **353 564** input invocations, каждый сравнивается с raw MIR VM,
final MIR VM и actual Z80. Три режима — **1 060 692** native executions.

| Режим | Полный BIN | Результат |
|---|---:|---|
| ordinary, ContractOpt=true | 342 B | PASS |
| UseHIRGrace=true, UseGrace=true, OptSize=true | 337 B | PASS |
| UseHIRGrace=true, UseGrace=true, OptSize=false | 337 B | PASS |

Z80 entry запускается настоящим opcode CALL из отдельного фиксированного
драйвера; драйверы не меняют compiled source/module. Параметры устанавливаются
по actual allocation homes, возвращаемое значение читается по actual contract.
Return PC должен совпадать с байтом после CALL; SP восстанавливается в 0xf000.
Измеренная максимальная глубина: 2 bytes у byte/word helpers, 10 у pressure.
Это глубина стека данного теста, не универсальная оценка stack demand.

Все 65 536 RAM bytes начинают с poison pattern. После всех calls сравнивается
вся память: разрешены только actual stack [0xeff6,0xf000) и ровно два байта
объявленного `_spill_pressure_r71` (адреса 0x8154/55 ordinary и 0x814f/50 Grace).
Source не имеет mutable globals. Код/литералы, CALL-драйверы и прочая RAM должны
оставаться неизменными. SP не разрешено опускаться ниже отдельной границы 0xefe0;
широкое окно памяти не может маскировать stack corruption.

[Retained executable gate](promoted-arithmetic-complete/gate.txt):
**PASS 3.69 s** (ordinary1.30, Size1.22, Speed1.17).
Go 1.26.8, GOCACHE=/tmp/minz-grace-gocache.
Executable SHA256 `f38a8e925b439a9a0627ed94c4e6f5273d788d00d0e652165516e24a31d5360c`
сохранён [pipeline.test](promoted-arithmetic-complete/pipeline.test);
[replay](promoted-arithmetic-complete/replay.txt).
Это focused proof текущего source, не новый общий pipeline/CLI gate.

[Артефакты](promoted-arithmetic-complete/manifest.json): exact source/raw MIR,
три full ASM/BIN/final MIR/function-contract packets, all-entry CALL driver bytes,
per-mode counts/ABI spill exceptions/stack depth, logs, executable и SHA256.
Скорость Z80 здесь не замерялась как optimisation benefit. 5B разница модулей
сообщается отдельно от correctness, а не как обобщённый результат Grace.
