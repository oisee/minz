# Итерация 7: общий borrowed-view descriptor и локальный Crystal Slice

Теперь работает материализованный локальный Slice:

```crystal
view = Slice.new(pointer, 3_u8)
copy = view
view.each do |value|
  sum = sum &+ value
end
length = view.size.to_u16
pointer_again = copy.to_unsafe
```

Полный проверенный исходник — [borrowed_view_showcase.cr](borrowed_view_showcase.cr).

## Обогащение общего HIR

`hir.BorrowedViewType(name, elem)` задаёт канонический addressable descriptor:
`data: ^Elem`, `len: u16`. Это обычный StructTy и обычные field loads/stores.
На текущем Z80 layout он занимает 4 байта (offsets 0 и 2), native Crystal Slice
на проверенном host — 16. Host read_only/size representation не переносится.

MIR2 SliceTy уже описывает product ABI; новый helper не притворяется, что этот
product — адрес обычного struct. BorrowedViewType явно выбирает struct storage,
доступное разным frontend. Lowerer, MIR2 opcodes и Z80 codegen не менялись.

Wire declaration имеет kind=borrowed_view. Go importer проверяет canonical
field names и типы через общий helper. Скопированный descriptor получает свой
pointer/length, а backing memory намеренно остаётся общей. Дескриптор не владеет
ею и не продлевает её lifetime.

## Проверка с Nanz уже есть

`TestBorrowedViewHIRToNanzNative` создаёт descriptor через HIR API, а читает
его отдельно разобранная Nanz-функция с `struct ByteView { data:^u8,len:u16 }`.
Все 256 byte seeds проходят raw MIR2, optimized MIR2 и собранный Z80.
Проверяется совместимость field ABI и настоящий buffer read, а не равенство
списка полей самому себе. Удобный встроенный Nanz Slice syntax этим не добавлен.

## Живой descriptor внутри each

Native oracle обнаружил существенную семантику Crystal:

```crystal
view = Slice.new(first, 3_u8)
view.each do |value|
  view = Slice.new(second, 1_u8)
end
```

Этот пример делает только одну итерацию: стандартный Indexable#each_index
перечитывает size, а unsafe_fetch берёт текущий data. Snapshot обоих полей
на входе был бы ошибкой.

Поэтому неизменяемая в block локальная view использует общий ForEachStmt,
а при прямом rebind view в block AST exporter выбирает обычный HIR while с
живыми descriptor fields. Скрытый индекс увеличивается до block, так что
next корректно переходит к следующей проверке, break и return сохраняют
обычные loop/function targets. При admitted max length 65535 индекс не должен
оборачиваться: значение 65535 уже не проходит условие index < len.

Projection `.size.to_u16` и `.to_unsafe` на inline constructor вычисляет оба
аргумента слева направо, включая побочные эффекты отброшенного поля. Это
проверено через ProjectionState: data/length последовательно меняют hits.

## Доказательства

```sh
python3 research/crystal-hir/verify_views.py
GOCACHE=/tmp/minz-crystal-go-cache go -C minzc test ./pkg/hir \
  -run '^TestBorrowedViewHIRToNanzNative$' -count=1
```

**1566 наборов на native Crystal, raw MIR2, optimized MIR2 и Z80:**
6 функций × 256 byte seeds, ещё 20 wide-length и 10 UInt16 cases. Покрыты
локальная view, copy/rebind и общая backing memory, shrinking/growing descriptor
во время each, next/break, word stride, длины 0/1/2/255/256 и порядок эффектов
projection. Отдельно — 7 отказов (5 source forms и 2 искажения wire schema).
Receipt/layout находятся в `evidence/iteration7-*`, временные полные artifacts —
`/tmp/minz-crystal-view-results`.

## Границы

- Только borrowed constructors с u8/u16 elements и u8/u16 length;
  heap allocation и named constructor arguments отклоняются.
- Нет aggregate argument/return ABI в Crystal bridge: view остаётся локальной.
- Native Slice [] с bounds checks пока отклоняется; to_unsafe имеет обычный
  raw-memory контракт. Source sizeof/size Int32 не подменяется молча: поддержан
  точный explicit `.size.to_u16` для admitted length domain.
- Это не borrow checker и не полный lifetime verifier. В корпусе все backing
  arrays живут дольше views; произвольные raw pointers требуют контракта caller.
- Alias analysis для mutable descriptor admission опирается на ограниченный
  профиль: прямые rebind видны в block AST; скрытая передача адреса descriptor
  и произвольные stdlib overrides не входят в поддержанный frontend.

Следующая цель — использовать уже существующий gen/yield pull protocol через
Crystal bridge и развивать общий buffer/view API, сохраняя семантику каждого
исходного языка вместо механического одинакового lowering.
