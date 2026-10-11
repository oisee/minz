# Indexed historical tables, byte lanes и свежий gate

Snapshot `f5c7a09e`: indexed converter `637e5356`, byte-lane identity `f5c7a09e`.
Root повторил actual ENRT5v triple на всех256 byte inputs, прямой и косвенный
вызов, independent modulo oracle, SP/code checks: pipeline PASS0.602с.
Reader и simultaneous interference negatives также PASS.
Это реальная historical запись: module34→31B, direct66→54T, indirect123→111T.
[Converter](../../reports/2026-10-11-indexed-historical-table-conversion.md).

Pure trunc8(and16(x,255)) identity сохраняет обе исходные byte-store инструкции.
FAT12 st_word220→93T, полный модуль −23B в каждом режиме. Root exhaustiveVM,
store identity/fault-prefix, FAT caller и оригинальный C89 utility gate:
MIR2 PASS0.136с, pipeline PASS11.160с.
[Изменение](../../reports/2026-10-11-065-byte-lane-extraction.md).

Свежие полные MIR2/VIR после indexed+lane: PASS12.391с /2.178с.
Новый import-enrt CLI собирается, собственных CLI tests пока нет;
unit reader и native converter integration tests приведены выше.
Это не новый полный HIR/Nanz/CLI или pipeline gate.
Полный pipeline3e143f9f продолжает работать в прежнем процессе.

Sites64 опубликован приватно: source936cca8b, compiler snapshot2d09774a,
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/current-state.html .
Он содержит parallelCLI/table-negative/legacy-prefix/CUDA/C89/FAT-hoist пакеты,
но предшествует indexed+lane и stage-timing audit. Новые пакеты в repo/share.
Полный опубликованный site64 скопирован и SHA-проверен: 9776 файлов,
247802158 байт. Архив публикации SHA256
36baefdfe8aa2f4aa180d663b24a2291026803abdec3b7cb17eefb5715282486.
Манифест site-v64-copy-manifest.json на /mnt/safe/minz/grace/2026-10-11.

Manifest byte-word-fusion первоначально ссылался на четыре ignored MIR файла;
они force-added в a71dd614, root заново проверил все99 payload SHA.
Byte-lane packet содержит199 проверенных payload, exactMIR сохранён в Git.
Это исправляет архивную полноту; unsafe direct16 counterfactual остаётся
явно test-only исследованием, не production memory-fusion.
