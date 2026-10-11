# Независимое ревью caller-owned struct ABI

Проверено 11 октября 2026. Это read-only ревью изолированного рабочего дерева `struct-return-lifetime`, HEAD `aab62291` после `475298f7` и `6c032fe9`, с незакоммиченными изменениями владельца. Снимки SHA проверенных файлов находятся в manifest.json. Здесь нет production-изменений и утверждения о завершении всего ABI.

## Публичный native-вход

Независимый тест через Go overlay вызывает настоящий private `buildAssertBootstrap`, обычный compiler pipeline и native assembler/emulator. Исходная функция принимает u8 и возвращает объект `{a:u8,b:u16}` с полями `{x,u16(x)+256}`; ветвь `x==0` сохраняется. Ordinary, Grace Speed и Grace Size: пять входов 0,1,127,128,255 в каждом режиме, всего 15 вызовов, PASS 0.037 s.

Проверены три фактических байта результата `{x,x,1}` по адресу fefd, HL=fefd, восстановленный SP=fefd, защитный байт ff00 и неизменность исходного списка Assert.Args. Bootstrap резервирует три байта ниже ff00 до подготовки аргументов. Это ограниченная проверка объекта и стека, не проверка всех 64 KiB RAM или всей пользовательской assert-схемы. Обычный scalar Expected для struct проверяет адрес результата; ожидания полей пользователю пока нужны через явную функцию-обёртку либо будущую объектную assert-схему.

## Косвенный runtime-контракт

В текущем WIP `struct_return.go:243` добавлен `ValidateIndirectTargetABI`; VM `vm.go:912` вызывает его до входа в разрешённую функцию. Проверяются количество и типы физических параметров/результатов и точное описание буферов. VM теперь записывает ExtraRets. Пробник подтвердил отказ другого типа параметра.

Обнаружен пропуск: одинаковые Params/Returns при `target.Attrs.FallibleCY=true` проходят проверку обычного typed callable. Пробник возвращает nil. CY/A error ABI должен быть явно совместим либо такой target должен отвергаться. Это не новая языковая политика: текущий HIR consumed-callback verifier уже отвергает fallible callback без CY-сигнатуры. Владелец ABI получил точное воспроизведение и минимальную рекомендацию до интеграции.

Native `z80codegen_call.go:162` всё ещё синтезирует indirect callee с одним результатом из inst.Ty; владелец отдельно исправляет multi-return ABI. Нельзя считать VM-проверку достаточным доказательством native multi-return.

## Копирование и идентичность

Production `inline.go:89` исключает caller-buffer функции; `isTrivialFunc` исключает прямые и косвенные вызовы. Поэтому shallow `remapInst` не копирует новые call-bearing metadata в текущем пути. Bounded unroll `grace_runner.go:1490` ограничен CMP/SUB-петлёй; pointer-hoist и byte-fusion копируют адресные/памятные инструкции. `contracts_pbqp.go:511` создаёт только временный аналитический вид функции, не независимо мутируемый исполняемый clone.

Research VIR `inliner.go:255` копирует Inst поверхностно и Args отдельно; IndirectParamTys/IndirectReturnTys/StructReturnBuffers остаются общими, ExtraRets не перенумеровывается. Если VIR снова получит call-bearing production authority, эти поля нужно явно клонировать, а ExtraRets перенумеровывать. Сейчас checked-local VIR не принимает calls; этот исследовательский gap не обоснование задерживать production ABI.

Measured-mode identity в `profile_modes.go:179,229` и contract strategies использует полный JSON MIR: экспортированные новые поля включены. HIR fingerprint отражает семантический граф. В текстовом dump `dump.go:233` условие только `len(IndirectParamTys)>0` скрывает return metadata zero-argument callable: пробник показывает отсутствие `[indirect-abi]`. Нужно печатать при непустых params **или** returns. JSON-привязка остаётся полной; это дефект читаемого представления, не обход identity.

## Воспроизведение

`public-buffer-overlay_test.go` — отдельный pipeline test, подключаемый `go test -overlay` к несуществующему имени файла внутри pkg/pipeline; он не меняет файлы владельца. `indirect-abi-probe.go` запускается через `go run` из проверяемого minzc. Пробник намеренно проверяет helper и dump, не утверждает, что его демонстрационный модуль прошёл Verify.

Терминальные логи сохранены без перезаписи. Последующие исправления владельца должны получить собственные negative runtime CY/layout tests и native multi-return/full-state proof; данный пакет фиксирует проверенный промежуточный снимок.
