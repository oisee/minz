# functional.lizp: whole original module and real indirect calls

База `075ec3f0`. Неизменённый `examples/lizp/functional.lizp`, SHA256
`ef3ea461b1bb82e294821a59350e7474a673d87f88c2a5addade7360606c5796`.
Parsed source сохраняет все 10 исходных assertions и восемь compiled entries:
apply, lambda_0, test_double, pythag_sq, weekday, lambda_1, lambda_2, transform.
Production code и syntax policy не изменены.

Независимые модели повторяют source mathematics, не выполняют compiler IR:
byte arithmetic modulo256; pythag_sq=(x*x+y*y)&255; weekday 0/1/2→77/84/87,
всё остальное63; transform mode0→2*x, mode1→x+10, остальные→x.
let* xx/yy — локальные scoped значения. **Все три исходные лямбды не имеют
lexical captures**; этот пакет не доказывает runtime environment ABI замыканий.

| Entry | Domain на каждый режим |
|---|---|
| pythag_sq | все65 536 byte pairs |
| transform | все65 536 mode/value byte pairs |
| weekday | все256 bytes |
| test_double | единственный zero-argument entry, результат10 |
| lambda_0, lambda_1, lambda_2 | каждая all256 bytes, соответственно2*x,2*x,x+10 |
| apply | all256 bytes × каждый из трёх реальных lambda addresses:768 calls |

Per mode **132 865** input invocations, raw MIR VM, final MIR VM и actual Z80.
Три режима ordinary/GraceSize/GraceSpeed: **398 595** native executions.
Все modes сохраняют **117 B полного module BIN** (включая trampoline/mul8).
Это отсутствие изменения размера в этом примере, не выигрыш Grace.

Для raw/final VM function-pointer arguments указывают на VM GlobalAddr того же
реального source target; native получает настоящий assembled symbol address.
Проверка не заменяет indirect path вызовом известной функции. В final MIR у
apply остаётся OpCallIndirect; в ASM — JP __call_ix/JP(IX). Test-double и
transform доходят до apply через tail JP, pythag_sq реально вызывает __mul8.
Native trace требует посещения apply и нужной lambda entry ровно один раз
для каждого higher-order invocation. Без этого source model мог бы скрыть
незаконную специализацию; здесь path также проверен.

Each entry вызывается opcode CALL из отдельного фиксированного driver, source
не дописывается. Actual allocation задаёт parameters, actual contract — return.
Return PC — байт после CALL, SP восстановлен0xf000. Максимальный временный
стек: pythag_sq10B, apply/test_double/transform4B, простые entries2B.
Вся64KiB RAM poison-initialized и сравнивается после calls. Source globals и
spill slots отсутствуют: единственное разрешённое изменение — реальные
stack bytes [0xeff6,0xf000). Code/literals/drivers/остальная RAM неизменны.

[Retained executable gate](lizp-functional-complete/gate.txt): **PASS4.01s**
(ordinary1.20s, Size1.23s, Speed1.59s), Go1.26.8. CWD replay —
[инструкция](lizp-functional-complete/replay.txt).
Executable SHA256 `6529a462ccac1311582f89512d8e673fad5c8d83986f75e215ea0c50000d0ec7`
сохранён в [pipeline.test](lizp-functional-complete/pipeline.test).
[Манифест](lizp-functional-complete/manifest.json) связывает exact source/raw MIR,
три ASM/BIN/final MIR/ABI packets, 24 CALL drivers, counts/trace/stack и executable.

## Отдельная обнаруженная граница erased callback

Неизменённый original проходит. Отдельная **синтетическая** диагностическая
программа передаёт `word(u16)->u16` в `apply(ptr,u8)->u8`, где funcall
предполагает u8→u8. HIR Verify и CompileHIRSteps принимают её.
probe(5) получает raw/finalVM261, nativeA5; word ожидает inputDE/outputHL,
apply передаёт inputA/outputA. Это несовместимый erased ABI, не определённый
source numeric oracle; нельзя объявлять261 желаемым u8 результатом.

[Диагностический source/ASM/BIN/MIR и observations](lizp-functional-callback-gap/manifest.json)
заморожены отдельно. Опциональный diagnostic test лишь показывает admission
и фактическое расхождение, его PASS означает «аудит завершился», не корректность
программы. Ширины callback должны согласовываться через propagated callable
contract либо отдельный adapter; запрещать все word callbacks оснований нет.
Verifier fix в этот пакет не включён. Не заявляется полнота всех closure/indirect
signatures, только исходные compatible u8 callbacks данного original source.
