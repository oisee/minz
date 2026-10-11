# Original typed_print and production text.print: native correctness

Exact source `examples/nanz/typed_print.nanz` и реальная dependency `stdlib/text/print.nanz` не менялись; их точные копии/SHA сохранены. Это pending corpus consumer production stdlib, который раньше проверялся raw VM и ASM validation без native execution. Выполнены полные исходные `main`, `probe_zero`, `probe_overflow` по два раза в persistent state. Main даёт `score=65535 u8=175` + NUL, zero — `0` + NUL, overflow — bounded `100` без NUL. Нет source asserts или OS output; результат — bytes и descriptor state.

Независимый oracle — Go decimal `strconv.Itoa` плюс bounded writer: min(length,capacity), overflow при заполнении, NUL только при свободном месте. Проверяются все 44 байта пяти globals (32 output,7 writer,2 text,2 dec,1 small), persistent tails и поля структуры. Writer.buf сравнивается с relocated output address данной VM/native машины; CText.ptr проверяется через точные CString bytes, а не приравнивается к другому address space. [Language Book v7 §2.6](../Nanz_Language_Book_v7.md#26-pointers-and-arrays) задаёт pointer bytes; [глава 4](../Nanz_Language_Book_v7.md#chapter-4-structs-methods-and-interfaces) — concrete impl dispatch; [§18.1](../Nanz_Language_Book_v7.md#181-three-string-types) — NUL CString. Capacity/finish semantics явно документированы actual stdlib.

Raw и final MIR VM дополнительно сверяют точный ordered output stores. Собранный Z80 проверяет все global bytes, PC/SP возврата, stack bounds/guards, запрет любых writes за пределами exact globals, named compiler spills и stack, immutable весь остальной image. Native режим flat all-RAM, без утверждения о ROM/физической ОС. Native порядок stores отдельно не трассируется; полное persistent состояние проверяется после каждого entry.

Проверка нашла два actual production miscompile:

- `OpField/OpPtrBump` для offset4+ использовали BC scratch, затирая live cap32 в 4; маршрутизация через HL также теряла live base. Exact main возвращал 0 и пустой output. Backend теперь сохраняет live HL и scratch pair вместе с их live byte aliases вокруг constant offset add.
- HIR ordinary CallExpr создавал move struct receiver с storage type `Dec8` width8, хотя фактически передавал 16-bit address формальному `^Dec8`. One-byte spill pointer давал output `u8=0`, main16 вместо18. Теперь только struct expression, переданный formal pointer, использует pointer width/type для ABI class move. Scalar/by-value semantics не расширялись. Исходный алгоритм библиотеки не переписан.

48 независимых struct/capacity/byte результатов отдельно проверяют receiver spill и field scratch. Additional formatter workload вызывает тот же настоящий stdlib через 13 values ×8 capacities: 0,1,9,10,99,100,255,256,999,1000,32767,32768,65535; capacities0,1,2,3,4,5,6,32. Эти 104 drivers — отдельный bounded workload, не замена original main.

Два образа сохранены отдельно: `original` компилирует только exact source (6 calls), `formatter-boundaries` добавляет 104 test drivers (110 calls). Размер/ SHA в focused-final.txt включают implementation spills и drivers; это correctness proof, не optimization benchmark. Исторические before/partial-after captures не переписаны. Дополнительный m18 HIR failure был existing multiply relocation issue; root fix ee5621ac cherry-picked как dependency, не входит в новый patch.

Каталог сохраняет все194 paths: теперь31 VM/native bounded proofs плюс отдельный1 native-CPM-only proof. Это не доказательство всех interfaces или всей stdlib. Focused результаты и полные HIR/MIR2/Nanz проверки сохранены. Полный медленный pipeline не запускался.

Повтор из `minzc`: `go test ./pkg/pipeline -run '^(TestOriginalTypedPrintNativeFullState|TestTypedPrintImportedFormatterBoundaryNative|TestStructMethodPointerAndFieldScratchNative)$' -count=1 -v`; экспорт `MINZ_TYPED_PRINT_NATIVE_ARTIFACTS=/absolute/path`.
