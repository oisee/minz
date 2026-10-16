# C89: старые сигнатуры, строки и явный предел varargs

16 октября 2026 года. Интегрировано в main 1247c19d через
[PR #167](https://github.com/oisee/minz/pull/167), поверх aggregate linking/lifetime.
Все шесть обязательных CI checks прошли на окончательном head 761f9136.
Advisory sweep: один ASM изменён (FatFS), exit/stdout/stderr изменений нет.

Новый строгий C89 исходник `testdata/language_admission/core.c` проверяет:

- K&R-определения, implicit int и вызов до полного прототипа;
- `sizeof(x++)` и `sizeof(touch())` без побочных эффектов;
- `#if`, `defined`, stringification `#` и token concatenation `##`;
- escapes, adjacent literals, встроенный NUL и конечный NUL C-строки.

Исходный lowering строки возвращал число — её индекс в таблице — вместо
адреса данных. Поэтому `STRING(packet)[0]` читалось как ноль и на raw VM,
и на собранном Z80. Получалось 19 вместо 131. Теперь frontend использует
разобранное значение строки из C-parser (включая escapes/concatenation) и
настоящий адрес CString в HIR. Одинаковые literals в одной единице компиляции
не получают несуществующие номера после дедупликации MIR pool.

Отдельный C→C/Nanz→C тест проверяет связывание нескольких строковых пространств.
Номера литералов импортируемого модуля переводятся в номера вызывающего,
кодирование сохраняется. Общий `hir.MergeImportedStrings` различает CString
и SString с одинаковым текстом. Это предотвращает чтение длины SString как
первого символа CString и сохраняет terminating NUL. Ошибочные imported
indices отклоняются до частичного обновления строковых ссылок.

`TestC89LanguageAdmissionVMZ80`: четыре наблюдаемых функции × девять входов
× три режима = 108 native и 144 raw/final VM наблюдения.
`TestC89StringImportVMZ80`: два фронтенда × одиннадцать входов × три режима
= 66 native и 88 VM наблюдений. Все проходят default/Grace/Size; native
assert runner проверяет результат, PC возврата и восстановление SP.
Две HIR-проверки отдельно подтверждают кодирование/дедупликацию и отказ без
частичного обновления при плохом индексе.

Strict host C89 сборка `cc -std=c89 -pedantic-errors -Wall -Wextra` подтвердила
runtime ожидания. Host `sizeof(int/long)` намеренно не используется как oracle
целевого Z80: там char=1, int=2, long=4 байта. Host-значения записаны отдельно.
Полный C89/HIR/Nanz/interop/C-parser после связывания строк — PASS
(29,670 / 28,989 / 26,329 / 0,002 с; C-parser cached).

Runtime varargs ABI пока отсутствует. Ранее `int sum(int n,...) { return n; }`
принималось молча, а исторический `stdarg.h` предполагал stack layout,
которого register ABI не обещает. Теперь variadic definitions, direct calls
и indirect calls отклоняются явной диагностикой frontend. Неиспользуемые
variadic prototypes остаются допустимыми для заголовков. Четыре отрицательные
случая и один положительный проверяют именно этот предел. Предложение
compile-time printf из ADR-0026 не объявляется реализованным runtime ABI.

Открыты wide CALL с живыми значениями caller, измерение цены scalar contracts,
global string pointer relocations, строгая интеграция и postmerge доказательства.
Полный hosted C и полное ISO соответствие не заявляются.
