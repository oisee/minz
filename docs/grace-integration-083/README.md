# Интеграция083: повторная проверка цели на3e143f9f

Три существующих замороженных train/heldout эксперимента повторены PASS28,959s. Это refresh старого prospective evidence, не новый слепой holdout; входы и решения не переобозначены.

- Parser: train1016414→992049T,2364→2344B; выбран append-only mask2. Тот же выбранный BIN на heldout2141572→2093170T.
- HashMap: Speed и Size выбирают mask3 по training26346→23437T,990→970B. Тот же BIN на48heldoutcalls258962→237948T. Independent results/snapshots/orderedmemoryaccess проверены.
- PFCCO packet: Speed выбираетmask1, heldout6252→5568T при52B; Size сохраняетmask0, так как все52B. Это реальная независимая цель Size, без скрытого штрафа Speed за размер.

Явный Z3 table gate PASS4,606s. Weightedleaf+actualcaller все256inputs: модуль44→20B,helper182→86T,caller238→142T с actualABI/SP. Нулевой gain low_nibble сохраняет baseline. Настоящий indirect apply(triple,x) все256:123→111T,34→31B; triple66→54T. Calls запрещены в local table admission, caller исполняет проверенный callee table по прежнему контракту.

Compile-only поддерживаемого toolchain `go test ./pkg/... ./cmd/... ./internal/... ./tests/... -run '^$'` PASS. Это не full runtime gate и не claim `go test ./...` со старыми mixed-package scripts. Живого pkg/lir каталога нет, оставшиеся UseLIR/LIRCheck строки в Go только negative retirement tests. Полезность всех возможностей архива LIR не объявляется завершённой: дальнейшие measured gaps рассматриваются отдельно.

Полный pipeline3e143f9f запущен до новых изменений; terminal ещё нет. Последний fullcore9b1637ef PASS сохранён в082. Осталось расширять independent behavior coverage и проверять полезные переносы/actualABIcost; общая цель активна. SHA payload archives повторно открыты и проверены, все файлы исходных запусков сохранены на dated share.
