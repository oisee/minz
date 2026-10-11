# Полноразрядное сравнение u32/i32 на Z80

База `e941fc14`. Исправлено реальное ordinary-source расхождение:
`a:u32=0`, `b:u32=256`, `u8(a<b)` даёт raw/final MIR VM **1**, прежний
Z80 **0**. [Заморожены source/ASM/BIN/MIR](u32-comparison-native/before/)
и [исходный FAIL](u32-comparison-native/before.txt).

Причина: `genCmp` распознавал широкий операнд только по allocation `LocDWord`.
Spilled u32 попадал в byte `CP`: младшие байты 0 и 256 совпадают. Старый
`genCmp32` также молча возвращался при lhs не HL, не устанавливал pending flag,
проверял Z только последнего word и не обеспечивал signed ordering.

Изменение ограничено dispatch и `genCmp32` в `z80codegen_cmp.go`. Dispatch
использует SrcTy/regInfo вместе с allocation. Новый emitter сохраняет исходные
четыре байта обоих операндов на стек до scratch-записей, затем сравнивает байты
от старшего к младшему. Signed high-byte bias 0x80 задаёт int32 ordering.
Получаются full-value C/Z; compound <=/> и bool-return получают правильную
метаинформацию. A, IX, BC и обе банки всех operand pairs сохранены. Максимальный
временный стек **22 байта**; это исправление корректности, не заявленный выигрыш
скорости или размера. Shift/F-writer helper изменений нет; соседний агент
исправляет flagStillNeeded независимо.

Независимый оракул выводится из Nanz Language Book v7 §§3.1,3.2,3.4:
ширина u32/i32, signed ordering и EXX representation. Go uint32/int32 модели
не используют emitter flags или его алгоритм.

- `pipeline/TestU32ComparisonSourceNative`: шесть операторов ==, !=, <, <=, >,
  >= для u32/i32; прямое выражение и настоящий двухблочный helper CALL.
  **135 000** входных пар/вариантов (24 × 75²), raw VM, final VM и native.
  Каждая из 32 позиций варьируется walking bit и inverse bit; zero, byte/word
  границы, signed границы и смешанный pattern. Return PC/SP и неизменность
  code/literals проверяются; разрешены только source globals и generated spills.
- `mir2/TestNativeWideComparisonAllHomes`: десять signed/unsigned CmpCond,
  восемь размещений (HL/DE/BC, одинаковая pair, spill/pair, pair/spill,
  spill/spill, immediate), 12² значений — **11 520** native запусков.
  Прямая emission проверяется без allocator repair: full C/Z, original A,
  IX, SP и все шесть main/shadow pair words.
- [Focused source gate](u32-comparison-native/after.txt): **PASS 1.340 s**.
- [Physical gate](u32-comparison-native/physical.txt): **PASS 7.577 s**.
- [Полные MIR2/HIR](u32-comparison-native/gates.txt): **PASS 29.197/24.776 s**.
  Go 1.26.8, GOCACHE=/tmp/minz-grace-gocache. Это gates данного isolated среза;
  текущий parent fullpipeline/CLI этим отчётом не объявляется PASS.

[Точные 24 compiled packets](u32-comparison-native/after/) сохраняют source,
ASM, BIN и final MIR. [SHA256](u32-comparison-native/manifest.json).
Не утверждается полный Cartesian 2^64, полнота всех widths, u8→u32 extension
или полный SHA256. Исправление исходникового u32 comparison не зависит от
исследовательских таблиц и Grace-флагов.
