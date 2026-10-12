# Tetris CP/M: попробовать в терминале

Запуск: `bash /mnt/safe/minz/grace/2026-10-12/play-tetris/play.sh`

Нужен интерактивный VT100/ANSI терминал. Стрелки — движение; Z — поворот; X — сброс; C — hold; Q — выход, затем любая клавиша для подтверждения. Исходник использует блокирующий ввод: игровое время продвигается по клавишам. Это не realtime-версия. Ctrl-C прерывает эмулятор; wrapper восстанавливает режим терминала.

COM ordinary от b6b4e0c7, runtime guarded stack1024; mze собран из f9d06718. Smoke через mze с вводом zqa: exit0, stdout329B (326 игровых ANSI/text bytes + 3 BDOS1 echoed keys), stderr0. Более строгий native test отдельно проверяет состояние/RAM/SP и clobbers. Настоящая CP/M/железо здесь не проверялись.

Для воспроизведения без шары: из minzc выполнить `go build -o <play-directory>/mze ./cmd/mze`; скопировать `reports/tetris-public-runtime-69ce1022/ordinary/program.com` как `<play-directory>/tetris.com`, рядом положить play.sh. Оба executable должны быть в одной директории с wrapper. Старый системный mze не использован.
