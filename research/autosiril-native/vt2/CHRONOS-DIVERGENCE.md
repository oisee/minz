# Chronos: расхождение параметров native и canonical Ruby

## Исправление первоначального вывода

Предыдущая версия этого документа ошибочно называла настройки двух цепочек
одинаковыми и предполагала повреждение второй ноты dyad. Исследование границ
стадий обнаружило другую причину: native chain и её reference fixtures используют
`max_offset=24`, canonical invocation исходного Ruby — `max_offset=6`.
Это различие конфигурации, а не доказанная ошибка arrange или codegen.

Canonical argv:

```
chronos.mid 2me,1p,1m 8 6 12 0 64 2 6
```

Последние параметры — `orn_repeat=2`, `max_offset=6`. Native standalone
`arrange/core.nanz` задаёт `max_offset=24`; `arrange/corpus.py` и default
`research/autosiril-vm/oracle.rb` согласованно используют24. Поэтому прежние
локальные differential gates могли проходить, не доказывая parity с canonical6.
`render_into_text` правильно воспроизводит переданный ему kind5 — kernel proof
сравнивает один и тот же вход и этим расхождением не опровергается.

## Что наблюдалось

Parent независимо повторил `chronos_divergence_repro.rb`: canonical Ruby render
и сохранённый `native_chronos` отличаются в62 из1352 строк. Первая — row259,
канал1 (`p`): native note51/ornament4 против Ruby note58/ornament0.

Дальнейший worker tracer сравнил normalized ornament payloads, а не только
числовые id из разных intern tables. Его stage-boundary evidence:

| Граница | Клетки/groups | Различия |
|---|---:|---:|
| Flatten / EVENTS kind2 | 2097 | 0 |
| Arrange / kind3 | 2097 | 31 |
| Echo / kind4 | 3215 | 62 |
| Mix / kind5 | 3215 | 62 |

На row259 вход arrange действительно содержит `{51,58}`. При limit24 offsets
`[0,7]` сохраняются и дают base51; при limit6 median7 отбрасывает offset0,
остаётся base58 и нулевой ornament. Native id4 означает `[0,0,7,7]` в его
таблице; Ruby canonical id4 относится к другому ornament. Использовать один
числовой id для вывода о содержимом чужой таблицы было ошибкой.

Actual Ruby arrange oracle с offset6 на native EVENTS даёт0 различий по всем
2097 клеткам и7 определениям. Это устанавливает причину различия параметров,
но ещё не является выполнением новой native цепочки с offset6.

## Оставшаяся проверка

Ветка `fix/autosiril-p-channel`, отдельный worktree и claude-z готовят явный
canonical6 профиль и его фактическое native выполнение arrange→echo→mix.
Требуется0 missing/extra/field differences на каждой границе. Существующий
профиль24 и его проверки сохраняются отдельно. Нельзя просто перезаписать
golden/настройки и объявить parity; новые значения должны следовать unchanged
original Ruby и actual target execution.

Старый reproducer остаётся полезен для выявления различия двух профилей:

```sh
ruby research/autosiril-native/vt2/chronos_divergence_repro.rb \
  --autosiril /path/to/autosiril --planning /path/to/render-planning
```

Original autooisee SHA256:
`41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`.
Сохранённые checkpoint3 receipts исторические: это наблюдения до уточнения
причины, не доказательство исправленного end-to-end native converter.
