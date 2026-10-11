# Pascal bubble sort: автоматический выбор и отложенные массивы

Полный исходный `examples/pascal/bubble_sort.pas` сохранён без изменения алгоритма и функций печати. Freeze **9baf339e** записан до поиска: три обучающих массива и восемь новых отложенных массивов. Исторический InitArray и диагностический вход исключены. SHA исходника `f461d9d23e9b78c841999d3867db1d1d028353b80ce94354d7a1b89c70280732`.

| Цель | Выбранный режим | Обучение, T | Отложенные входы, T | Полный BIN |
|---|---|---:|---:|---:|
| Speed | ContractOpt=false, MIR Grace=true | 66 891 → 66 720 | 164 671 → 164 275 | 488 → 486 B |
| Size | тот же | 66 891 → 66 720 | 164 671 → 164 275 | 488 → 486 B |

Выигрыш на heldout **396 T, 0,2405%**; размер меньше на 2 байта. Все четыре кандидата собраны и измерены на каждом обучающем массиве. Reusable `MeasureCompilerModeWorkloads` суммирует такты только при совпадении исходника/HIR/ABI/контекста, Options, ASM/BIN, полного MIR и allocation. Размер считается один раз для одного исполняемого образа. При равенстве выбирается первый кандидат; любой отказ отменяет весь выбор. Exact выбранный BIN повторён на всех трёх обучающих состояниях, затем без нового поиска на восьми heldout. SHA победителя `ad33cb3741e279aa6c53f6daa55eea627e512025f9b32912155f42a871d5fba5`.

Независимый оракул проверяет unsigned byte сортировку и перестановку, полный ARR, I=9, SWAPPED=0, J=0 и TEMP последнего обмена по исходному порядку repeat/for. Raw VM, native состояние и упорядоченные наблюдения байтов совпадают. Native стенд проверяет сохранность кода, допустимые записи всей памяти, исходный PC возврата и SP.

Для допуска Pascal компилятор теперь записывает single-unit SHA и точные позиции ключевых слов while/repeat/for. USES и include/link directives, которые frontend не разрешает, не могут объявлять полный source manifest. Допускаются только канонические встроенные ConOut/WriteCrLf/WriteStr/WriteU8 с точным HIR и ABI, совместно недостижимые в исходном и оптимизированном графах. Произвольный ASM запрещён. Негативные тесты меняют template/clobbers/inputs/ABI, добавляют ASM и function-address escape, удаляют вызов из кандидата; всё отвергается. Переход SORT→ConOut в actual ASM остановлен до выполнения BDOS.

По дороге исправлены самостоятельные ошибки: RenumberRegs забывал ASM Ins/Outs (9df07d04), fresh-factory checker считал совпадение адресов пустого BreakStmt общим изменяемым объектом (01c77e83). Admission/origins: 812fbaca; aggregate API: e25e103c; retained HIR: c7b84000; full MIR/allocation binding: e956c6c4.

Authoritative пакет: `bubble-sort-heldout/frozen-native/`; инструкция и старые диагностические контексты: `bubble-sort-heldout/PROVENANCE.md`. Frozen test executable SHA `5b71dce5a801c3e71f396c2b9b42ab67f9e51b3b280cf76ccefc9aceeab495ed`; saved executable отдельно включён в манифест. Terminal frozen suite PASS, prospective test ~10 s; полный Pascal/MIR/HIR PASS 0,019/21,864/24,392 s. Это bounded SORT workload, **не** выполнение main/BDOS и **не** runnable CP/M .COM: measured image ORG 0x8000. Польза небольшая, но получена автоматически на третьем самостоятельном примере с заранее замороженными heldout.
