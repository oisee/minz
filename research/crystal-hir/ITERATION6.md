# Итерация 6: native Crystal Slice#each → общий HIR foreach

Работает обычный синтаксис Crystal, без нового marker:

```crystal
sum = 0_u8
Slice.new(pointer, length).each do |value|
  next if value == 0_u8
  sum = sum &+ value
  break if sum == 255_u8
end
```

Этот фрагмент иллюстрирует поддержанную форму. Полный исполненный корпус —
[iteration_showcase.cr](iteration_showcase.cr).

Exporter распознаёт native Indexable#each на непосредственно созданном borrowed
Slice.new(ptr,len), сохраняет typed block body и превращает его в wire foreach.
Go importer вычисляет pointer и length один раз в исходном порядке, создаёт
общий `hir.ForEachStmt` и использует существующее HIR→MIR2→Z80 lowering.
Новый Slice object/closure на target не материализуется. Ядро compiler не менялось.
В простом byte loop итоговый backend использовал DJNZ.

## Проверки

```sh
python3 research/crystal-hir/verify_iteration.py
```

**1822 набора на native Crystal, raw MIR2, optimized MIR2 и Z80:**

- 7 byte-сценариев × все 256 seed values = 1792: sum, break с изменением
  следующего элемента через alias, next, nonlocal return, snapshot pointer/length
  при изменении исходных переменных в block, nested loops и empty null-pointer view;
- 20 проверок lengths 0/1/2/255/256, включая 256-byte backing buffer;
- 10 UInt16 seeds для word pointer stride и wrapping значений.

5 отказов: signed length, heap-allocating Slice constructor, valued break,
valued next и materialized/escaped Slice local. Пустой slice не запускает block;
изменение следующего байта во время block видно следующей итерации. Native
reference и target сравнивают наблюдаемые значения/эффекты; полного наблюдения
каждого машинного memory read этот corpus не предоставляет.

Регрессия macro/meta + records также прошла: 768 наборов и 7 отказов, четыре engine.
Artifacts: `/tmp/minz-crystal-iteration-results`; receipt —
`evidence/iteration6-results.json`.

## Границы

- unsigned u8/u16 elements и u8/u16 length;
- непосредственный borrowed constructor, без named arguments;
- каждый block имеет один элементный аргумент; captures допустимы в проверенных
  формах, break/next без значений и nonlocal return сохраняют свой смысл;
- each используется как statement; Slice value/Iterator object, map/select и
  произвольные escaped closures этим шагом не реализованы;
- admission проверяет native filenames для each и constructor; эксперимент
  привязан к установленной stdlib Crystal 1.20.1. Произвольное переопределение
  её внутренних методов не входит в поддержанный профиль;
- backing memory остаётся raw-pointer контрактом. Нет общего bounds/lifetime
  verifier; тестовые buffers живут дольше foreach. Signed lengths намеренно
  отклонены, чтобы не подменять Int32 размер Slice wraparound-счётчиком target.

Это подключает существующую общую итерацию к Crystal, но ещё не импортирует
Crystal lazy Iterator в Nanz gen/yield и не реализует общий Slice API.

Следующий шаг: общий borrowed-view API и подключение generator/pull протокола,
с дальнейшими oracle-проверками demand, aliasing и lifetime.
