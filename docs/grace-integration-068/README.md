# CP/M integration: очистка CLI test state и новый путь корпуса

Полный CLI на 9639275a завершился FAIL за 36,265 с: новый rejection test оставлял глобальный hirProfileAutoDirectory, ломая последующие тесты. Исправлен cleanup самого теста. Повтор в одном процессе runtime→default→profile→memoryhost PASS за 0,061 с; новый полный CLI gate запускается отдельно. Full MIR2 того же контекста PASS за 8,093 с.

Добавление cpm_leaf_console.nanz расширило корпус с 193 до 194 точных путей. Каталог намеренно обновлён: 30 прежних путей имеют ограниченные независимые VM/Z80 proofs, новый demo отдельно обозначен native CALL5-hook proof без VM/настоящей ОС. Каталог PASS за 1,856 с. Остальные 163 пути сохраняют явные pending/runtime/ASM требования. Исторические 193-path артефакты не переписываются.
