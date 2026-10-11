# Поддерживаемые пакеты без LIR

На 5b317150 команда `go test ./pkg/... ./cmd/... ./internal/... ./tests/... -run '^$' -count=1` завершилась exit 0. Это compile-only поддерживаемых пакетов, не полный runtime gate. rg не нашёл живых Go imports github.com/minz/minzc/pkg/lir и каталога pkg/lir. Исторический архив research LIR сохранён и не входит в build graph.

Полные HIR/MIR2 после multiply relocation уже сохранены отдельно; runtime recursion, typed_print и indirect signature fixes в процессе. Общая цель не закрыта. Standalone scripts с историческими mixed-package/relative-import проблемами не входят в эту команду; общий `go test ./...` PASS не заявляется.
