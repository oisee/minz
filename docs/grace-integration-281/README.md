# Проверка env условий в heldout tests

Прочитаны все шесть соответствующих тестов. Пять прежних семейств (parser, hashmap, PFCCO packet, arithmetic, Pascal sort) без env только не сохраняют новые файлы; само execution/selection/heldout сравнение остаётся. Их rootPASS35,948с в пакете266 не был пустым skip.

Новый literal-call тест требует exportdir и явно Skip без него. Пропущенный jointgate был исправлен в описании276/279, а actualrootexported PASS2,221с и37payloads сохранены в280. SHA текущих проверенных test sources рядом. Тестовый PASS означает выполненную проверку только после сверки её scope/guards.
