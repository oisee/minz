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


`zork2-well.txt` — вода в teapot, necklace, подъём в bucket и Tea Room:
30/400 на36 ходах. Полный текст VM/Z80 совпадает; Frotz seed1 подтверждает
комнаты, score и inventory. Carousel зависит от RNG;19/64 seed подтверждают
этот checkpoint.


`zork2-cakes.txt` — зелёное пирожное, красное в бассейн, candy, синее:
45/400 очков на 44 ходах. `zork2-robot.txt` продолжает до освобождения клетки,
красной сферы и треугольной кнопки: 65/400 на 56 ходах.
Оба checkpoint подтверждены Frotz seed1 и побайтовым сравнением mzv/Z80.
Запятая в командах роботу на Spectrum — SYMBOL SHIFT+N.
Carousel зависит от RNG; это воспроизводимые маршруты для проверенного
начального состояния, а не решения для любого seed.


`zork2-return-well.txt` сохраняет три сокровища, забирает teapot и спускает
bucket обратно в Circular Room: 65/400 на 63 ходах. Кнопка магнита оставлена
ненажатой. Checkpoint подтверждён Frotz seed1 и полным сравнением mzv/Z80.


`zork2-key.txt` — Tiny Room, ключ на mat, отпирание двери и синяя сфера:
85/400 на 86 ходах. Opener после выталкивания ключа нужно вынуть из keyhole.
Полный вывод mzv/Z80 совпадает; независимый Frotz checkpoint использует
seed14. Среди seed1..64 только этот seed подтвердил тот же маршрут Carousel.


`zork2-dragon.txt` — дракон у ледника, chest в логове, спасение принцессы
и golden dragon statuette: 110/400 на 105 ходах. Полный текст mzv/Z80
совпадает; Frotz seed14 подтверждает результат. Пять сокровищ сохранены.
Маршрут зависит от случайных выходов Carousel, как `zork2-key.txt`.


`zork2-princess.txt` следует за принцессой сразу после открытия chest,
проверяет открытие прохода и получение gold key/rose в gazebo.
Native Z80/mzv:105/400 на139 ходах; Frotz seed14:105/400 на137 ходах.
Предметы и события совпадают; случайный темп NPC отличается. Статуэтка
оставлена в chest, её получение проверено отдельным маршрутом dragon.


`zork2-princess-treasure.txt` возвращается через открытый проход за
статуэткой. mzv/Z80:125/400 на151 ходах; Frotz seed14:125/400 на149 ходах.
Полный вывод mzv/Z80 совпадает, инвентарь сохраняет gold key и сокровища.


`zork2-bank.txt` получает portrait и bills, выносит их через curtain:
mzv/Z80:170/400 на172 ходах; Frotz seed14:170/400 на170 ходах.
Wizard Float временно мешает последнему выходу в mzv, но оба предмета
сохранены. DROP записан через запятые, все команды укладываются в79 символов.


`zork2-bank-return.txt` ждёт окончания Float, возвращается в Dragon's Lair,
оставляет bills/portrait и забирает прежние сокровища. mzv/Z80:170/400
на236 ходах; Frotz seed14:170/400 на230. Полный вывод mzv/Z80 совпадает.


`zork2-ruby.txt` проходит растопленный ледник до Lava Room и берёт ruby:
mzv/Z80:185/400 на243 ходах; Frotz seed14:185/400 на237 ходах.
Полный вывод mzv/Z80 совпадает, ruby и прежние сокровища сохранены.


`zork2-balloon.txt` — supplies из Tea Room, ожидание Fall, полёт и посадка
на Narrow Ledge, крепление hook, stamp в Library: mzv/Z80195/354 хода.
`zork2-balloon-frotz.txt` — независимый маршрут для Frotz seed14:195/368,
те же события и полный инвентарь. Carousel и темп NPC требуют разных команд.


`zork2-wide-ledge.txt` — zorkmid, подъём и посадка на Wide Ledge:
mzv/Z80215/369 ходов. `zork2-wide-ledge-frotz.txt` — независимый маршрут
Frotz seed14:215/356, та же посадка, закреплённый шар и полный инвентарь.
Это отдельные команды для разных RNG, не общий маршрут для любого seed.


`zork2-crown-frotz.txt` — независимая проба Frotz seed 14: взрыв сейфа,
корона, 235 очков на 367 ходах. Это пока не маршрут для mzv/Z80
и не подтверждение безопасного спуска после разрушения площадки.


`zork2-crown.txt` — кирпич и фитиль, взрыв сейфа и корона.
mzv/Z80: 235 очков на 473 ходах, полный инвентарь;
все 41 288 байт вывода совпали, стек 70 байт, история неизменна.


`zork2-crown-descent-vm.txt` продолжает маршрут короны: вход в корзину,
отвязка, закрытие горелки и спуск до Volcano Bottom. mzv: 235 очков
на 504 ходах. `zork2-crown-descent-frotz.txt` независимо подтверждает
корону, полный инвентарь и спуск в Frotz seed 14 на 408 ходах.
Нативный спуск PASS: 42 664 байта вывода совпали с mzv,
стек 70 байт, история неизменна.
