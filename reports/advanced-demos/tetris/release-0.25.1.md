# MinZ Demos 0.25.1: играть и проверить

[Релиз опубликован 15 октября 2026](https://github.com/oisee/minz/releases/tag/v0.25.1).
Источник пакетов и тега — main `a284fafc` (PR #128).
В комплекте Zork I/II, Spectrum Tetris с выбором скорости, rotozoomer и SMC sprites.

| Платформа | Игровой ZIP |
|---|---|
| Linux x64 | [minz-demos-0.25.1-linux-amd64.zip](https://github.com/oisee/minz/releases/download/v0.25.1/minz-demos-0.25.1-linux-amd64.zip) |
| Linux ARM64 / Raspberry Pi 64-bit | [minz-demos-0.25.1-linux-arm64.zip](https://github.com/oisee/minz/releases/download/v0.25.1/minz-demos-0.25.1-linux-arm64.zip) |
| Intel Mac | [minz-demos-0.25.1-darwin-amd64.zip](https://github.com/oisee/minz/releases/download/v0.25.1/minz-demos-0.25.1-darwin-amd64.zip) |
| Apple Silicon | [minz-demos-0.25.1-darwin-arm64.zip](https://github.com/oisee/minz/releases/download/v0.25.1/minz-demos-0.25.1-darwin-arm64.zip) |
| Windows x64 | [minz-demos-0.25.1-windows-amd64.zip](https://github.com/oisee/minz/releases/download/v0.25.1/minz-demos-0.25.1-windows-amd64.zip) |
| Windows ARM / Parallels | [minz-demos-0.25.1-windows-arm64.zip](https://github.com/oisee/minz/releases/download/v0.25.1/minz-demos-0.25.1-windows-arm64.zip) |

Распакуйте и запустите `mzx`, `mzx.exe` или `MinZ Demos.app`.
Up/Down/Enter выбирают демо; Escape возвращает меню. Tetris: 1 — пиксель за
два кадра, 2 — пиксель за кадр; O/P — движение, Q — поворот, A — вниз,
SPACE — сброс, H — hold. F3 включает turbo, F4 удерживает его временно.
SAVE остаются в пользовательском config-каталоге при обновлении.

[Отдельные образы](https://github.com/oisee/minz/releases/download/v0.25.1/minz-demos-0.25.1-images.zip)
содержат SNA/TRD, лицензии и пример запуска Zork II с bank 10.
Скрипт образов использует установленный `mzx` из PATH или переменную `MZX`.
Для сохранений используйте свой существующий диск либо новую копию blank TRD.
Mac app подписан ad-hoc; Apple notarization отсутствует.

## Что подтверждено

[Native-сборка из main](https://github.com/oisee/minz/actions/runs/38663514838):
все шесть платформ PASS. На native runners собраны GUI/headless файлы,
проверены embedded boot, уровни Tetris, disk/session и запуск готовых файлов.
Все package manifests и ZIP CRC проверены. Девять экранных SHA и SHA
сохранённого диска совпали между платформами. Загруженные релизные файлы
скачаны обратно: все семь ZIP и два файла метаданных совпали по SHA256.

[Контрольные суммы](https://github.com/oisee/minz/releases/download/v0.25.1/SHA256SUMS)
· [Манифест](https://github.com/oisee/minz/releases/download/v0.25.1/manifest.json).
Игровая копия: `/mnt/safe/minz/releases/0.25.1/`.
Полные новые CI-протоколы находятся локально, на шару не копируются.

Tetris использует парные запечённые draw/erase прямо в экран; адреса готовятся
до HALT. Спокойное падение проверено при выбранной скорости. Некоторые
входные/lock действия занимают два кадра, проверенные тяжёлые line clears —
до трёх. Физическое железо не входит в эту проверку эмулятора.
[Полный renderer, оракулы и границы](direct-renderer.md).

Следующий helper/preview checkpoint уже в main `b4cb44d7`, PR #130:
обычный модуль 24 868 → 24 560 байт. Его готовые пакеты ещё не выпущены;
не переносим этот выигрыш на бинарники 0.25.1.
[Изменения и измерения](span-helper.md).
Глубокая матрица релизного тега завершена:
[run 38664425375](https://github.com/oisee/minz/actions/runs/38664425375).
Все 12 pipeline-шардов, assert matrix и judge self-tests прошли.
Folded fuzz: 10 000 PASS, 868 секунд.

Общий workflow имеет статус FAILURE: default-output sweep обнаружил
изменения по сравнению с HEAD~1. Сравнены 838 исходников и 326 выходных
файлов. Единственное изменение exit status — Tetris: прежний duplicate
import error сменился успешной сборкой. Ещё девять исходников имеют
изменённый ASM; сам sweep сравнивает байты, а не доказывает эквивалентность.
Эти изменения нельзя автоматически считать ни регрессиями, ни доказанными
оптимизациями по одному SHA.

Отдельный direct-call fuzz завершился с находками: 7 119 PASS,
370 расхождений Z80 != MIR2, 2 511 ошибок компиляции, 513 секунд.
По текущей политике CI этот режим сохраняет находки и не красит job за
обычные findings; его зелёный job не означает отсутствие расхождений.
Не установлено, какие из них новые и какие относятся к известным ограничениям.
Артефакты сохранены локально в `tetris-direct/release-fuzz-direct-report/`;
они требуют отдельного разбора и не меняют область доказательств Tetris.
Этот релиз не подтверждает корректность всех произвольных программ.
