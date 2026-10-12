# Общая копия доказательств корпуса

Аудитор формирует список 4952 уникальных файлов, используемых каталогом 194 исходников и 85 manifests (4486 payload entries). Общая копия создаётся в /mnt/safe/minz/grace/2026-10-12/corpus-evidence-4df8b2f9 с repository-relative структурой. Копирование и последующая проверка SHA ещё выполняются: session 54112; общего PASS shared copy пока нет.

После копирования проверка: python3 scripts/audit_corpus_artifacts.py --shared-root /mnt/safe/minz/grace/2026-10-12/corpus-evidence-4df8b2f9 --output /tmp/minz-corpus-shared-audit.json. Опция только читает shared copy; программа проверяет совпадение SHA и Git tracking локального оригинала. Три unittest покрывают исторические форматы, испорченную shared copy/неотслеживаемый source и unsafe path.

Это сохранность перечисленных доказательств, не повтор исполнения корпуса. Каталог не объявлен полным PASS; живой pipeline отдельный. После терминального результата сохранить аудит рядом и обновить этот статус.
