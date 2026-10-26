# Итерация 1: native Crystal frontend, специализации и ошибки

Целевая модель уточнена: **без GC**, с arena и мономорфизацией, объединением
одинаковых тел и ошибками через существующие outcomes/Nanz CY ABI.
Такой вариант заметно ближе к имеющемуся HIR, чем перенос host runtime.

Работа находится в `spike/crystal-hir`, worktree `/tmp/minz-crystal-hir`.
Исходный compiler checkout — `f7b5d9e5d`, Crystal 1.20.1 / LLVM 22.1.4.
Ни HIR, ни MIR2, ни backend в этой итерации не менялись.

## Что заработало

```text
Crystal::Compiler(no_codegen = true)
  → normalize + semantic + cleanup
  → reachable Call.target_defs (готовые специализации)
  → typed Lanz + merge одинаковых scalar-тел
  → обычный HIR → MIR2 → Z80
```

Из нативного frontend действительно переиспользованы parser, типизация,
разрешение overloads, мономорфизация generic-функций и раскрытие macro.
Нового вывода типов для Crystal мы не пишем. Например:

```crystal
def twice(x)
  x &+ x
end
```

Из этого исходника получены две разные нативные специализации,
`twice(UInt8) -> UInt8` и `twice(UInt16) -> UInt16`.
Отдельные `flavor(UInt8)` и `flavor(UInt16)` разрешены самим Crystal.
Macro `bias` превратилась в `7_u8` до HIR.

Всего native frontend отдал **19 reachable specializations**. Небольшой
локальный merge объединил одинаковые `identity(UInt8)` и
`identity_twin(UInt8)`: в HIR осталось **18 функций**. В этой итерации merge
сравнивает типизированные сигнатуры и тела с теми же именами параметров;
это консервативное точное сравнение, не общий поиск эквивалентных функций.
Адреса функций и external linkage в принятом подмножестве отсутствуют.

## Примеры

Все примеры — в [showcase.cr](showcase.cr), и именно они были скомпилированы
и исполнены проверками, а не только использованы для иллюстрации.

| Пример | Результат |
|---|---|
| Triangle wave | Все 256 фаз; `0, 128, 254, 254, 0` на фазах `0, 64, 127, 128, 255` |
| RGB888 → RGB332 | `(255, 128, 64)` → `0xF1` |
| CRC-8/ATM | Все 256 входных байтов при начальном CRC=0; цепочка `123456789` → `0xF4` |
| Hex parser | `AF` → `175`, `FF` → `255`; ошибочный символ → rescue-маркер `238` |
| Arena reservation | Резервирование 16+16 в 32 байтах проходит; 32+1 отклоняется; следующий frame начинает с нуля |

Arena-пример проверяет **учёт cursor/capacity**, исчерпание, границы и новый
frame. Он не выдаёт реальные указатели и не доказывает layout/memory safety
объектной arena. Перенос struct/pointer layout и фактические load/store —
следующая самостоятельная проверка.

## Как понижены исключения

`raise "статическое сообщение"` получает маленький код ошибки. Для функции,
которая может выпустить исключение, scalar UInt8 результат временно
упаковывается в UInt16:

- success: `0x00VV`;
- failure: `0x01EE`.

Адаптер вычисляет escaping effects до неподвижной точки по resolved call graph.
Вложенные выражения с fallible calls раскрываются в последовательность
временных переменных и немедленных проверок; ошибка первого вызова не позволяет
выполнить следующий. Например, ошибочный первый hex-символ распространяется
до обработки второго. Bare `rescue` в хвосте функции превращается в локальную
ветку с fallback. Нет исключительных объектов, GC, unwinder или runtime table.

**Текущий HIR `@check` не является catch:** и он, и `@propagate` сейчас
распространяют ошибку. Поэтому rescue в этом эксперименте сначала выражен
через обычные typed values/ветки HIR. Никаких неверных предположений о
существующем `@check { handler }` в адаптере нет.

`crystal-hir-outcomes` затем строит тонкие HIR-адаптеры, переводящие tagged
outcome в имеющиеся `@error` и `@propagate`. Это проверенный путь к **настоящему
Nanz CY ABI**, с двумя слоями propagation. Прямой экспорт fallible defs в CY
и отдельный HIR-узел получения outcome могут впоследствии убрать промежуточную
упаковку. Такой прямой exporter пока не реализован.

На Z80 проверены и success, и failure, возвращаемый A, Carry и SP. Перед
вызовом тест намеренно выставляет Carry, чтобы success действительно должен
был его очистить. Размер propagation не объявляется «одним RET C»: текущий
backend также сохраняет/восстанавливает outcome и регистры.

## Проверки и воспроизведение

Из корня worktree, локально:

```sh
python3 research/crystal-hir/verify_typed.py
python3 research/crystal-hir/verify.py
```

Нужны Crystal с исходниками compiler, Go, Python 3 и LLVM development libraries
для сборки native semantic exporter. На проверенном Mac используется Homebrew
LLVM 22. Небольшая библиотека `native_frontend.cr` намеренно не включает CLI:
общий `compiler/requires` тянет docs/shards (`markd`), которые здесь не нужны.
Для встроенного compiler необходимо передать CRYSTAL_PATH; runner получает
его через `crystal env CRYSTAL_PATH`.

Результаты итерации 1:

- **844 одинаковых случая** на native Crystal, raw MIR2, optimized MIR2, Z80.
- **23 случая CY ABI** на raw MIR2, optimized MIR2, Z80; A, CY и SP проверены.
- **5 отрицательных контролей**: checked `+`, нативная ошибка типов, ensure,
  bound rescue и union result.
- Исходная syntax-only итерация сохраняет свои 26 случаев и 11 отказов.

Логи, manifest с mangled symbols/alias/error codes, HIR, MIR2, Z80 asm,
нативный проверочный source и receipts пишутся в `/tmp/minz-crystal-typed-results`.
`results.json` содержит SHA256 showcase и exporter. Копии receipts и manifest
сохранены рядом в `evidence/`.

На одном прогретом запуске native semantic export занял 1.56 s,
HIR/MIR2/Z80 вместе со всеми 844 проверками — 2.80 s. Это наблюдение для этого
корпуса и машины, не оценка производительности больших программ.

## Границы итерации

Экспортируются библиотечные kernel-функции. Нижние top-level вызовы в showcase
служат **корнями inference**, не превращаются в target `main` и не исполняются
exporter. Использование arbitrary top-level программы ещё не поддержано.

Приняты scalar UInt8/UInt16/Int8/Int16/Bool, wrapping arithmetic, bitwise,
сравнения и unsigned shifts с literal count меньше bit width. Ограничения
нулевой итерации по locals/branch merging остаются. Instance receivers,
struct layout, union dispatch, closures, динамические строки и stdlib calls
ещё не импортируются. Host stdlib участвует в native semantic, но не линкуется
в Z80-продукт.

Исключения пока ограничены статическим сообщением raise, UInt8 success и
единственным bare rescue в хвосте функции. Typed/bound rescue, exception
payloads, ensure, retry и nested rescue отклоняются. Fallible loop condition
также требует отдельного ANF lowering и пока отклоняется.

Следующий содержательный шаг — struct/Pointer с нашим target layout и arena,
затем прямое CY lowering с явным catch outcome. GC для этого не нужен.
