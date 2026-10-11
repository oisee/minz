# CLI fixed-winner replay и свежая сборка корпуса

Интегрирован cb9e8ba6: --hir-profile-compiler-modes-replay TRAIN_DIR запускает сохранённого победителя на другой workload. Записанный режим компилируется для восстановления ABI, затем требуются точные ASM, MIR и BIN; поиска на heldout нет. Сохраняется тот же compiler executable/source context. 12 прежних frozen PFCCO calls: 5568 T, 52 B, тот же SHA256 бинарника. Независимый Python проверяет арифметику modulo65536, global counter и byte access. Интегрированные четыре CLI replay группы PASS 2,873 с. Дополнительный аудит доверия к сохранённым замерам продолжается; SHA не является подписью и сам по себе не доказывает достоверность чужих замеров.

Свежий полный showcase compile/assemble на a12920cc с MINZ_SHOWCASE_CHECK_KNOWN=1: 192 PASS, один FAIL, ноль SKIP. Единственный отказ — zvm.nanz со story[131072], превышающим обычную 16-bit модель. Результат не доказывает поведение 192 приложений.

Replay packet (37 файлов) и оба полных лога сохранены SHA256-проверенной копией в /mnt/safe/minz/grace/2026-10-11/compiler-docs/. Большой pipeline-cc502-full по-прежнему выполняется; terminal PASS не заявляется.
