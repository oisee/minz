# C89: матрица практического покрытия

Рабочий каталог целей, не процент соответствия ISO. Каждая строка называет
проверяемое семейство и его пределы. «Проверено» означает конкретные независимые
ожидания в исполняемых тестах; успешный разбор всего файла этого не заменяет.
Aggregate scalarization интегрирована в main d6404fa4 через PR #163;
switch/control-flow — в main 8fd960e5 через PR #164. Исправление ABI импортов — в main ce6d79ba через PR #165. Lifetime интегрирован в main 97e78d71 через PR #166.
Language-admission — PR #167; wide CALL, стоимость scalar ABI и глобальные
relocations проверены в следующей ветке, пока не в main. Полная цель остаётся активной.

| Семейство | Исполняемое доказательство | Состояние и граница |
|---|---|---|
| char/short/int/long, integer promotions | `TestCIntegerSemanticsNativeVM` | VM/Z80; target char=8, short/int/pointer=16, long=32 |
| signed/unsigned сравнения, сдвиги, compound assignment | `TestCIntegerSemanticsNativeVM` | VM/Z80 на граничных парах; UB signed overflow не используется |
| 32-bit multiply/divide/mod | `TestCIntegerSemanticsNativeVM` | C→raw/final VM/Z80 на граничных значениях; wide CALL с живыми caller значениями: `TestC89WideCallLiveVMZ80`, 2025 native / 2700 VM вызовов, пока в ветке |
| Pointer stride, difference, arithmetic | `TestCPointerArithmeticNativeVM` | VM/Z80; complete object types, target адреса 16-bit |
| Lvalue, postfix, compound: вычислить адрес один раз | `TestCLvalueSingleEvaluationNativeVM` | VM/Z80; побочные эффекты и адреса |
| if/while/for/do, continue и shadowing | `TestControlContinueShadowProbe` | raw/final VM/Z80; требуется расширить независимые семейства |
| switch fallthrough/default/labels/nearest break | `TestC89ControlMatrixVMZ80` | VM/Z80; 10 семейств, интегрировано через PR #164 |
| goto и join пути | `TestC89ControlMatrixVMZ80`, HIR goto suite | Простой C goto и регрессия join подтверждены; сложные C переходы требуют дополнительной матрицы |
| logical/conditional/comma effects | `TestC89ExpressionEffectsNative`, `TestC89ControlEffectsNative` | C source + GCC frozen oracle; raw/final VM/Z80, порядок событий, память и PC/SP; deep profile |
| Local/static storage, initialization | `TestCScalarConversionsNativeVM`, `TestCLocalAddressesNativeVM` | VM/Z80; отдельные адресуемые объекты, static long/array/struct |
| Private symbols, extern, separate imports | `TestCSourcePrivateSymbolsNativeVM`, `TestCSourceImportNativeVM`, `TestC89AggregateImportVMZ80` | Mixed C/Nanz VM/Z80; aggregate ABI после связывания интегрирован через PR #165 |
| volatile scalar/global/field/local | `TestCVolatileAccessNativeVM`, `TestCLocalVolatileTrafficNativeVM` | VM/Z80, observable access count/order |
| volatile aggregate/by-value | `TestCStructParametersNativeVM`, `TestC89AggregateParameterMemoryFallback` | Copy semantics и память; раздельно проверять traffic и ABI |
| Struct layout, array/struct initializer | `TestCAggregateStorageNativeVM` | VM/Z80, bytes/layout; C99 designated cases считать отдельно от C89 |
| Nested struct/union/overlapping bytes | `TestCNestedUnionLayoutNativeVM` | VM/Z80; memory fallback, не shallow scalar optimization |
| By-value struct input | `TestCStructParametersNativeVM`, `TestC89AggregateScalarizationBehavior` | Copy semantics direct/indirect/recursive и изменение копии |
| Small struct return → multiple values | `TestC89AggregateScalarizationABI`, `TestC89AggregateScalarizationBehavior` | Два HIR/MIR result slots, native значения; ≤4 bytes shallow fields |
| Small struct input → multiple arguments | Те же aggregate tests | Два HIR/MIR parameter slots, no alloca; адрес/indirect/volatile сохраняют memory ABI |
| Цена aggregate scalar ABI | `TestC89AggregateTransferCostVMZ80` | Пара primary/heldout, реальные буферы против tuple slots; 732 native / 1464 VM вызовов, пока в ветке |
| Возврат большой структуры и косвенный aggregate return | `TestC89AggregateReturnLifetimeVMZ80` | Буфер caller и копирование до освобождения frame; интегрировано через PR #166 |
| Глобальные address initializers | `TestC89GlobalRelocationsVMZ80`, `TestC89GlobalRelocationImportVMZ80` | Строки, поля, массивы, static local, function pointers, C/C и Nanz/C; 231 native / 308 VM вызовов, пока в ветке |
| Косвенный вызов с живым out-pointer | `TestC89IndirectOutPointerAdmission` | Typed void CALL с живым адресом локальной структуры: 15 native / 20 VM, исправлен admission, пока в ветке |
| Enum width/value | `TestCEnumWidthsNativeVM` | VM/Z80; конкретный ABI для крайних значений |
| sizeof/layout | integer/storage cases + `TestC89LanguageAdmissionVMZ80` | Target layout и sizeof без вычисления x++/вызова; новый source-level набор пока в ветке |
| Arrays/string/byte initializers | storage/string8 + language admission/string import suites | Escapes, соседние literals, NUL, C/Nanz linking, CString/SString identity; новая literal linkage пока в ветке; global pointer relocation имеет отдельный VM/Z80 набор в ветке |
| Function pointers | `TestCStructParametersNativeVM`, aggregate indirect case | VM/Z80 на typed signatures; varargs ABI не следует из этого |
| Includes/preprocessor | include/FatFS + `TestC89LanguageAdmissionVMZ80` | #if/#/## имеют raw/final VM/Z80 оракулы в новом наборе; не все варианты preprocessing |
| K&R definitions, implicit int/prototypes | `TestC89LanguageAdmissionVMZ80` | K&R byte/int parameters, implicit int, forward no-prototype call на VM/Z80; новая матрица пока в ветке |
| Variadic functions/stdarg | `TestC89VariadicAdmission` | Новый frontend явно отклоняет variadic definition/direct/indirect call; неиспользуемые prototypes допустимы; runtime ABI не реализован |
| Bitfields | `TestCRejectsUnsupportedBitfieldStorageAndAccess` | Явно отклоняются для Z80; полный ISO C89 не заявляется |
| float/double/long double | `TestCRejectsUnsupportedFloatingTypes` | Явно отклоняются для Z80 |
| Hosted standard library | Embedded/runtime helpers и FAT12 | Полная hosted libc вне нынешнего допуска; проверять только заявленные helpers |

Расширения C99 `_Bool`, designated initialization, long long и VLA не являются
доказательством C89. `_Bool` имеет отдельные semantic tests; unavailable long
long/VLA отклоняются. Их наличие в прежнем C-корпусе не увеличивает оценку C89.

Новые доказательства: [wide CALL и стоимость scalar ABI](call-abi-and-cost.md),
[глобальные адресные инициализаторы](global-address-initializers.md).
Следующие работы: завершить строгую интеграцию admission и новых ABI/relocation
проверок, воспроизвести их на main и оценить оставшиеся пробелы в матрице.
Runtime varargs и неизвестные indirect parameter ABI с живыми caller значениями явно
не входят в заявленный Z80 допуск. Матрица сохраняет открытые строки.
