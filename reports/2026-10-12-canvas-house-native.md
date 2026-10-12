# Исходный canvas_house: VM, native и внешний ABI

Неизменённый `examples/nanz/canvas_house.nanz` проверен целиком: `draw_scene`, `main`, `Circle.draw`, `Box.draw`, `Box.outline`. До исполнения зафиксированы исходник, actual `mir2/canvas.go` API и 13 ожидаемых вызовов сцены. `main` добавляет `canvas_save` с точным NUL-terminated `build/canvas_house.png`: исходник уже содержит `c"..."`, исправления строк не понадобилось.

Два реальных native сбоя сохранены отдельно. На базе d0b667e1 второй и последующие word-параметры extern получали один DE; `canvas_init(256,192,0)` становился `(256,0,0)`, `canvas_line(55,70,90,40,15)` — `(55,15,15,15,15)`. После восстановления ABI обнаружился другой сбой: width=256 в struct записывался как 0, потому что word-store считал named spill байтом и записывал high=0. Raw и final VM давали правильные аргументы в обоих случаях. Before-пакеты имеют отдельные SHA-манифесты и не заменены новыми результатами.

`065217c4` вводит один collision-free Z80 extern formal plan, общий для public allocation и call emission. Пары/их половинки не пересекаются; пять words получают HL/DE/BC/IX/IY, избыток — width-correct именованные slots callee. Explicit HIR annotations сохраняются в `Param.ClassHard`; ручные ClassRegX тоже остаются жёсткими. Конфликт явных pins отклоняется; default preferences разрешено перемещать. Этот ABI со static overflow slots предназначен для синхронных внешних вызовов, не обещает reentrant/interrupt-safe stack arguments. Source API canvas и filesystem не аннотировались вручную. Проверены 2ptr, 5word, 6word со spill, mixed byte/word alias и pin conflicts. Forged ClassHard отклоняется существующим memory-host identity guard; JSON и MIR dump содержат pin.

`698e84db` исправляет только OpStore16: spilled source с типом word сохраняет старший байт; настоящий u8 по-прежнему zero-extends. Независимые 78 native cases: u8/u16/i16, HL/DE/BC/IX/IY/spilled pointer, значения 0/1/255/256/65535 по ширине. Проверены точные два записанных байта, остальная RAM, PC и SP; memory order/fusion не менялись.

После исправлений ordinary / Grace Size / Grace Speed дают одинаковую сцену: raw/final VM и связанный native совпадают с независимыми аргументами, порядком вызовов и framebuffer 256×192. Модель отдельно реализует half-open fill rectangles, inclusive outline endpoints, Bresenham и midpoint circle в области зафиксированного Canvas API. Все 49152 индексных пикселя совпадают. PNG от production Canvas экспортируется в test artifact path; соответствие filename в исходном `canvas_save` проверено отдельно, настоящий файл по исходному пути не пишется.

Native запускает неизменённый compiled caller; только пустые внешние linker slots снабжены test-owned RET/callback. Callback читает actual allocated formal homes, включая named spill, возвращает u16 через HL и намеренно меняет остальные основные scratch-регистры/F. Сцена, locals и CALL/return выдерживают это. `draw_scene` и main возвращаются на PC7003/SPf000; native RAM проверена целиком с точными исключениями stack и emitted spill slots. Это external test-owned device, не новый производственный graphics runtime и не реальный аппаратный framebuffer.

| Режим | Linked BIN | draw_scene T | main T |
|---|---:|---:|---:|
| Ordinary | 1918 B | 17001 | 17038 |
| Grace Size | 1918 B | 17001 | 17038 |
| Grace Speed | 1918 B | 17001 | 17038 |

Такты учитывают исполняемый Z80 caller; native host drawing callback не эмулирует стоимость устройства. Прямые методы дополнительно проверены на шести заранее ограниченных наборах полей (200/40/25/14;256/192/0/255;0/96/256/96/4;255/191/1/1/255;60/70/60/50/2;256/192/0/0/65535), по трём режимам, включая возвращаемое host значение 0x3456. Это не полный Cartesian всех u16 struct полей.

Focused scene/methods PASS 0.577 s; external ABI PASS 0.013 s; identity negatives PASS 0.008 s; word stores PASS 0.462 s. Полный MIR2 PASS 82.226 s на ABI+store code до cosmetic dump follow-up. Новый retained pipeline executable повторил scene/method groups PASS. Полные pipeline/CLI gates здесь не запускались. Методы и исходные entries закрыты указанным bounded proof; rotozoomer не относится к этому пакету.

Commits: prospective freeze `cf119480`, original native counterexample `94e6d5fd`, ABI `065217c4`, second before `7a9cccaf`, store `698e84db`, pin dump/JSON `790e0357`, model/method proof `399a65a0`. Пакет `reports/canvas-house-native/final/manifest.json` включает исходники, ASM/BIN, raw/final MIR, allocation, events, pixels/PNG, регистры и логи. Share: `/mnt/safe/minz/grace/2026-10-12/agent-artifacts/canvas-house-native/`; executable SHA/path в final/executable.json. Исторические before manifest сохранены отдельно.
