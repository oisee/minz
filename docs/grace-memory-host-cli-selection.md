# Закрытый memory-host выбор через CLI

Добавлен явный opt-in CLI режим --memory-host-arena-append DIR с
--memory-host-workload FILE и необязательным --memory-host-profile FILE.
Он принимает оригинальный Nanz, backend Z80 и явный targetCPM. Osize меняет
цель выбора, не compiler options кандидатов. Обычные HIR/legacy PGO режимы
и несовместимые параметры отклоняются до исполнения и записи.

Исходник читается один раз, ограничен8MiB; первая версия принимает ровно
одну полную source unit, импорты отклоняются. Workload4MiB разбирается из
одного owned буфера с отказом duplicate/case-alias/unknown/trailing/null keys.
Профиль8MiB либо реально собирается тем же exe, либо строго загружается и
повторно наблюдается. Нет произвольных host callbacks/assembly/proofs/outcomes.

Четыре candidatebaseline сохраняются целиком. Выбранные ASM/BIN берутся прямо
из measured evidence; normal codegen повторно не запускается. decisions.json
связывает SHA точных сохранённых selection.json байтов, выбранные choices,
BIN SHA, размер и T. Эти бинарники включают закрытые adapters/layout для
workload; unrestricted standalone внешнее окружение не заявляется.

Absolute/resolved-parent пути и SameFile проверяют input/output/artifact
коллизии, включая symlink/hardlink, до исполнения и перед записью. Внешний-o
копирует тот же ASM; конкурентная подмена файловой системы вне контракта.

Изолированная новаяCLI прошла full30 speed/size: speedboth1147025T2197B,
sizeappend1150124T2076B. Все8payload прошли независимые host/typed/native
audits. Smallcollect/providedprofile replay совпадают полностью, mask1
763T2212B; external-o совпал с выбраннымASM.

Три frozenCLI файла интегрированы; cmd/minzc30.102 s PASS, backendunchanged.
Rootbuild имеет другой executableSHA от isolatedbuild: обе сохранены отдельно.
Новыйrootexe прошёл freshfull30goals иsmallprofile/replay/output проверки;
все8candidate BIN совпали с изолированной сборкой, profilecontexts привязаны
к новомуexe. Independenthost/typed/selection/preproof audits прошли,
пакетыisolated иproduction сохранены отдельно. Общий pipeline предыдущего неизменного
selector прошёл569.037 s; повтор без backend изменений не требуется.

[Артефакты](grace-memory-host-cli-artifacts/manifest.json).
