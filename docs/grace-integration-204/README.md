# Корпус: новые независимые проверки и явные границы

Все 194 пути сохранены. Теперь 133 имеют независимое или ограниченное VM/native покрытие; ещё два — отдельный bounded native SMC статус, без утверждения эквивалентности PatchSlot VM. Два остаются pending: Lizp cond_test и PL/M Hello. Остальные сохраняют категории ASM/runtime/Agon/CPM/host.

Root повтор: assert полный byte Cartesian — PASS 111,521 с; SMC/hello — PASS 0,347 с; Tetris/advantage — PASS 7,670 с; arena20 — PASS 0,683 с; строки/указатели/fib-pipe — PASS 1,522 с. Каждый пакет сохраняет собственный frozen input domain, source SHA, ASM/BIN/ABI/VM/native traces и ограничения. Compile metadata test не заменяет эти CPU выполнения.

SMC проверяет реальный generated patcher, точные изменения operand bytes и последующие writes original sprite. VM использует heap-slot abstraction, поэтому эти строки не включены в число VM/native-equivalent. Tetris — только original geometry/board test module, не whole game/device. Arena — исходный modular next>end guard, не новый overflow-safe allocator.

Full pipeline 29df08bb продолжает работать; terminal PASS пока отсутствует. PR74 с общим обзором и README открыт в main, auto-merge ожидает обязательные checks. Никакой перенос всей исследовательской ветки в main этим PR не заявляется.
