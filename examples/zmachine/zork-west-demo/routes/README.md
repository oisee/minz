# Проверенные игровые маршруты

`early-dungeon.txt` — первые 52 команды walkthrough Microsoft Jericho,
адаптированные для Nanz VM и исходной V3: атака на тролля повторяется шесть
раз, затем добавлены score/inventory/quit. Контроль: 63 очка на 57 ходах,
восемь предметов, включая sceptre, bell, book, candles и горящий torch.

Источник: https://github.com/microsoft/jericho/blob/master/jericho/game_info.py
Jericho задаёт seed12 и Zork I V5. Поэтому это не готовый V3 walkthrough,
а проверенный начальный сегмент; RNG и тексты боя отличаются.
Тест Z80 сравнивает весь текст с Nanz VM; Frotz seed12 отдельно проверяет
контрольный score и полный инвентарь. Полное прохождение ещё не доказано.


`dam-lobby.txt` продолжает тот же маршрут до первых 100 исходных команд
Jericho (с дополнительными атаками). Контроль: 102 очка на 104 ходах,
Dam Lobby, platinum bar/candles/bell/book/lamp/lit torch. Coffin перенесён
в trophy case. Frotz seed12 подтверждает score и инвентарь; случайный текст
не является общим оракулом для двух разных RNG.


`crystal-skull.txt` продолжает первые 130 исходных команд Jericho:
ритуал bell/candles/book, Land of the Dead, skull, возврат в дом.
Контроль: 127 очков на 134 ходах; в case platinum bar/crystal skull/gold coffin,
в инвентаре lamp/lit torch. RNG-адаптация боя сохраняется.


`cyclops.txt` — первые 146 исходных команд Jericho, затем score/case/inventory.
Контроль: 142 очка на 150 ходах, четыре сокровища в case, rusty knife/lamp/lit
 torch в инвентаре. Более длинный probe до200 исходных команд даёт146 в VM,
но Frotz seed12 погибает после кражи света; он не принят как общий checkpoint.


`trident.txt` проходит первые200 исходных команд Jericho: maze/coins, dam,
mirror, Atlantis Room/trident и air pump. Контроль146 на204 ходах, четыре
сокровища в case и pump/trident/lamp/lit torch в инвентаре. Frotz checkpoint
использует seed1: проверка seed1..24 даёт18 успешных маршрутов и6 смертей;
seed12 здесь погибает после кражи света. RNG игрового Nanz/Z80 не изменён.


`treasure-room.txt` проходит первые 260 исходных команд Jericho. Контроль:
233 очка на 264 ходах, Treasure Room с живым thief и silver chalice;
в инвентаре nasty knife/rusty knife/lamp/lit torch. Z80 побайтово совпадает
с VM; Frotz seed1 отдельно подтверждает счёт, chalice и полный инвентарь.
Это проверенный длинный сегмент, ещё не полное прохождение.


`full-walkthrough.txt` — полный маршрут V3 после адаптации атак на тролля:
350/350 на 399 ходах, Inside the Barrow, Master Adventurer. Frotz seed1
подтверждает финал, Nanz VM и Z80 дают побайтово одинаковый игровой вывод.
Маршрут включает смерть вора и возврат его сокровищ; сравнение 16-битного
branch offset в spill требует исправления `72399de6`.
`cold-passage.txt` сохраняет промежуточную точку 271/304 для диагностики.


`late-save-walkthrough.txt` сохраняется после первых 300 команд полного
маршрута, меняет инвентарь, восстанавливает его и заканчивает игру: 350/350 очков на 400 ходах.
Native SAVE1 660/2 204 B. Полный вывод совпадает с VM.

`zork2-garden.txt` проверяет карту V3 из 2dungeon.zil: Great Cavern→SW→S→SE,
Gazebo, семь предметов, Path Near Stream. Frotz seed1 подтверждает inventory
и score0/15 moves; VM/Z80 текст совпадает. Это отдельный маршрут, не walkthrough II.


`zork2-riddle.txt` решает каменную дверь (`answer "a well"`) и берёт pearl
necklace:20/400 очков на25 ходах. Z80/VM полный текст совпадает; Frotz seed2
подтверждает checkpoint. Carousel зависит от RNG; seed audit1..64 даёт14
совпадающих checkpoint. Кавычки на Spectrum вводятся SYMBOL SHIFT+P.
