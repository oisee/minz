# Знаковое widening: все допустимые Z80 homes

База `b1adbf09`, prerequisite unsigned donor `b212c6cc` (локальный cherry-pick
`c4be2cb0`). Собственный production change — только `genSextValue` в
`z80codegen_move.go`; `genExt` и unsigned helper не изменены.

**Default ordinary source проходил до исправления.** Шесть widths i8→i16/i24/i32,
i16→i24/i32, i24→i32, прямой cast и параметр/return helper сравниваются с raw VM,
final VM и независимым native oracle. Старый wide spill staging переводил эти
casts в pair homes и скрывал ошибки внутреннего emitter.

При допустимом physical home обнаружен настоящий counterexample: `OpSext`
i16→i32, source HL=0, destination LocMem, poison RAM 0xA5. Старый emitter
пишет только младший word, результат **0xA5A50000 вместо 0**. [Замороженный
FAIL](signed-wide-extension/counterexample-before.txt) и exact ASM/BIN в
[physical-before](signed-wide-extension/physical-before/). Это прямой
native emitter fixture, **не default source miscompile**. Другие допустимые
shadow/index/spill homes давали invalid assembly либо неправильную sign lane;
[полный первоначальный FAIL](signed-wide-extension/physical-before.txt).

Новый extender сохраняет каждый исходный byte на стек до записи в результат,
берёт sign из последнего оригинального byte, пишет sign fill и восстанавливает
исходные bytes в destination. Перекрытие source с destination/main/shadow halves
безопасно. Существующий outer PUSH/POP AF сохраняет live A и все F bits, включая
физический pending predicate. Для register i24 unused четвёртый byte равен нулю;
для memory i24 пишутся ровно три bytes. Тест проверяет byte непосредственно
после spill, чужие RAM/code/literals, A/F и AF', обе банки пар и index bytes.
Temporary stack максимум **8 байт** внутри extender, плюс caller return.

Оракул: Book v7 §§2.9,3.1,3.2 (casts, widths, signed representation);
Go int8/int16 и арифметический sign-extension int24, с отсечением destination
width. Не используется алгоритм emitter для expected value.

- `TestSignedExtensionNative`: **148** допустимых source/destination/width homes;
  all256 i8, boundary samples для word/triple, четыре F states — **100 368**
  actual native executions. [PASS 16.387 s](signed-wide-extension/physical-after.txt).
  Включены A/C/H, IXL/IYH, B'/A', memory, HL/DE/IX/IY, destination HL/DE/BC/memory;
  неподдерживаемые физически 24-bit IX/IY sources исключены по типу home.
- `TestSignedWideExtensionSourceNative`: **1 590** input/mode executions,
  raw/final/native, all-byte i8 и word/triple boundaries, code/literal guard.
  [До PASS 0.062 s](signed-wide-extension/before.txt),
  [после PASS 0.086 s](signed-wide-extension/after.txt).
- [Full MIR2/HIR](signed-wide-extension/gates.txt): **PASS 72.053/24.435 s**.
  Go 1.26.8, GOCACHE=/tmp/minz-grace-gocache; данные этого isolated checkpoint,
  не новый parent fullpipeline/CLI gate.

[Source/ASM/BIN/final MIR до](signed-wide-extension/before/) и
[после](signed-wide-extension/after/) фиксируют default поведение, включая
параметр/return ABI. [Physical emitter snippets](signed-wide-extension/physical-after/)
сохраняют все 148 homes; exact comparison ASM/BIN заморожен для конкретного
counterexample. [Манифест SHA256](signed-wide-extension/manifest.json).

Это prerequisite безопасных allocation variations и checked certificates.
Нет заявления, что default allocator выбирал ошибочное home, о полном int16/int24
Cartesian переборе или выигрышах скорости/размера.
