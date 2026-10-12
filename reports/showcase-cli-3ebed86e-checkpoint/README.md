# Compile/assemble корпус и CLI после Z80 literal fold

Checkpoint 3ebed86e содержит production literal-overwrite rule и проверенные parser/hashmap heldout. Showcase compile/assemble завершился с PASS: 193 пути PASS, один SKIP по 16-bit capacity. Полный список и точная причина находятся в showcase-result.json; это не исполнение всех программ.

Новый полный CLI test binary запущен отдельно. На момент записи работает, итог не объявлен. Бинарник сохранён локально и на шаре, SHA256 совпадает. Session/PID, команда и окружение находятся в cli-job.json. Этот новый gate нужен после backend fold; прежний CLI85fd7531 PASS относится к коду до изменения.

Длинный полный pipeline5d90b7aa продолжает свой первоначальный запуск и не перезапускается; он тоже не содержит новое backend rule.
