# Третий потребитель структурного emit: masked sprite

13 октября 2026. Pointer-ветка `stdlib/hw/sprite.nanz` теперь строит типизированный
HIR для `(old & mask) | graphic`. В `hir_binary` добавлены &, | и ^ для
совпадающих u8/u16; bool, указатели и смешанные типы отклоняются.

Декларация DSL сохраняется: destination/region/row/via. Compile-time Nanz
по-прежнему проверяет layout, границы и hex-строки. Во время построения кода
используются `hir_var`, `hir_ref`, `hir_load`, `hir_binary`, `hir_store` и
`emit_hir`. Нет форматирования и повторного разбора pointer-операций.
Прозрачные байты пропускаются; непрозрачные пишутся без чтения.

```nanz
let pixel: u16 = hir_ref(c"sprite_ptr", c"ptr")
hir_append(region, hir_var(c"sprite_ptr", c"ptr",
    hir_int(u16(address + col), c"ptr")))
let old: u16 = hir_load(pixel)
let kept: u16 = hir_binary(c"&", old, hir_int(u16(mask), c"u8"))
let value: u16 = hir_binary(c"|", kept, hir_int(u16(graphic), c"u8"))
hir_append(region, hir_store(pixel, value))
emit_hir(region)
```

Сохранённый текстовый генератор служит baseline. В трёх примерах и трёх
режимах ordinary/Grace Speed/Grace Size ASM совпал побайтно:

| Пример | Байты до / после | Такты до / после |
|---|---|---|
| Непрозрачные 4 байта | 21 / 21 | 90 / 90 |
| Смешанные маски | 46 / 46 | 198 / 198 |
| ZX rows 7→8 | 64 / 64 | 273 / 273 |

Первая попытка поместить адрес непосредственно в load/store дала худший
ASM для смешанных масок. Явная переменная указателя восстановила исходное
построение. Это ограничение текущей lowering/allocator формы; миграция
повышает надёжность генератора, дополнительного ускорения не заявляем.

Полный `TestHardwareSprite.*` набор прошёл за 9,467 с: pointer и compiled
варианты, независимые модели памяти на VM/Z80, PC/SP, маски, границы ZX,
цепочки и full-machine сценарии. Исходники, MIR, ASM/BIN и результаты
сохранены в native-пакете на шаре.

Быстрый pipeline: 352 из 699 тестов, четыре процесса, PASS. Этот checkpoint
включает main с PR #100 и новую sprite-миграцию; автоматическая композиция
и CLI пока проверяются в отдельных PR. Глубокий релизный корпус не повторён.
