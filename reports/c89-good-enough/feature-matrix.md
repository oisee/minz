# C89: матрица практического покрытия

Рабочий каталог целей, не процент соответствия ISO. Каждая строка называет
проверяемое семейство и его пределы. «Проверено» означает конкретные независимые
ожидания в исполняемых тестах; успешный разбор всего файла этого не заменяет.
Aggregate scalarization интегрирована в main d6404fa4 через PR #163;
switch/control-flow — в main 8fd960e5 через PR #164. Исправление ABI импортов
пока в исследовательской ветке. Полная цель остаётся активной.

| Семейство | Исполняемое доказательство | Состояние и граница |
|---|---|---|
| char/short/int/long, integer promotions | `TestCIntegerSemanticsNativeVM` | VM/Z80; target char=8, short/int/pointer=16, long=32 |
| signed/unsigned сравнения, сдвиги, compound assignment | `TestCIntegerSemanticsNativeVM` | VM/Z80 на граничных парах; UB signed overflow не используется |
| 32-bit multiply/divide/mod | `TestCIntegerSemanticsNativeVM` | C→raw/final VM/Z80 на граничных значениях; отдельно расширить wide CALL с живыми значениями caller |
| Pointer stride, difference, arithmetic | `TestCPointerArithmeticNativeVM` | VM/Z80; complete object types, target адреса 16-bit |
| Lvalue, postfix, compound: вычислить адрес один раз | `TestCLvalueSingleEvaluationNativeVM` | VM/Z80; побочные эффекты и адреса |
| if/while/for/do, continue и shadowing | `TestControlContinueShadowProbe` | raw/final VM/Z80; требуется расширить независимые семейства |
| switch fallthrough/default/labels/nearest break | `TestC89ControlMatrixVMZ80` | VM/Z80; 10 семейств, интегрировано через PR #164 |
| goto и join пути | `TestC89ControlMatrixVMZ80`, HIR goto suite | Простой C goto и регрессия join подтверждены; сложные C переходы требуют дополнительной матрицы |
| logical/conditional/comma effects | `TestC89ExpressionEffectsNative`, `TestC89ControlEffectsNative` | C source + GCC frozen oracle; raw/final VM/Z80, порядок событий, память и PC/SP; deep profile |
| Local/static storage, initialization | `TestCScalarConversionsNativeVM`, `TestCLocalAddressesNativeVM` | VM/Z80; отдельные адресуемые объекты, static long/array/struct |
| Private symbols, extern, separate imports | `TestCSourcePrivateSymbolsNativeVM`, `TestCSourceImportNativeVM`, `TestC89AggregateImportVMZ80` | Mixed C/Nanz VM/Z80; новое разложение aggregate ABI после связывания, пока в ветке |
| volatile scalar/global/field/local | `TestCVolatileAccessNativeVM`, `TestCLocalVolatileTrafficNativeVM` | VM/Z80, observable access count/order |
| volatile aggregate/by-value | `TestCStructParametersNativeVM`, `TestC89AggregateParameterMemoryFallback` | Copy semantics и память; раздельно проверять traffic и ABI |
| Struct layout, array/struct initializer | `TestCAggregateStorageNativeVM` | VM/Z80, bytes/layout; C99 designated cases считать отдельно от C89 |
| Nested struct/union/overlapping bytes | `TestCNestedUnionLayoutNativeVM` | VM/Z80; memory fallback, не shallow scalar optimization |
| By-value struct input | `TestCStructParametersNativeVM`, `TestC89AggregateScalarizationBehavior` | Copy semantics direct/indirect/recursive и изменение копии |
| Small struct return → multiple values | `TestC89AggregateScalarizationABI`, `TestC89AggregateScalarizationBehavior` | Два HIR/MIR result slots, native значения; ≤4 bytes shallow fields |
| Small struct input → multiple arguments | Те же aggregate tests | Два HIR/MIR parameter slots, no alloca; адрес/indirect/volatile сохраняют memory ABI |
| Enum width/value | `TestCEnumWidthsNativeVM` | VM/Z80; конкретный ABI для крайних значений |
| sizeof/layout | `TestCIntegerSemanticsNativeVM`, aggregate storage cases | Подтверждён target layout; расширить случаи unevaluated expressions |
| Arrays/string/byte initializers | aggregate storage, `TestOriginalC89StringMemoryVMZ80` | Original string8.c, strlen/memset/memcpy/memcmp на VM/Z80; конкретные layouts и диапазоны, не вся libc |
| Function pointers | `TestCStructParametersNativeVM`, aggregate indirect case | VM/Z80 на typed signatures; varargs ABI не следует из этого |
| Includes/preprocessor | include tests + upstream FatFS workload | Реальный код и вложенные includes работают; добавить native oracles для #if/#/## |
| K&R definitions, implicit int/prototypes | C parser compatibility | Отдельного executable C89 допуска пока нет |
| Variadic functions/stdarg | Старый minimal header присутствует | Не объявлено проверенным; аудит ABI и явных диагностик остаётся |
| Bitfields | `TestCRejectsUnsupportedBitfieldStorageAndAccess` | Явно отклоняются для Z80; полный ISO C89 не заявляется |
| float/double/long double | `TestCRejectsUnsupportedFloatingTypes` | Явно отклоняются для Z80 |
| Hosted standard library | Embedded/runtime helpers и FAT12 | Полная hosted libc вне нынешнего допуска; проверять только заявленные helpers |

Расширения C99 `_Bool`, designated initialization, long long и VLA не являются
доказательством C89. `_Bool` имеет отдельные semantic tests; unavailable long
long/VLA отклоняются. Их наличие в прежнем C-корпусе не увеличивает оценку C89.

Следующие полезные работы: подтвердить wide CALL и varargs diagnostics,
добавить preprocessor/sizeof/K&R cases, отдельно проверить время жизни
неразложенных возвращаемых структур и цену передачи scalar slots. После этого оценить remaining gaps и выполнить строгую
интеграцию и воспроизведение в main. Матрица намеренно сохраняет открытые строки.
