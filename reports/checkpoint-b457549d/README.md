# Проверка checkpoint b457549d

Свежая направленная сверка arithmetic, Pascal bubble sort, PFCCO и 12 комбинаций contract/Grace/table прошла. Frozen входы и независимые ожидания сохранены. Это повтор существующих heldout, не объявление новых отложенных нагрузок. [Лог](heldout-tables.txt).

Weighted caller: 238 → 142 такта, модуль 44 → 20 байт; дополнительные PFCCO/Grace gains на нём отсутствуют. Bubble sort heldout: 164671 → 164275 тактов, 488 → 486 байт. PFCCO Speed heldout: 6252 → 5568 тактов при 52 байтах; Size сохраняет первый вариант при равенстве размера. Arithmetic выбирает baseline, heldout 404193 такта.

Parser/hashmap heldout завершились PASS 22,074 с: parser2195027→2137641 такта при2377→2478 байтах; hashmap258962→237948 тактов при996→976 байтах. Полный core завершился с единственным падением Nanz catalog: устарели SHA profile_guided_sum_test после перепроверенной cost-сверки c94feb87 и cpm_runtime_test после CLI pin gate. Все остальные пакеты прошли, включая HIR30,625с и MIR2 71,560с. Обе SHA references исправлены, полный Nanz завершился PASS 17,628 с после ремонта references; первичный FAIL лог сохранён. Полный CLI ещё выполняется. Retained полный pipeline94a41120 подтверждён живым; он предшествует свежим изменениям и не доказывает этот checkpoint. Точные команды и log paths сохранены в jobs.json. Общая цель остаётся открытой.
