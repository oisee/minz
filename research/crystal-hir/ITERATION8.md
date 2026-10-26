# Итерация 8: native Crystal yield → общий HIR generator/pull

Обычный Crystal-код теперь использует существующий HIR generator lowering:

```crystal
def byte_stream(src : Pointer(UInt8), n : UInt8, trace : Pointer(UInt8), &)
  while n != 0_u8
    yield stream_peek(src, trace)
    src = src + 1_u16
    n = n &- 1_u8
  end
  nil
end

byte_stream(pointer, count, trace) do |value|
  sum = sum &+ value
  break if sum == 255_u8
end
```

Это иллюстрация поддержанной формы. Полный выполненный корпус —
[generator_showcase.cr](generator_showcase.cr).

## Мост без нового generator runtime

Exporter получает scalar element type из типизированных Yield expressions и
сохраняет тело producer как wire function с elem. Go importer строит обычные
`hir.Func{IsGen:true}`, `YieldStmt` и `GenForStmt`. Общий `ExpandGenerators`
выводит concrete frame и next-функцию, как для Nanz. Target получает обычные
loads/stores, labels, branches и calls; нового MIR2 coroutine opcode нет.

Crystal call с block здесь потребляет producer немедленно. Native функции с
односторонним yield и игнорируемым результатом block становятся pull producer;
consumer break/next/nonlocal return управляет запросами того же frame. Это ещё
не импорт стандартных Crystal Iterator objects и не first-class lazy API.

Native semantic frontend выдал 8 producer specializations для 8 consumer sites:
несколько byte_stream с разными block contexts, positive filter, RLE и бесконечный
counter. Нынешний record path не объединяет их автоматически по телам.

## Найдено и исправлено в общем HIR

**94cc4337b:** explicit completion return в yield-free structured region мог
пройти в bool next-функцию как valueless return, без DONE. Nanz reproducer до
исправления потреблял три элемента вместо одного. Теперь такие return по всем
структурным дочерним блокам выставляют DONE и возвращают false, оставляя out
неизменным. Отдельно проверен повторный pull после раннего return при изменившемся
внешнем состоянии. Это исправление полезно и без Crystal.

**2d7fa7904:** GenForStmt может поручить frame layout общей expansion, но dump
раньше разыменовывал отсутствующий frontend frame. Dump теперь отображает
`<derived>`. Валидный frontend HIR с таким frame проверен до выполнения и далее
на raw/optimized MIR2 и Z80.

Regression gates: новые `TestGeneratorExplicitCompletionDone`,
`TestGeneratorForDerivedFrame`, а также существующие Nanz
`TestG1GenYield`, `TestG1GenYieldNegative`, `TestG1BoolComparisonYield` прошли.

## Проверки Crystal

```sh
python3 research/crystal-hir/verify_generators.py
```

**2048 наборов = 8 consumers × все 256 byte seeds**, каждый исполняется native
Crystal, raw MIR2, optimized MIR2 и собранным Z80:

- полный drain, including producer completion effects;
- break с записью в следующий исходный элемент через alias;
- next с продолжением producer после yield;
- nonlocal return из consumer;
- filter + ручной take(2): читаются только необходимые source items;
- RLE с вложенными while, zero-length run и 6 instrumented source reads;
- конечное потребление двух значений бесконечного producer;
- empty/null-pointer source без чтения данных.

UInt16 digest сохраняет sum и независимые поля reads/before-yield/after-yield/
completion. Например, для filter [0,0,7,8,9] с take(2) read count равен 4,
а post-yield effect второго элемента не выполняется после break.

5 отказов: feedback через результат yield, non-Nil completion value,
несколько значений yield, valued break и reserved frame parameter tag.

Artifact receipt/layout: `evidence/iteration8-*`; полные временные файлы:
`/tmp/minz-crystal-generator-results`. Сохраняются pre-expansion HIR, expanded
HIR и raw MIR2 даже при неуспешном runtime assert.

Target-проход занял около 361 s. Sampling текущего процесса подтвердил, что
существенное время уходит на повторную assembly всего модуля для отдельных
Z80 assertions. Это время harness, не benchmark скорости generated program.

## Границы и следующий шаг

Producer пока top-level, block имеет один scalar аргумент. Source method должен
возвращать Nil либо никогда не возвращаться. Yield используется как statement;
feedback/send, aggregate locals/outputs, nested generator consumption в producer
и yield внутри foreach отклоняются. Source pointer parameters остаются raw
borrow контрактом; проверенные backing arrays живут дольше немедленного consumer.
Ensure/exception cleanup и произвольные stdlib Iterator subclasses не реализованы.

Frame layout пока консервативен: ANF temporaries могут попадать в frame даже
когда их storage можно было бы сократить. Не следует считать нынешний layout
минимальным. Общая библиотека buffer/view операций и улучшение lifetime/effect
контрактов остаются следующими шагами; цель не завершена одним yield bridge.
