# Работа возобновлена через Z.ai workers

Alice после завершения книги явно попросила решать оставшийся tail через
`codex-z + claude-z`. Это новое указание заменяет паузу из handover. Legacy STOP
marker не удаляется: daemon/Herdr не используются, работа запущена явно.

Оба wrapper найдены в ~/.zshrc. Минимальный headless smoke с low effort:
Claude→Z.ai ответил READY за2.4s, Codex→Z.ai за11.5s. Это подтверждает доступность
каналов на момент проверки. Codex предупреждает о fallback model metadata;
глобальная конфигурация и ключи не менялись. Первая прежняя попытка Claude с
длинным audit prompt/default effort имела timeout120s; это историческая запись,
не текущий статус обоих коротких smoke.

| Worker | Изолированная ветка/worktree | Первый checkpoint |
|---|---|---|
| codex-z / glm-5.3 | spike/autosiril-render-z; /tmp/minz-autosiril-render-z | Nanz row kernel + raw/final MIR2/native Z80 против203-case Ruby corpus |
| claude-z / glm-5.3[1m] | spike/autosiril-vt2-z; /tmp/minz-autosiril-vt2-z | Actual Ruby Step9..11 vs independent host model и bounded pattern/VT2 design |

Первый checkpoint не заменяет полный objective. После review нужны actual
file stages, весь admitted profile/IO/negative contracts, inherited gates,
required fresh/frozen CI, downloaded artifact audit и own PR/main integration.
Полный VT2 writer отделён от будущего VTX export; не называть planning/kernel
proof готовым native MIDI converter.

Рабочие задания и launcher: /tmp/minz-z-workers/{render-task.md,vt2-task.md,
run_worker.py}. Status/log/final files там же; output roots render-results и
vt2-results. Состояние live проверять по process/session, не только JSON-файлу.
Каждый worker должен оставить WORKER-REPORT.md и локальный commit или diff.
Parent проверяет semantics/coverage/scope и только затем push/PR/merge.

MCP/browser integrations отключены. У workers нет поручения push/merge/release,
сообщения другим, SSH/i7/Herdr, изменение credentials/global config, root user
files, books/workflow или compiler defaults. Shared MIR/pipeline bug сначала
сообщить parent для координации с Claude u7; не исправлять core молча.

Сами runners имеют ограничение30min на checkpoint и сохраняют журналы. При
timeout не перезапускать автоматически: проверить процесс, diff и причину,
передать адресный follow-up. При stop Alice остановить оба процесса; не
продолжать из старого wakeup.
