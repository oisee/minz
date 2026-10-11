# Корпус: ещё четыре независимых native-оракула

На базе `ff11ba69` проверены четыре actual-source программы из каталога,
относящиеся к разным алгоритмическим областям. До этого именно эти нагрузки
не были покрыты независимыми моделями в нашем corpus catalog.

| Actual source | Нагрузка | Независимый оракул | VM + native Z80 |
| --- | --- | --- | --- |
| examples/nanz/04_lut_popcount.nanz | Все 256 входных байтов | Go bits.OnesCount8 | PASS |
| examples/nanz/12_state_machine.nanz | 4 состояния × 7 событий + alive/speed, 36 calls | Таблица переходов/свойств вне source lowering | PASS |
| examples/lizp/crypto.lizp | 250 boundary calls XOR/Caesar/hash/checksum | Unsigned byte arithmetic modulo256 и XOR | PASS |
| examples/pascal/sieve.pas | Все 101 flags и COUNT | Trial division для нечётных чисел 3..203: 45 простых | PASS |

Для Caesar oracle учитывает фактический typed u8 intermediate: сначала
`(ch-65+shift)&255`, затем `%26`. Это не утверждение о криптографической стойкости
XOR/Caesar. Неформальный mathematical Caesar без u8 wrap был бы другой функцией.
Unsigned byte widths/casts описаны в Nanz Language Book v7 §§3.1–3.4; enum/match
в Ch16; LUT в §2.12. Pascal mapping Boolean→bool, Integer→i16 и общая frontend
цепочка — Ch21/§21.6. Expected formulas независимы от compiler IR.

## Реальная ошибка массива bool

Первый запуск неизменённого Pascal sieve упал на VM: `store out-of-bounds
address 75 (heap size75)`. MIR ArrayTy считала width как `Elem.Width()*Len`:
101 одноразрядный bool занимал 13 байт, хотя каждый indexed load/store обращался
к отдельно адресуемому byte. Это могло перезаписывать соседние native данные.

Исправлена ширина хранения ArrayTy: `ByteWidth(Elem)*8*Len`. Значащая ширина
scalar bool остаётся 1 bit, storage scalar bool — 1 byte. Bit-packed массивы
не вводятся. Регрессия layout покрывает bool/u8/u16/u24 и вложенные bool arrays.
После исправления все 101 byte флага и COUNT=45 совпадают с независимой моделью
на VM и assembled BIN. Исходный алгоритм sieve не изменён.

Для native workload взят **точный computational prefix исходного main** до
первого WriteI16; добавлен только void-return adapter. Console output требует
CP/M BDOS и не является частью этой нагрузки. Native execution обязано достичь
исходного PC возврата 7000 со SP=fff2. VM не получает дополнительный heap,
маскирующий ошибку storage.

Лог sieve-after-layout фиксирует промежуточную ошибку тестового adapter: он
не распознал WriteI16 и вошёл в BDOS. Граница адаптера уточнена явно; это не
compiler miscompile. Сохранены оба before/after результата.

## Живой A после NOT

Новая регрессия содержит настоящее live значение A=37, употребляемое **после**
NOT: return `37 + u8(~input)`. По 18 входам bool/u8/u16/u24/u32 результат сравнен
с независимой формулой и VM; stack SP=fff0 остаётся balanced. Таким образом
проверяется не просто return комплемента, а сохранение unrelated accumulator
через complement и narrow cast. Все проверки PASS.

## Что означает каталог

Обновлённый catalog.json содержит все **193 exact paths** с SHA исходников,
entries/externs/inline ASM contracts и выбранными workload/model. Здесь четыре
строки получили bounded independent VM+Z80 coverage после actual runs.

| Категория | Файлов |
| --- | ---: |
| Covered bounded workloads | 11 |
| Ещё выбрать независимый oracle | 122 |
| Inline ASM target/runtime audit | 32 |
| Extern runtime audit | 22 |
| Agon eZ80/MOS/VDP target | 5 |
| ZVM host memory model | 1 |

Это не statement «все193 nativePASS» и не exhaustive map уже существующих tests.
Предыдущие семь покрытых строк сохраняют свои доказательства/ограничения.
Sieve output outside workload отражён явно. Platform и host exceptions остаются;
программы с ASM также могут иметь pure subroutine, пригодную для следующего шага.

## Верификация и локальные артефакты

Focused четыре workloads PASS; 18 live-A regressions + layout PASS;
полные `pkg/mir2`, `pkg/hir`, `pkg/pascal` PASS. Root отдельно проверяет весь
showcase compile/assemble после integration. Файл pipeline.go не изменялся.

docs/grace-showcase-algorithm-oracles/ содержит representative sieve ASM/BIN,
catalog и логи before/after с SHA256 manifest. ASM/BIN — реальные результаты
native memory test; попcount/FSM/crypto используют production native assertion
harness с ABI из actual allocation.

Повтор из minzc:
`go test ./pkg/pipeline -run '^TestShowcasePopcount|^TestShowcaseStateMachine|^TestShowcaseLizpCrypto|^TestShowcasePascalSieve'`;
`go test ./pkg/mir2 -run '^TestArrayStorage|^TestZ80NotPreserves'`.
