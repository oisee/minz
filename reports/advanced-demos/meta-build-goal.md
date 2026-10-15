# Цель сборки доступна внутри Nanz-метафункций

15 октября 2026. Продолжение ветки `experiment/sprite-preparation-options`,
пока не main и не изменение зафиксированного релиза 0.25.3.

`meta_opt_size() -> bool` сообщает генератору, включён ли `--Osize`.
Вызов исполняется на compiler VM во время раскрытия `fun @name`;
он не добавляет функцию, runtime-флаг или ветвление в target-код.
Генератор сам выбирает, какую специализацию построить.

CLI передаёт одну цель и Nanz parser, и позднему pipeline. При использовании
Go API нужен `nanz.ParseOpts{OptSize:true}` **до** раскрытия метафункций.
`pipeline.Options{OptSize:true}` после разбора не меняет уже построенный HIR.
Контекст передаётся через Nanz imports, generated declarations и statement
invocations; default false сохраняет обычное поведение. Он хранится внутри
конкретного parser/invocation, не в глобальной переменной frontend.

```nanz
fun @make_value() -> void {
    if meta_opt_size() {
        emit(c"fun chosen()->u8 {return 9}")
    } else {
        emit(c"fun chosen()->u8 {return 7}")
    }
}
@make_value()
```

Это минимальный пример механизма, не пример ускорения. Практический стык —
[два варианта подготовки эффекта](sprite-preparation-options/README.md):
генератор получает политику для компактного цикла или развёрнутых вызовов.
Сам автоматический выбор этих вариантов по полному замеру ещё не подключён.
Hint не доказывает оптимальность произвольной специализации.

Проверено восемь последовательных разборов: локальные/импортированные
метафункции, два повторения Size/default, декларационные и вложенные
statement-раскрытия. Вложенное декларационное раскрытие проверяется с явным
import определения, как у текущих composed generators. Target MIR не
содержит `meta_opt_size`. Четыре компиляции через CLI-путь проходят независимые
MIR2/Z80 asserts и не оставляют intrinsic в ASM.

Полный `pkg/nanz` — PASS за 37,408 с. Три спрайтовые регрессии — PASS за
12,281 с; CLI Size-контекст/default output/flags — PASS за 0,019 с.
Протоколы сохранены локально как `meta-size-goal-*.log` в
`grace-reports/2026-10-15/`. Это не общий PASS всего компилятора.

Также проверен настоящий процесс `mz`: default и `--Osize` проходят
MIR2/Z80 asserts. Контрольный Size-запуск с неверным ожиданием отказал
и не записал ASM; это ожидаемый отрицательный тест механизма. Во всех
успешных ASM отсутствует `meta_opt_size`. Бинарник CLI, исходники, stdout,
stderr и SHA сохранены локально в `meta-size-goal-cli-real/`.

[Language Book, раздел 20.4](../../docs/Nanz_Language_Book_v7.md#204-vm-hosted-metafunctions)
теперь описывает текущий `emit` → Nanz → HIR и реальный host API,
а не прежний концептуальный пример Lanz-строк.

The reviewed showcase catalog reference to the mutable book is refreshed after this chapter20.4 edit. Chapter21.4 (Lizp last-form semantics), used by that evidence row, is byte-identical to `c3574329`; historical test manifests remain unchanged. The complete Nanz suite validates the refreshed reference.
