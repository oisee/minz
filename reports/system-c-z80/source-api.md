# Смешанные C/Nanz-модули: CLI, static и extern

16 октября 2026 года. Работа в `feat/system-c-z80`,
[PR #162](https://github.com/oisee/minz/pull/162). Интеграция в main ещё не завершена.

Standalone `.c` CLI теперь использует тот же загрузчик C/Nanz, что и Nanz import.
Он разрешает вложенные `#import` и локальные headers относительно исходного файла.
C-прототипы и extern-декларации пишет автор: resolver связывает HIR, но не
генерирует заголовки. Go API предоставляет `nanz.CSourceResolver`; отсутствие
resolver при `#import` теперь диагностируется вместо пропуска импортируемого кода.

## Реальный сбой изоляции

Два C-файла содержали `static local(x)`: первый возвращал x+3, второй x+5.
Смешанный модуль для x=10 возвращал 26 вместо 28 и на raw VM, и на Z80:
одинаковое имя private-функции связывало второй вызов с первой реализацией.
Теперь стандартный загрузчик даёт импортируемой C translation unit отдельный
private prefix. Прямые вызовы, function pointers, адреса private globals,
рекурсия и local-static storage используют разрешённое имя; локальная переменная
с тем же исходным именем продолжает затенять глобальную.

`extern` без initializer объявляет объект, но не выделяет второе хранилище.
C может обращаться к глобальной переменной, определённой в Nanz. Повторный import
одного файла дедуплицируется. Одинаковые функции при merge coalesce; разные
публичные определения одного имени возвращают явную ошибку. Это не заявление
о полном поведении стандартного host C linker; пользовательские CompileFunc
для других resolver должны обеспечивать свою изоляцию private symbols.

## Проверки

Семь семейств private symbols × 15 входов × default/Grace/Size:
315 native Z80 и 630 raw/final VM исполнений с независимыми арифметическими
ожиданиями. Семейства: private functions/globals, function pointers,
function-local static, shadowing, recursion, address-of. Проба до исправления
26 и после исправления 28 сохранена отдельно.

CLI выполняет C → nested C → Nanz → Nanz, включая локальные headers,
32-битный wrap и общий extern counter. Шесть assert исполняются самим CLI;
ещё 45 native и 90 VM проверяют 15 граничных входов в трёх режимах.
Сохранены именно выданные CLI ASM, их BIN с origin 0x8000, HIR и все шесть
исходных файлов/headers. Проверки native используют собранный Z80 и PC/SP;
сборка экспортированного CLI ASM также проверяется отдельно.

```sh
go -C minzc test ./pkg/c89 ./cmd/minzc ./pkg/interop -run '^Test(C89ExampleHIRContracts|CSourcePrivateSymbolsNativeVM|CSourceConflictingPublicFunctions|StandaloneCSourceImportsNativeVM|CImportWithoutResolverDiagnostic|MergeInto)$' -count=1
```

В общем core HIR/MIR2/cparse/Nanz/interop прошли за
29.669/114.217/0.004/27.058/0.002 секунды. C89 обнаружил старый тест,
который разбирал #import без resolver. Тест исправлен: теперь действительно
загружает зависимости, а не исключает пример. Повторный полный short C89
прошёл за 35.455 секунды; focused C89/CLI/interop — 0.495/0.723/0.002.
FAT12 повторно прошёл за 8.584 секунды: 24 native и 48 VM сценариев.
Это не полный прогон всех тестов CLI и не постинтеграционная проверка main.

Артефакты и SHA256:
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/system-c-z80/source-api/manifest.json`.
На шару с игровыми релизами CI-артефакты не копируются.

## CI предыдущего checkpoint

Все шесть обязательных проверок a6d1fa98 прошли. Advisory output sweep
отметил пять изменений exit status, все baseline FAIL → candidate PASS:
math8, pointers, struct_promote и два новых system-c entry. Переходов
PASS → FAIL там нет. Изменения ASM отражают в том числе исправленные типы;
успешный sweep сам по себе не доказывает поведение полного стороннего FatFS.
Advisory lint сообщает 27 замечаний, преимущественно необработанные ошибки
LoadMemory в тестах; это отдельный проверяемый список, не required gate.
Новый commit должен пройти обязательные CI заново перед merge.
