# Shape-таблицы для байтового доступа к памяти

Рабочий допуск checked-table больше не ограничивается чистой арифметикой.
Один leaf-блок может содержать канонические 16-битные byte pointers, PtrAdd
с 16-битным integer offset, байтовые LOAD/STORE и void-return. Порядок операций
сохраняется. Word memory accesses, globals/calls, более широкие или другие
typed pointers и произвольный CFG не добавлены.

Это перенос уже существующих VIR-примитивов в проверяемый production-кандидат.
Исходный MIR/ABI остаётся обязательной привязкой; при Emit исходные операции
строятся заново и сравниваются с предложением. Достаточно переименовать LOAD
в локально допустимый CONST и пересчитать таблицу — такой кандидат всё равно
отклоняется. Адрес и значение STORE проверяются раздельно по типам; STORE
не создаёт SSA-значения. Флаги и scratch продолжают проверяться прежним допуском.

## Настоящая аппаратная библиотека

```nanz
import hw.sprite
fun sprite_test() {
    @sprite() {
        destination "linear" 24576 6 1 6
        region 0 0 6 1
        row "fff00faa5500" "00050340aaff"
        via "pointer"
    }
}
```

Одна строка содержит прозрачный, четыре частично masked и один opaque byte.
Структурный генератор библиотеки остаётся прежним. Из его текущего MIR
получены фрагменты назначений: pointer в HL, загруженный/вычисляемый byte в A.
Последний STORE сохраняется вместе с void RET, остальные фрагменты заканчиваются
на STORE. `ComposeCheckedAutomatically` находит разбиение и заново допускает
полное назначение. Поиск подготовки не вызывает Z3.

| Конфигурация | Байты baseline / candidate / selected | Такты baseline / candidate / selected |
|---|---:|---:|
| greedy, Grace off | 46 / 43 / 43 | 198 / 186 / 186 |
| greedy, HIR+MIR Grace | 46 / 43 / 43 | 198 / 186 / 186 |
| PBQP контрактов, Grace off | 46 / 43 / 43 | 198 / 186 / 186 |
| PBQP контрактов, HIR+MIR Grace | 46 / 43 / 43 | 198 / 186 / 186 |

Везде OptSize=true, ContractOpt=true, AssertMode=none. Selected — обычный
checked-local-table selector; он берёт только меньший полный BIN. Выигрыш
−3 байта и −12 T (−6,1%) относится к этой функции и этому descriptor.
Нет дополнительного выигрыша от PFCCO или Grace в этой матрице.

Для каждого masked byte исчезает пересылка через C:

```asm
; baseline                   ; checked candidate
LD HL, 24577                 LD HL, 24577
LD C, (HL)                   LD A, (HL)
LD A, C                      AND 240
AND 240                      OR 5
OR 5                         LD (HL), A
LD (HL), A
```

Последний opaque store в кандидате дороже immediate-store baseline; его
проигрыш уже входит в полные числа таблицы. Это назначение регистров по
форме операций, не таблица готовых пикселей и не новое распознавание спрайта.

## Проверка и границы

Baseline, raw candidate и selected исполняются на 256 полных фонах памяти
и начальных flags в каждом из четырёх режимов: 3072 native-случая. Независимый
оракул применяет `(old & keepMask) | graphics`, сверяет все 65536 байт,
включая код и область стека, и исходные return PC/SP. Исходный MIR VM также
сверяется с этим оракулом на всех фонах. Сам sprite не требует scratch writes.

Отдельный native read-byte проверяет все 65536 исходных pointer-значений с
runtime offset=1, включая wrap к адресу 0: возвращённый byte, вычисленный
адрес HL, неизменную RAM, BC/DE/IX/IY и PC/SP. Неверные типы адреса/offset,
word access, defined STORE и pointer constant вне 0..65535 отклоняются.

Новые typed/native тесты: PASS (VIR 0,021 с, sprite 5,298 с после последнего
усиления и сохранения BIN). Полный VIR: PASS 22,722 с. Checked-table,
структурный sprite и pointer sprite pipeline: PASS 25,358 с. Solver-dependent
проверки без VIR_TEST_Z3 пропущены; они не включены в заявленное native покрытие.
Native sprite внесён в fast inventory, полный deep-релизный корпус не повторён.

Аудит неизменённых источников до/после допуска показал: sprite_test теперь
подготавливается, в typed tokenizer — str_len. Его ordinary ASM уже равен
`ADD HL,DE; LD A,(HL); RET`; ускорение tokenizer этим этапом не заявляется.
Подключение таблиц к measured CLI и перенос фрагментов на другие descriptors
остаются следующими шагами. Обычные defaults не меняются.

Артефакты: `/mnt/safe/minz/grace/2026-10-13/checked-local-byte-memory/manifest.json`.
В отчёте сохранены исходник, fragment pool, полная таблица и ASM всех четырёх
режимов; на шаре также лежат точные BIN, до/после аудит, исходники и логи.

Аудит допуска можно повторить из корня checkout:

```sh
go -C minzc run ../reports/checked-local-byte-memory/audit.go /path/to/minz /tmp/byte-memory-audit
go -C minzc test ./pkg/vir -run '^TestCheckedByteMemory' -count=1 -v
go -C minzc test ./pkg/pipeline -run '^TestCheckedByteMemoryComposedSpriteNative$' -count=1 -v
```
