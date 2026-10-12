# Rotate sled: выбор по Speed/Size после интеграции

Speed/default сохраняет встроенные RLCA. Size складывает только серии 4–7 инструкций, когда сумма экономии превышает 8 байт полного shared runtime; равенство сохраняет inline. Это общий emitter путь, проведённый через обе pipeline функции.

На неизменённом оригинальном rotate sled: 192 → 186 байт, main 1142 → 1088 тактов. Все байтовые входы helpers, точный stdout main и native память/PC/SP проверены. Дополнительный Size fixture 37 → 36 байт включает полный runtime и все 256 входов.

Root focused peephole/native и original tokenizer/self-Lanz/heldout прошли; точные terminal логи рядом. Full MIR2 на source5f8db72d запущен отдельно, результат ещё ожидается. Старый полный pipeline executable1f2365be не включает эту правку. Before/after evidence и независимый freeze сохранены в reports/rotate-sled-policy.
