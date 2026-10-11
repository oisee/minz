# Manhattan: physical stack traffic и object effects

На checkpoint `50fb0ede` воспроизведён точный исторический FAIL `TestOriginalManhattanNativeModel`: первый default call `(0,0,0,0)` возвращает правильный0 и object bytes0, но profiler на будущем q.x адресе `efea` считает3 writes/3 reads вместо ожидаемых2/2. Исходный March13 ex19 и frozen3856 calls/mode сохранены без изменений; исходный source содержит9 функций/40 assertions, этот конкретный тест покрывает manhattan/get_x/get_y, не подменяет остальной ex19 пакет.

Причина подтверждена instruction timeline, а не предположением про caller-owned copy. В `manhattan` два local Point frame по2B. Адреса `efea/efeb`, позднее занятые q, сначала используются временными AF saves:

| PC | Opcode | Действие на ef ea/eb |
|---|---|---|
| 808c | f5 PUSH AF | scratch write |
| 8095 | f1 POP AF | scratch read |
| 8096 | f5 PUSH AF | ещё save live A перед p.x store |
| 8099 | f1 POP AF | restore |
| 80ac /80b3 | 02 /77 | настоящие q.x/q.y stores |
| 80bb /80cf | 0a /4e | настоящие q.x/q.y reads |

`before/default/first-object-timeline.json` содержит фактический PC/op/address/count/value. Прежний oracle уже учитывал одну scratch пару; дополнительная live-A save пара изменила физическую кратность. Никакой лишний source object store не появился. Native scratch/stack/save поддерживают значение A и результат, и ASM before/current совпадает: production code не менялся.

Вместо замены2→3 новый test oracle независимо требует **полную точную последовательность data events**: stores p.x,p.y,q.x,q.y с каждым исходным argument byte; затем reads p.x,q.x,p.y,q.y. У get_x/get_y stores оба поля и read только нужного поля. Адреса и значения всех bytes сравниваются, финальные bytes объектов тоже. Raw/final VM прежний независимый oracle сохраняет свои exact events.

Общие profiler read/write counts теперь равны source data event counts плюс отдельно декодированным реальным stack accesses PUSH/POP/CALL/RET. PUSH проверяется немедленно: SP-2 и оба физических байта exact register/return word. Stack reads допускаются только внутри текущего frame. Таким образом scratch writes нельзя спрятать, поменяв ожидаемое число; неожиданное data событие или неточная profiler декомпозиция дают FAIL. PC возврата и SP, полный64KiB вне наблюдаемого stack/spills, код и guard по-прежнему проверяются. Все3856 прежних calls/mode ×3, без нового урезания workload.

Сохранены доизменений FAIL/ASM/BIN/raw/final/alloc и timeline, после — тот же исходник/полный frozen calls, точный executable и native evidence. SHA manifest и копия `/mnt/safe/minz/grace/2026-10-11/manhattan-object-oracle/REPORT.md`. Полный pipeline не перезапускался. Это обновление test-observer под реальные scratch эффекты, не новый ABI/optimization claim.

Финальный frozen executable PASS3,93s, все11568calls.
