# Widemath observer после caller-owned struct ABI

Основа `bbe17da0`, исходный `examples/nanz/widemath.nanz` не изменён; SHA проверяется историческим `reports/widemath-native/freeze.json`. Все 26 исходных callable и 31 assertion остаются в полном модуле. Здесь изменён только test-owned harness, не production compiler.

До исправления: воспроизведён точный FAIL 0.055 s (`before.txt`). Первый `newtype_area(0,0)` показывал три события вместо четырёх. Реальная ASM записывает первую локальную W8 через `LD (DE),A`; декодер видел только `(HL)` и терял эту запись. В новом local-frame emitter две независимые constructor stores расписаны в обратном порядке, но operator reads сохраняют исходный порядок a.v затем b.v. Полный исходный RAM-эффект корректен.

Исправление декодирует byte accesses через BC/DE/HL/IX/IY. Instruction fetch, word spill и стековые PUSH/POP не становятся source-data событиями. Constructor oracle требует ровно два разных локальных store, затем два исходных ordered reads с правильными значениями и адресами реально записанных объектов; физический порядок независимых частных store не фиксируется.

`op_add/op_sub` теперь имеют настоящий appended hidden result pointer. Harness передаёт защищённый отдельный byte buffer 6200 через фактическое allocation location, исходные указатели 6000/6010 сохраняются. VM binder передаёт caller-owned storage автоматически. Оракул строго проверяет input read a затем b, private result store/load snapshot с одним адресом, отдельный final public output store. Native возвращённый указатель должен быть ровно 6200, значение — независимый u8 modular arithmetic model. Полные 64 KiB RAM сравниваются: дополнительно разрешён только ожидаемый byte6200; guards61ff/6201, исходные input bytes, code, остальные RAM и SP contract остаются защищены. Прежнее маскирование только фактического stack extent и объявленных spill homes сохранено.

Негативные проверки отвергают missing store, reversed public input reads, wrong constructor value, aliased W8 objects, missing/wrong snapshot, output/private/input alias. Decoder проверяется независимыми BC/DE/indexed read/write инструкциями и отрицательными stack/fetch controls.

Итог `after-final.txt`: PASS 1.931 s, 11 947 исходных workload calls на режим × ordinary/Grace Size/Grace Speed = 35 841 actual assembled native calls; для каждой нагрузки также raw и final VM. Значения и multi-return model, ordered W8 effects, caller buffer byte, полный RAM/code/SP проходят. Это ограниченный исходный Widemath workload, не доказательство всех integer inputs (MINZ_WIDEMATH_EXHAUSTIVE не включён).

Before/after ASM/BIN/raw-final MIR/allocation/source/calls сохранены отдельно без изменения исторического freeze. Их SHA256 находятся в manifest.json. В scope не добавлены новые production API или методы компиляции.
