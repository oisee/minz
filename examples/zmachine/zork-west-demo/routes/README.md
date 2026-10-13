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
