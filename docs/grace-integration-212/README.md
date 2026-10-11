# PL/M catalog после executable module fix

Каталог сохраняет прежние counts и pending статус исходного Hello: module entry и byte literals исправлены, но runtime termination/output остаются ограниченными. Новый bounded original-main packet и root209 replay заменяют устаревшее утверждение об отсутствующем main. String/pointer proof hashes обновлены после фактического повторного root209 выполнения изменённого test fixture; его source algorithms/domains не менялись.

Registry сверяет текущие exactsource/evidence hashes. Изменение источника или доказательного файла не должно молча делать старый PASS актуальным. Эта metadata проверка не заменяет CPU suite.
