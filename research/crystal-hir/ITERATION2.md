# Итерация 2: Crystal macro → Nanz meta VM → typed HIR

Рабочий прототип соединяет нативные Crystal macros с существующей Nanz
мета-системой. Генератор написан на Nanz и исполняется при компиляции на MIR2 VM.
Сгенерированные функции проходят обычный HIR/MIR2/Z80 pipeline.

## Пример

В [meta_showcase.cr](meta_showcase.cr):

```crystal
macro meta_repeat(value, count, delta)
  minz_meta_repeat({{value}}, {{count}}, {{delta}})
end

def three_iterations(x : UInt8)
  meta_repeat(x, 3_u8, 7_u8)
end
```

`minz_meta_repeat` — явный типизированный маркер. Его обычная Crystal-реализация
используется только как нативный эталон; в target она не экспортируется.
После native semantic адаптер проверяет типы и literal count/delta, затем
вызывает отдельный `crystal-meta-bridge` через `Process.run` без shell.

Путь исполнения:

```text
Crystal macro expansion + native type checking
 → minz_meta_repeat(value, 3, 7)
 → crystal-meta-bridge repeat 3 7
 → Nanz fun @repeat, исполненная на MIR2 VM
 → emit_hir: 3 типизированных сложения в отдельной helper-функции
 → HIR verification → Lanz → HIR verification
 → MIR2 → Z80
```

Результат — `meta_repeat_u8_3_7(value:u8)->u8`. Сложения оборачиваются по
разрядности UInt8, как `&+` в нативном эталоне. Повторные обращения к тому же
count/delta используют одну сгенерированную функцию в пределах одного экспорта.

## Настоящая Nanz мета-функция

Исходник — `minzc/cmd/crystal-meta-bridge/repeat.nanz`, он встроен в Go-адаптер
через `go:embed`. Мета-функция использует существующие `hir_caller_ref`,
`hir_ref`, `hir_block`, `hir_var`, `hir_binary`, `hir_assign`, `emit_hir`.
Количество сложений определяет цикл самой мета-функции на VM.

Go не реализует алгоритм генератора: он проверяет аргументы, создаёт небольшую
Nanz функцию-оболочку, вызывает публичный `nanz.Parse`, проверяет сигнатуру и
результат, сериализует HIR через Lanz. Реальное исполнение `fun @repeat`
происходит через существующий parser/meta-runtime, без копии его реализации.

Это **проверка стыковки**, пока не общий публичный `MetaContext` API. Мета-функция
видит свой synthetic caller с параметром `value`, а не произвольный Crystal AST
или все bindings исходного caller. Вынести общий API из Nanz parser можно следующим
отдельным изменением, сохранив проверенную семантику.

Версия межпроцессного протокола — `minz-meta-hir-v1`. JSON receipt содержит target
`z80`, имя функции, count/delta, число сгенерированных сложений, SHA256 реального
Nanz generator source, HIR и Lanz. Вывод программы не исполняется как shell-код.

## Что проверено

```sh
python3 research/crystal-hir/verify_meta.py
```

Runner собирает Go bridge, semantic Crystal exporter и HIR consumer. Он задаёт
`MINZ_META_BRIDGE` абсолютным путём к своему собранному bridge. Зависимости те же,
что в итерации 1; тяжёлые операции выполняются только локально.

- **7 Crystal-функций × все 256 значений UInt8 = 1792 случая на каждый engine**:
  native Crystal reference, raw MIR2, optimized MIR2, собранный Z80 в эмуляторе.
- **5 Nanz meta specializations**, суммарно 44 сложения в HIR до оптимизации:
  count=0/1/3/8/32, включая delta=255 и переполнение.
- Повторные macro-вызовы одной specialization используют один helper.
- Имя caller-параметра `meta_result` не конфликтует с локальной переменной
  генератора того же имени; проверено на всех байтах.
- **8 отрицательных контролей**: dynamic count, count выше лимита, отсутствующий
  bridge, четыре нарушения диапазонов producer и некорректная числовая строка.
- В целевом Lanz нет `minz_meta_repeat`, `@repeat` и `emit_hir`.

Артефакты и логи: `/tmp/minz-crystal-meta-results`. `manifest.json` сохраняет
receipts генераторов; каждый generated helper отдельно записан в `.hir`.
Копии итоговых manifest/receipt сохранены в `evidence/iteration2-*`.

## Обнаруженный дефект assembler

Сквозные тесты выявили существующий дефект `z80asm`: инструкция до forward EQU
могла использовать старый нулевой адрес, хотя финальная таблица символов уже
показывала правильный адрес.

```asm
ORG 0x8000
CALL alias       ; раньше могло собраться как CALL 0
HALT
alias EQU target
target:
LD A,21
RET
```

Assembler завершал passes при стабильных **размерах инструкций**, не проверяя
изменения **значений символов**. Теперь оба условия должны сойтись. Для forward
EQU chains добавлен ограниченный бюджет passes; при несходимости возвращается
ошибка вместо потенциально неверного бинарника.

Регрессии воспроизводили дефект до фикса. Теперь покрыты forward alias, его
сочетание с JRS relaxation, цепочка из 16 EQU, ADL 24-bit CALL и расходящаяся
система EQU. Полный `go test ./pkg/z80asm` проходит. Мост не обходится перестановкой
функций или отключением оптимизаций — прогоняется исходный сгенерированный код.

## Оставшиеся ограничения

Пока поддержан один generator protocol: UInt8 repeat с count 0..32 и delta
0..255, оба параметра — literal после native semantic. Runtime-значения count
не принимаются как compile-time данные. Общие schema/type introspection,
генерация struct methods и target layout API ещё впереди.

На уровне языка зарезервировано opt-in имя `minz_meta_repeat`; это прототип
binding, не универсальное распознавание произвольной функции по её поведению.
Для другой мета-функции надо добавить явное описание вызова и контракт результата.

Следующая содержательная цель — struct/Pointer с нашим layout и arena, либо
выделение общего typed meta invocation API из parser. GC для обеих задач не нужен.
