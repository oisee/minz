# Интеграция 164: terminal full CLI

Полный `go test ./cmd/minzc -count=1 -timeout=20m` на code checkpoint568b7d08 завершился **PASS384,240с**, exit0. [Terminallog](cli-terminal.txt). Exact executable ранее сохранён локально/наshare с SHA в [start provenance](cli-start.json). Process/session подтвердили terminal, не перезапускались по elapsed времени.

Checkpoint включает callbacks/purity/direct-MIR-call verification, но предшествует caller-owned struct ABI. Поэтому результат не переносится на новый ABI без проверки. Full current pipeline/CLI остаются отдельными последующими gates.
