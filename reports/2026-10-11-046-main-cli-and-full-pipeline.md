# Полный SelfLanz, CLI и результат большого прогона

В ветку интегрированы атомарная публикация артефактов compiler-mode CLI и исправление CString в исходном main SelfLanz. Выбранный HashMap: 26 346 → 25 378 T, 990 → 981 B. Это замер обучающей нагрузки; отложенная проверка находится в отдельном отчёте.

Полный main SelfLanz проверен на собранном Z80 с выводом через порт 23 и с явным CP/M BDOS-адаптером. Проверены 135 байтов вывода, все области памяти и globals, стек и неизменность кода. Запуск на физическом железе не заявляется. Исторические parser-эксперименты привязаны к прежнему неизменяемому исходнику, их артефакты сохранены.

После интеграции CLI-тесты прошли за 4,509 с; независимый аудит всех CLI-кандидатов прошёл. Пять проверок main, исторического parser и prospective heldout прошли за 24,664 с.

Большой pipeline-прогон на более раннем cb587fe4 завершился FAIL за 2618,872 с. Старые эталоны parser и настройка ContractOpt уже исправлены последующими коммитами. Условные invariant-кандидаты возвращали 257/513 вместо 1: TestConditionalInvariantRegression, TestConditionalInvariantSiteCandidates и TestInvariantSiteRulesConditionalCandidates. Это требует воспроизведения на текущем коде и исправления; работа поручена отдельному агенту. Полный pipeline PASS не заявляется.

Артефакты: docs/grace-compiler-mode-artifacts (56 payloads), docs/grace-original-lanz-main-native (48 payloads), docs/grace-integration-046 (логи). Копии первых двух пакетов проверены SHA256 на /mnt/safe/minz/grace/2026-10-11/compiler-docs/. Интегрированный код: fed26a86.

Последний опубликованный сайт: https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/current-state.html — публикация v61 предшествует этой интеграции; свежие результаты пока доступны в репозитории и на шаре.
