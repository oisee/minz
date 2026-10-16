# C89: декомпозиция до четырёх байт

16 октября 2026 года, `feat/c89-aggregate-widths`. Этап пока не интегрирован
в main; проверяет границу обещанного shallow scalar ABI, дополняя Pair.

Четыре строгие C89 формы: три byte-поля; четыре byte-поля; два word-поля;
word плюс два byte-поля. Factory заполняет объект присваиваниями полям,
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
4 формы × 12 входов × 3 режима — 144 native запуска; raw плюс три final VM —
192 вызова. Все прошли. GCC `-std=c89 -pedantic-errors` независимо подтвердил
1024 результата на всех 256 входах четырёх форм.

Полный core до последних marker/volatile уточнений: C89 37.991 s,
HIR 28.799 s, MIR2 104.044 s, Nanz 25.774 s, interop 0.003 s, PASS.
После marker уточнения aggregate/promotion tests — C89 4.127 s и HIR 0.011 s,
PASS. После volatile guard: C89 4.765 s и HIR 0.010 s, PASS.

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
