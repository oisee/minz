# Iterator-стадии range → forEach

Продолжение аудита HIR/MIR, 10 октября 2026.

## Что найдено и исправлено

Цепочка `range(0..n).filter(keep).map(mapper).forEach(callback)` имела два пробела: распознавание двухаргументного forEach проверяло только непосредственный range, а обработчик стадий range-forEach поддерживал только map. Проба на исходнике завершалась panic при lowering RangeSourceExpr. Теперь источник распаковывается до выбора memory/range; фильтр оборачивает последующие стадии и callback условием. Отрицательный предикат пропускает весь оставшийся суффикс. Continue не используется: он обходил бы неявный decrement DJNZ.

На callbacks с captures нашлись ещё два сбоя общего пути. Удаление мёртвых block params не обновляло body/exit edges DJNZ; исправлена индексация с учётом неявного первого параметра-счётчика, и он сохраняется даже без явных reads. Исправление общее для обычного MIR и Grace. В Z80 сохранение A выбирало первый по ID выделенный vreg, который мог давно умереть; теперь выбираются действительно живые в месте операции значения. Это сохраняет сумму при вычислении промежуточного map. Range-forEach использует обычные классы loop expressions; fold сохраняет прежний режим accumulator.

```nanz
fun test(n:u8)->u8 {
    var sum:u8=0
    let limit:u8=2
    let delta:u8=1
    let keep=|x:u8| x>limit
    let mapper=|x:u8| x+delta
    range(0..n).filter(keep).map(mapper).forEach(|x:u8| {sum=sum+x})
    return sum
}
```

При n=5 countdown выдаёт 5,4,3,2,1. Фильтр оставляет 5,4,3; map даёт 6,5,4; сумма 15. Capture-метаданные сами не гарантируют корректное машинное исполнение — этот пример потребовал проверки обоих backend.

## Проверки

48 сквозных вариантов: Grace off/on, n=0/1/5/255; filter→map, map→filter, два фильтра с побочным эффектом, всегда ложный фильтр, захваченные limit/delta и оба направления записи range. Независимый oracle моделирует countdown, порядок стадий, u8 wrap и число эффектов. Сырой MIR2 проверен verifier и VM; production Z80 проверен эмулятором. Отдельный MIR regression проверяет удаление параметров обоих рёбер DJNZ обычным и Grace-проходом.

Пакеты HIR/MIR2/Nanz/pipeline/Lanz проходят с исключением отдельно учитываемого showcase. Showcase отдельно: прежние 32 падения, новых нет. Это исправление корректности, не новый выигрыш Grace; проценты ускорения не заявляются.

[Воспроизведение до исправления](grace-iterator-artifacts/before.txt), [48 вариантов VM/Z80 и T-states/bytes](grace-iterator-artifacts/dual-vm.txt), [пакеты](grace-iterator-artifacts/packages.txt).

## Границы

Здесь проверены сохраняющие u8 тип stages и single-return captured expressions. Общая смена типа map, callbacks с ранним return, сложные captured callbacks с локальными statements и полноценный environment ABI ещё требуют отдельного аудита. Это не заявление о корректности всех iterator-форм. PGO ещё не реализован; его основание — идентичность событий и инвалидирование профиля — следующий отдельный этап.
