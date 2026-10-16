# C89: декомпозиция до четырёх байт

16 октября 2026 года. Основной механизм интегрирован в main d43e8559
через PR #170. Этот checkpoint расширяет проверки границы shallow scalar ABI.

Все 11 порядков полей шириной 1/2 байта с суммой до четырёх байт,
плюс signed word/byte/byte и byte/byte/word. Factory заполняет объект присваиваниями полям,
consumer получает его по значению, caller использует исходный x после CALL.
Triple и Mixed намеренно имеют одинаковые имена полей при разных типах.

```c
struct Quad { unsigned char a,b,c,d; };
struct Quad make_quad(unsigned char x) {
    struct Quad p;
    p.a=x; p.b=x+1; p.c=x^90; p.d=x+3;
    return p;
}
```

Без brace initializer возврат раньше сохранял буферный ABI, хотя приём
структуры уже раскладывался. Теперь scoped analysis использует объявленный
тип объекта; initializer не нужен для идентификации типа. Возврат читает
текущие значения полей. После безопасной проверки всех использований локальные
поля становятся scalar declarations с последующими присваиваниями.
У volatile/escaping storage остаются прежние ограничения. Удаление исходных
field assignments применяется только к действительно переписанному старому
pointer-return шаблону, а не к возврату текущего значения структуры.

Новая проверка также нашла настоящий native сбой: `quad_check(0)` выдавал
31 вместо 379, при правильном raw/final VM. Третий и четвёртый byte-результаты
имели ClassGeneral с одним каноническим регистром C. Вывод четвёртого результата
перезаписывал третий. Четвёртый byte-result теперь имеет отдельный ClassRegD;
caller и callee используют согласованную четвёрку A/B/C/D.

`TestC89AggregateScalarWidthVMZ80` проверяет число и типы реальных MIR argument
и result slots, отсутствие alloca, независимую unsigned-модель и native PC/SP.
13 форм × 12 входов × 4 режима (default, Grace, Size, без contract optimization)
дают 624 native запуска; raw плюс четыре final VM — 780 вызовов. GCC
`-std=c89 -pedantic-errors` независимо подтвердил 3328 результатов на всех
256 входах 13 форм. Signed arithmetic остаётся в определённом диапазоне C int.

Режим без contract optimization нашёл отдельный сбой Z80: RHS-константа 32
имела номинальный HL, но её загрузка была удалена как ненужная для immediate.
Перестановка операндов MUL принимала старое содержимое HL за значение константы.
Теперь константа остаётся справа. Forced-allocation матрица проверяет 3400
произведений с 17 константами; прежние 2000 случаев переменного HL/DE умножения
сохраняются. Это обычный backend fix, не требование включать Grace/PFCCO.

Forwarding проверяется отдельно: nested/assignment calls сохраняют буферный ABI,
вызывают factory ровно один раз и не изменяют исходный объект при передаче копии.
`TestC89AggregateForwardingVMZ80`: 81 native / 108 VM. Наличие этой проверки
не означает, что все формы aggregate expressions уже scalarize.

```sh
cd minzc
go test ./pkg/c89 -run '^TestC89AggregateScalarWidthVMZ80$' -count=1
go test ./pkg/c89 ./pkg/hir -run '^(TestC89Aggregate|TestPromote|TestStructPromotion|TestCVolatile)' -count=1
```

Проверки значений и ABI разделены. Типы float, bitfield, overlapping/nested
storage и структуры больше четырёх байт не превращаются в этот scalar ABI.
Их существующие диагностики либо корректный буферный ABI сохраняются.
Протоколы, исходное падение, raw/final MIR, ASM и строгий host oracle сохранены
локально в `minz-artifacts/grace-reports/2026-10-16/c89-good-enough`.
