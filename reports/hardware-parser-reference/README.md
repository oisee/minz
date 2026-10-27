# Обновление native-эталона parser после исправления +1

Замер на 12 октября 2026 года, исходный compiler checkpoint `93cd3248`. Это одна замороженная программа self-Lanz parser и её прежние 30 входов; результат не обобщается на другие программы.

| Native-код | Такты Z80 | Байты |
|---|---:|---:|
| Предыдущий эталон | 1 296 226 | 2377 |
| После исправления unit RHS | 1 288 772 | 2368 |

Изменение: −7454 такта (около 0,58%), −9 байт. Исправление `2dbe4355` (research `59f88d33`) потребляет константу +1 непосредственно при генерации word ADD/SUB, включая переназначенные источники и назначения. Соответствующая native-регрессия проверяет poisoned RHS, граничные значения, живые регистры и точный возврат PC/SP.

Эталонный исходник имеет SHA256 `0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526`. **Исходник, финальный MIR и allocation побайтно совпадают** с прежним checkpoint. ASM diff касается только замен +1 на копирование/INC. Все наблюдения старого и нового native-прогона совпадают, кроме `steps`, который считает исполненные инструкции и закономерно изменяется.

Перед обновлением чисел все три маршрута уже прошли независимый Go parser-оракул: результаты, глобальные значения, содержимое регионов памяти и упорядоченные host-access события. Они затем падали лишь на закреплённом старом T/size/hash. После обновления все три проходят; direct factory и instrumented association подтверждают одинаковый native-артефакт и стоимость. Это обновление воспроизводимого cost checkpoint, а не ослабление семантического оракула. Проверка полного числа вызовов добавлена перед оракулом.

- `audit.json`: сравнение всех трёх маршрутов и происхождение замера.
- `before/`, `after/`: исходники, ASM/BIN, MIR и allocation.
- `assembly.diff`: конкретные изменения эмиссии.
- `reference-before.txt`: сохранённые три отказа старого cost checkpoint.
- `reference-after.txt`: PASS 6,071 с; `unit-native.txt`: PASS 0,011 с.

Полные JSON-наблюдения находятся в persistent checkpoints `hardware-parser-reference-refresh/before` и `/after`, скопированы на шару вместе с этим отчётом. Полный pipeline и CI в рамках обновления не перезапускались; прежний общий прогон закончился timeout, поэтому этот focused PASS не объявляется общим PASS.

Refresh 2026-10-20: -3844 T / -14 bytes, oracle unchanged. Clean origin/main dde023f4
already emitted this exact binary (the drift predates the byte spill slot width fix,
which leaves it unchanged).
The current parser reference is 1284928 T / 2354 bytes, SHA256
`29922da40df51e65b4d9701e6b57e1960a2d000a0191fbfbc6a5ae09e04843d8`.
The archived measurements, logs, binaries, audit, and manifest above retain their
historical values; the executable pin is in
`minzc/pkg/pipeline/memory_host_baseline_test.go`.

Refresh 2026-10-27 (fix/deep-drift): -4947 T / -43 bytes versus the previous executable
pin, oracle unchanged. The binary left the pinned O7 state at P28 (a7c98cee4,
1255941 T / 2258 B), changed bytes only at O4+ (2b70eaa74), and moved again at O9
(fe3f327fb) to the current 1255341 T / 2226 B, SHA256
`0da41545c3af393eaed0f59a6e81b1b07750c3fd9597f07a21bdaf50876484e9`.
All three routes still pass the independent parser state/event oracle; the
executable pin remains in `minzc/pkg/pipeline/memory_host_baseline_test.go`.
