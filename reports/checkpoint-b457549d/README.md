# Проверка checkpoint b457549d

Свежая направленная сверка arithmetic, Pascal bubble sort, PFCCO и 12 комбинаций contract/Grace/table прошла. Frozen входы и независимые ожидания сохранены. Это повтор существующих heldout, не объявление новых отложенных нагрузок. [Лог](heldout-tables.txt).

Weighted caller: 238 → 142 такта, модуль 44 → 20 байт; дополнительные PFCCO/Grace gains на нём отсутствуют. Bubble sort heldout: 164671 → 164275 тактов, 488 → 486 байт. PFCCO Speed heldout: 6252 → 5568 тактов при 52 байтах; Size сохраняет первый вариант при равенстве размера. Arithmetic выбирает baseline, heldout 404193 такта.

Полные core и CLI, а также parser/hashmap heldout запущены отдельно на b457549d и пока не имеют terminal результата. Retained полный pipeline94a41120 подтверждён живым; он предшествует свежим изменениям и не доказывает этот checkpoint. Точные команды и log paths сохранены в jobs.json. Общая цель остаётся открытой.
