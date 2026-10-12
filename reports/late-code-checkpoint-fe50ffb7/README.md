# Поздний код относительно живого pipeline

Checkpoint fe50ffb7. Точная сверка diff относительно b6b4e0c7 сохранена в scope.json: только CalledMain/Agon entry/reserved symbol, Agon VDU library и два новых test files. Арифметический production и аллокатор после сборки большого pipeline не менялись.

Свежие полные HIR, Nanz, Lanz и Lizp завершились PASS; Agon entry regression также PASS. MIR2/z80asm завершились PASS: 96,507 s и 0,047 s; exit 0, лог backend.txt. jobs.json сохраняет исходные PID/session и команду. Большой pipeline продолжается тем же executable; его будущий результат относится к b6b4e0c7, не к поздним изменениям.

Этот пакет подтверждает только перечисленные завершённые проверки и границы checkpoint. Независимые device-пакеты Agon и композиционные VM/native домены проверены отдельно. Полного PASS текущего корпуса пока нет. Проверка Git-пayload retention повторена: 194 исходника, 85 manifests, 4486 записей, 0 ошибок.
