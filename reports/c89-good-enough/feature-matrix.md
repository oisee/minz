# C89: матрица практического покрытия

Рабочий каталог целей, не процент соответствия ISO. Каждая строка называет
проверяемое семейство и его пределы. «Проверено» означает конкретные независимые
ожидания в исполняемых тестах; успешный разбор всего файла этого не заменяет.
База с aggregate scalarization интегрирована в main d6404fa4 через PR #163;
switch delta пока в отдельной ветке. Полная цель остаётся активной.

| Семейство | Исполняемое доказательство | Состояние и граница |
|---|---|---|
| char/short/int/long, integer promotions | `TestCIntegerSemanticsNativeVM` | VM/Z80; target char=8, short/int/pointer=16, long=32 |
| signed/unsigned сравнения, сдвиги, compound assignment | `TestCIntegerSemanticsNativeVM` | VM/Z80 на граничных парах; UB signed overflow не используется |
| 32-bit multiply/divide/mod | native wide arithmetic suite + C frontend mapping | Нужны отдельные строгие C89 сквозные случаи; MIR-only проверка не закрывает строку |
| Pointer stride, difference, arithmetic | `TestCPointerArithmeticNativeVM` | VM/Z80; complete object types, target адреса 16-bit |
| Lvalue, postfix, compound: вычислить адрес один раз | `TestCLvalueSingleEvaluationNativeVM` | VM/Z80; побочные эффекты и адреса |
| if/while/for/do, continue и shadowing | `TestControlContinueShadowProbe` | raw/final VM/Z80; требуется расширить независимые семейства |
| switch fallthrough/default/labels/nearest break | `TestC89ControlMatrixVMZ80` | VM/Z80; новый delta 9e31d824, ещё не main |
| goto и join пути | `TestC89ControlMatrixVMZ80`, HIR goto suite | Простой C goto и регрессия join подтверждены; сложные C переходы требуют дополнительной матрицы |
| logical/conditional/comma effects | `TestConditionalExpressionEffectsRaw`, `TestControlEffectsRawVM` | Часть проверок raw-only: добавить C89 final/native coverage |
| Local/static storage, initialization | `TestCScalarConversionsNativeVM`, `TestCLocalAddressesNativeVM` | VM/Z80; отдельные адресуемые объекты, static long/array/struct |
| Private symbols, extern, separate imports | `TestCSourcePrivateSymbolsNativeVM`, `TestCSourceImportNativeVM` | VM/Z80 на конкретных mixed C/Nanz источниках |
| volatile scalar/global/field/local | `TestCVolatileAccessNativeVM`, `TestCLocalVolatileTrafficNativeVM` | VM/Z80, observable access count/order |
| volatile aggregate/by-value | `TestCStructParametersNativeVM`, `TestC89AggregateParameterMemoryFallback` | Copy semantics и память; раздельно проверять traffic и ABI |
| Struct layout, array/struct initializer | `TestCAggregateStorageNativeVM` | VM/Z80, bytes/layout; C99 designated cases считать отдельно от C89 |
| Nested struct/union/overlapping bytes | `TestCNestedUnionLayoutNativeVM` | VM/Z80; memory fallback, не shallow scalar optimization |
| By-value struct input | `TestCStructParametersNativeVM`, `TestC89AggregateScalarizationBehavior` | Copy semantics direct/indirect/recursive и изменение копии |
| Small struct return → multiple values | `TestC89AggregateScalarizationABI`, `TestC89AggregateScalarizationBehavior` | Два HIR/MIR result slots, native значения; ≤4 bytes shallow fields |
| Small struct input → multiple arguments | Те же aggregate tests | Два HIR/MIR parameter slots, no alloca; адрес/indirect/volatile сохраняют memory ABI |
| Enum width/value | `TestCEnumWidthsNativeVM` | VM/Z80; конкретный ABI для крайних значений |
| sizeof/layout | `TestCIntegerSemanticsNativeVM`, aggregate storage cases | Подтверждён target layout; расширить случаи unevaluated expressions |
| Arrays/string/byte initializers | aggregate storage/native examples | Покрытие конкретных layouts; нужна отдельная чистая C89 строковая матрица |
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

Следующие полезные работы: закрыть raw-only conditional effects на Z80,
проверить C89 32-bit arithmetic и varargs diagnostics, добавить preprocessor/
string/sizeof/K&R cases. После этого оценить remaining gaps и выполнить строгую
интеграцию и воспроизведение в main. Матрица намеренно сохраняет открытые строки.
