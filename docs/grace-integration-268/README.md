# Tokenizer ABI: отрицательный эксперимент сохранён

Четыре кандидата проверены на двух новых заранее замороженных training входах. Greedy: 519253 такта / 5465 байт; PBQP: 520331 / 5593. Возврат structural slot0-u8 к greedy дал точное равенство greedy; slot2 не помог. Оба критерия сохранили greedy; тот же ASM/BIN прошёл четыре заранее отложенных входа: 957537 тактов.

Нового победителя нет. Production hook для эксперимента не добавлен: research overlay и frozen executable сохранены отдельно. Текущий build получил только read-only экспорт attribution. Root audit.py подтверждает SHA, все четыре стоимости и точное повторное использование выбранного ASM/BIN на heldout; root attribution log рядом.

Разница PBQP +128 байт локализована: init_keywords +120 и четыре str_alloc helper по +2. Независимые class-transfer costs не учитывают совместные physical homes/cycles; поэтому эксперимент с rematerialization литералов на фактических входах вызова будет проверяться отдельно. Это не повод hardpin всех General в один регистр.
