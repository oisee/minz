# Frozen CLI: точные контексты и replay со шары

Канонические пакеты документации находятся в `compiler-docs/`. Одноимённые
каталоги в `compiler-reports/` — сохранённые дополнительные исторические копии;
их не удаляли. `compiler-reports/` прежде всего содержит отчёты из repo `reports/`.
Для новых ссылок используйте `compiler-docs/`.

| Frozen executable в compiler-checkpoints | SHA256 | Канонический пакет |
|---|---|---|
| mz-compiler-modes | d49dc066e0f3eb0b29fea3eca7f75b521fc8e463e9e774354352c1cb3d1a4555 | compiler-docs/grace-compiler-modes-cli |
| mz-memory-host-selected-output | 2c9f5ee900ef909ba662e4242761f08199b8a99c8b19f8d3bdaf54834336ee75 | compiler-docs/grace-memory-host-selected-output |
| mz-compiler-mode-replay-final | fd1834f762935a3413738d0fabc429a5ba5cd23c732f40ee12a2224f8d769047 | compiler-docs/grace-compiler-mode-replay |
| mz-compiler-mode-replay-trust-final | 180bd2a027b427a6f582fc18080135a9222a39646b4982c0522ad53e7577506b | compiler-docs/grace-compiler-mode-replay-trust |

Это разные исполняемые компиляторы. Training одного нельзя подавать другому:
replay сверяет хеш executable, полную HIR/ABI, workload и зависимости. Смена
исполняемого файла требует нового training. Не пересобирайте executable для
повтора исторического замера.

## Проверенный replay существующего training

Команда запускается из repo/worktree, где относительный source-unit имеет ровно
имя `examples/nanz/pfcco_weighted_packet.nanz` и SHA256
`6a8ecc8d50e41f5609cabb061b7a06474a7e72619295259d9e5aa4e3cc5bb4ab`.
Абсолютное имя того же файла изменит frontend source-unit identity и будет
отклонено. Для автономного повтора можно создать рабочий каталог с таким
относительным путём и скопировать туда `source.nanz` из training; исходник без
импортов. Проверяйте хеш перед запуском.

```sh
/mnt/safe/minz/grace/2026-10-11/compiler-checkpoints/mz-compiler-mode-replay-trust-final \
  examples/nanz/pfcco_weighted_packet.nanz --target cpm \
  --hir-profile-compiler-modes-replay /mnt/safe/minz/grace/2026-10-11/compiler-docs/grace-compiler-mode-replay-trust/artifacts/training \
  --hir-profile-auto ./packet-heldout-share-replay \
  --hir-profile-auto-workload /mnt/safe/minz/grace/2026-10-11/compiler-docs/grace-compiler-mode-replay-trust/artifacts/heldout/workload.json \
  -o ./packet-heldout-share.bin
```

Выберите отсутствующий output-каталог: существующие evidence не заменяются.
В проверке использован frozen executable прямо со шары. Команда воспроизвела
записанного победителя на training, затем исполнила его сохранённый BIN на всех
12 heldout вызовах. Нового поиска четырёх кандидатов не было. Результат:
mode 1, **5 568 T / 52 B**, SHA256 BIN
`23a46320f4d5adaf3a706457cf721145f0ff0df85efcd4c8e0e2b0f0d83642f0`.
Логи, новый heldout output и точный CWD записаны в `verification.json`.
Автономный повтор также проверен из нового каталога, содержащего только
скопированный исходник по этому относительному пути: `standalone-replay.txt`.

Весь вывод этих режимов — измеренный assembled image, а не связанная CP/M COM
программа. Memory-host parser использует собственный workload/schema и отдельный
CLI; его команды находятся в `compiler-docs/grace-memory-host-selected-output/REPORT.md`.

## Границы проверки

Проверены SHA256 всех 11 executable в `compiler-checkpoints` и 276 payloads
трёх пакетов в канонической и дополнительной копиях: replay (37×2), replay-trust
(36×2), memory-host-selected-output (65×2). Старый checkpoint-манифест перечислял
9 executable и пропускал оба replay-файла; сохранён в `checkpoint-manifests/`.
Новый полный checkpoint-манифест перечисляет все 11 executable.

Это проверка конкретных пакетов и пользовательского replay. Исторические полные
`compiler-docs-sha256.json`/`compiler-reports-sha256.json` сохраняют прежний checkpoint
и прежний охват; новые пакеты не объявляются частью тех проверок. Полную свежую
сверку всех docs/reports этот аудит не выполнял.
