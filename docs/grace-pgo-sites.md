# Первый слой LLM-less PGO: места циклов в исходном HIR

10 октября 2026. Следующий этап после [аудита iterator stages](grace-iterator-stages.md).

## Что работает

Nanz прикладывает к разобранным statements логическое имя входного unit, точный byte offset первого токена, строку, byte column и SHA256 полного исходника. Метаданные хранятся отдельно в `hir.Module.SourceLocations` и не меняют машинный ABI. Импортированные Nanz-модули сохраняют собственные происхождения после mangling имени функции. Для metafunction-generated statements координаты исходника не выдумываются.

`hir.BuildLoopSiteMap` проверяет HIR и выдаёт JSON-сериализуемый реестр исходных while, for-range и for-each. ID строится детерминированно из unit, окончательного имени функции, позиции, вида и ordinal совпадающих мест; transient pointer/node ID не используется. Записи отсортированы по ID. Синтетические loops без origins считаются в unmapped_loops. Неверная приложенная origin-запись приводит к отказу построения карты.

`CompileHIRSteps(..., Options{CollectHIRSites:true})` возвращает карту в `Steps.HIRLoopSites` **до Grace и splitting**, с отметкой stage=input-hir. По умолчанию сбор карты выключен. SourceLocations Nanz формирует при разборе независимо от этого флага.

```text
Nanz statement
    ├── source unit / offset / kind / function / ordinal → site ID
    └── SHA256 полного source unit → версия исходника
                         ↓
              карта исходных циклов HIR
                         ↓
      будущий collector и lineage к MIR (ещё не реализованы)
                         ↓
      профиль выбирает стоимость; proof/guard разрешает rewrite
```

## Настоящий исходник

У `examples/nanz/01_sum_array.nanz` один исходный while. Карта указывает строку 10, byte column 5, byte offset 281, функцию sum_array. [Исходник](grace-site-artifacts/sum-array.nanz) и [полный sites.json](grace-site-artifacts/sum-array-sites.json) сохранены вместе.

```json
{
  "function": "sum_array",
  "kind": "while",
  "hir_path": "body/s2",
  "origin": {
    "unit": "examples/nanz/01_sum_array.nanz",
    "byte_offset": 281,
    "line": 10,
    "byte_column": 5
  }
}
```

Это карта происхождения, не профиль исполнения и не сертификат корректности. Компилятор пока не принимает профиль для выбора Grace-правил.

## Проверки

Повторный parse даёт побитово одинаковый JSON. Изменение тела с сохранением позиции может оставить ID прежним, но обязательно меняет SHA256; сдвиг позиции меняет ID. Синтетическая замена while не наследует origin автоматически. Неверный SHA отклоняется. Проверен импорт helper с сохранением unit/hash и mangled function. CompileHIRSteps проверен с collection on/off и Grace on/off: карта остаётся явно input-HIR.

[Направленные проверки](grace-site-artifacts/contracts.txt), [пакеты HIR/MIR2/Nanz/pipeline/Lanz](grace-site-artifacts/packages.txt). Showcase отдельно: прежние 32 падения, новых нет. Проценты ускорения не заявляются.

## Что ещё нужно до PGO

- Идентичность expression-sites, iterator-generated loops и inline-context. Сейчас покрыты statements Nanz; другие фронтенды без приложенных origins остаются unmapped.
- Lineage при splitting, клонировании, fusion и HIR→MIR lowering. Эта версия не переносит ID через преобразования автоматически.
- Полный dependency/typed-IR/semantic/target fingerprint. SHA исходного unit — лишь первая часть ключа; профиль нельзя переиспользовать только по site ID.
- Collector и определения событий: число iterations, trips, hits/misses guards; нулевые loops тоже должны учитываться.
- Refusing loader и cost policy для size/T-states. Профиль влияет на profitability; legality продолжает проверяться независимо.

Исходный SHA намеренно консервативен: любое изменение unit меняет его версию. Стабильные declaration IDs и declaration-level invalidation пока отсутствуют. API карты не является security boundary и не разрешает оптимизацию на основании пользовательских origins.

## Продолжение

Первый collector input-HIR наблюдений реализован отдельным opt-in API, без переноса через Grace или изменения production pipeline: [события, пример и ограничения](grace-pgo-collector.md).
