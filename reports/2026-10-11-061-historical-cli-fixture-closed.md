# Историческая CLI fixture исправлена

В c784dc4a TestSearchCLICollectAndPairedReplay читает существующий immutable parser snapshot и проверяет точный SHA256 0e92689c..., вместо изменяющегося showcase source. Production guards, native oracle и проверка сохранённых артефактов не изменены. До исправления тест падал с exact source required; после него collect/paired replay/admission PASS за 245,382 с.

Speed: policy1/mask1, 701 T, 2486 B. Size: policy0/mask2, 796 T, 2344 B. Это короткий контроль инфраструктуры; польза на полной нагрузке не выводится из этих цифр. Исторические источники и прежние frozen экспериментальные входы сохранены.

Пакет docs/grace-search-cli-historical-fixture проверен SHA256 на шаре. Запущен новый полный cmd/minzc package run на c784dc4a с timeout15m, лог cli-c784-full.txt. Полный pipeline-cc502-full остаётся в прежнем процессе. Оба terminal PASS пока не заявляются.
