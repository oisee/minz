# Условный возврат Z80 и исходные IRC-классификаторы

Production fix `706b9c16` (donor `d3c4f964`) убирает пересылки аргументов Then с пути возврата TermCondRet: выбранная ветка выполняет их в своём trampoline. 6 144 native/VM регрессии покрывают шесть условий, два порядка блоков и возврат из A/E. Root targeted CondRet: PASS 0,496 с.

IRC source fix `7e8869b5` отдельно переводит 27 литералов на CString согласно контрактам API и Book v7 §18.1. Старый source SHA сохранён: первоначальный PING уже в raw VM давал Unknown. После исправления строк обнаружился независимый backend miscompile, теперь исправленный.

Root повтор на `abcb764c` исполняет три computational entry исходного исправленного полного модуля: 200 736 native Z80 и 401 472 raw/final VM вызова. Точные output, public reads, память и PC/SP проверяются. Терминальный результат приведён в `irc.txt`. Сеть, TUI и полный main не исполнены; исходный статус external runtime сохраняется.

[Полные ограничения и оракулы](../../reports/irc-classifiers/REPORT.md), [контрпример условного возврата](../../reports/irc-condret-edge/REPORT.md).
