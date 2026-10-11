# Explicit unsigned pair comparison policy

В компиляторе появился явный режим сравнения u16 с сохранением живых пар.
По умолчанию остаётся Legacy. На неизменённом Self-Lanz новый публичный API
подтвердил **1132449 T / 2001 B** вместо **1135370 T / 2000 B**:
−2921 такт (около 0,257%) при +1 байте. Повторное использование загрузки
и диагностические hooks предыдущего эксперимента в production не переносились.

## API и привязки

`mir2.UnsignedPairComparePolicy` допускает Legacy=0 и PreserveForSpeed=1.
`pipeline.Options.UnsignedPairComparePolicy` задаёт режим явно; resolved policy
сохраняется в Steps и передаётся обычной, вторичной и native-аннотированной
генерации. Неизвестный режим, неподдержанный backend и конфликт speed-режима
с OptSize отклоняются до изменения исходного HIR. Zero-valued caller сохраняет
Legacy; Goal=speed сам по себе этот backend-режим не включает.

В обычном профиле параметры компиляции явно привязывают policy=1. Zero-value
пропускается при JSON-сериализации Options, поэтому прежний Legacy config ID
сохраняется. Отпечаток нового executable меняется естественным образом.

`RunZ80MemoryHostBaseline` сохраняет старую сигнатуру и v1 Legacy-контракт.
`RunZ80MemoryHostBaselineWithOptions` принимает только поддерживаемые типизированные
параметры, копирует и проверяет их до вызова source factory. Результат — v2
envelope: compile_options + baseline. Полный envelope обязателен: вложенный
v1-shaped payload сам по себе не описывает выбранную policy.
Selector и закрытый CLI пока продолжают использовать прежний Legacy API.
Memory recipes, host authority и ISA-разрешения не расширялись.

## Проверки

Backend: 280 actual machine cases — границы слов, оба unsigned предиката,
входящие флаги, живые пары и половины, повторные сравнения после арифметики —
и 19 проверок отказов. Интеграционные тесты автоматически распределённого
исходника проверяют результат и глобальную память, Legacy/default/OptSize
равенство, v2Legacy==v1, неизменность raw/source, отказ неизвестных режимов
до factory и отказ при подмене Steps policy до native исполнения.

Два полноразмерных прогона публичного API, по 30 последовательных вызовов:

| Режим | T-states | Байты | Шаги |
| --- | ---: | ---: | ---: |
| Legacy | 1135370 | 2000 | 109829 |
| PreserveForSpeed | 1132449 | 2001 | 109966 |

Actual тест PASS 2.41 s. Два независимых saved-only аудита подтвердили все
60 наблюдений, результаты, 10 globals, три ресурса и 3017 упорядоченных host
событий для каждого варианта. Legacy точно совпал с production dba, а speed
с предыдущим частным compare BIN. Свежий executable context:
`e64597309250ac45b88ac9b31bd2b01c3e3ed05536cea761eb41f6fa9ae9b1a8`.
Этот executable сохранён локально вместе с исходниками, whole v2 envelope,
ASM/BIN/MIR/allocation и независимыми проверками.

В рабочую ветку код перенесён как d16b370b +659efd6a +1155fc66.
Общие HIR/MIR/Lanz/C89/CLI проверки прошли; Nanz без showcase PASS 2.278 s.
Showcase имеет те же 31 падение, новых нет. Полный pipeline **PASS 1078.565 s**.

[Манифест](grace-unsigned-pair-policy-artifacts/manifest.json).
[Предыдущий изолированный эксперимент](grace-unsigned-pair-compare.md)
содержит отдельный compare+reuse результат; его 3,46% не относятся к этой
production-интеграции.

## Следующий этап

Policy-aware association и selector v2 свяжут одну явную backend-политику
с исходным профилем и всеми четырьмя arena/append кандидатами. Размер и время
будут измеряться заново; исходные счётчики не становятся счётчиками native
преобразованных циклов. Старые API/CLI останутся Legacy до отдельного переноса.
