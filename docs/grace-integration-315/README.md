# TUI ZX в каталоге и проверка сборки пакетов

Каталог сохраняет 194 пути и прежние **153** записи independent/bounded VM/Z80. TUI ZX переведён из inline-ASM audit в отдельную категорию native ASM: теперь их 15, непроверенных inline ASM — 9. Основание: исходный main, 194 ROM output bytes, полный RAM и настоящий DI/HALT в трёх режимах. Этот результат не становится VM differential или моделью экранных пикселей. [Точная граница](../../reports/tui-zx-main/README.md). Экспорт каталога **PASS 3,705 с**.

Команда `go test ./pkg/... ./cmd/... ./internal/... -run '^$' -count=1` собрала и запустила пакеты без тестовых функций. Один запуск, cmd/mzx, завершился GLFW panic: DISPLAY отсутствует. Остальные пакеты прошли. Это проверка сборки и package init, не исполнение тестового корпуса. Отдельный `go test -c ./cmd/mzx` собрал executable успешно. `xvfb-run` здесь не установлен; runtime GUI init остаётся проверкой для окружения с X server. [Лог запуска](compile-run.txt) · [Сборка MZX](mzx-build.txt).

Полный pipeline94a41120 ещё исполняется. Новый tui_zx/rotozoomer main проверен отдельно и отсутствует в этом старшем executable. LIR package отсутствует в текущем дереве; сохранившиеся упоминания extraction в комментариях не являются импортами.
