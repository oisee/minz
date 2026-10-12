# Исправлен тест живого состояния caller через BDOS

Полный pipeline94a41120 сообщил unexpected BDOS0/0. Текущая ветка воспроизвела падение. Generated assembly показал, что test-owned live_state отсутствует: CP/M runtime испускает main-reachable closure, а main её не вызывал. Тест читал отсутствующий symbol без проверки и запускал PC0. Это ошибка теста, не подтверждённый miscompile.

Теперь тестовый main содержит вызов live_state, чтобы runtime сохранил функцию. Перед прямым вызовом тест обязательно проверяет наличие и диапазон symbol. Независимый byte/predicate oracle не изменён:512 native вызовов (256 inputs, Ordinary/Grace), реальный BDOS all-clobber adapter и стековые canaries проходят. Полный исходный impl CPM main и десять негативных admission вариантов тоже проходят. Совместный root PASS0,175с.

Тестовый main используется только для достижимости дополнительного stress entry; доказательство исходного main остаётся отдельным. Runtime/backend не изменены. Старые frozen logs сохраняются; полный retained executable ещё работает и его FAIL не переписывается.

[Исходное падение](before.txt) · [Диагностика](diagnostic.txt) · [Повтор проверок](root.txt).
