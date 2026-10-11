# Исходный self_tokenizer: полный main и native stdout

База `9bdb26c7`. **Исходник `examples/nanz/self_tokenizer.nanz` не изменён.** Теперь проверяется весь main: init_keywords, явная загрузка source bytes, tokenize_all, print_all_tokens, счётчики и завершение. Прежние 30 helper workloads остаются отдельным доказательством.

Аудит по Language Book v7 §18.1 не нашёл CString ошибки: input этого main записывается явными poke, а строки @print обрабатываются через специфицированный формат SString. Исполнение не требует изменения source/HIR/body. Никакой checked derivative не используется.

## Независимый оракул и фактический результат

Go-модель лексера/interner отдельно сканирует исходный fixture `fun add(a: u8, b: u8) -> u8 { return a + b }`, выделяет keyword strings и строит токены с pointer sharing. Независимый renderer сопоставляет модельные token types/keyword pointers с printable names; он не вызывает tok_name и не копирует native/VM вывод.

После whole-main проверены все **8192 string arena bytes, 4096 token arena bytes, 4096 source bytes и 47 globals**, включая незаписанные хвосты. Проверены соседние memory guards, stack canaries, SP и все immutable image bytes. Только именованные globals, явные wrapper fields и exact compiler spill extents могут меняться внутри assembled image.

Результат — **19 токенов, 91 intern byte, 146 stdout bytes**:

```text
Source: fun add(a: u8, b: u8) -> u8 { return a + b }
Tokens: 19
fun id ( id : id , id : id ) -> id { return id + id } 
Interned strings: 91 bytes
```

Пробел после последнего токена и LF входят в точное сравнение bytes.

| Режим | T-states | Размер | SHA256 BIN |
|---|---:|---:|---|
| Обычный native порт 0x23 | 199 102 | 5466 B | `cb7a8da6dd152d5bc97f99d05a7f33c41b53bda6c8accb56b1f7f1144c6f5545` |
| Явный CP/M BDOS2 runtime adapter | 226 357 | 5554 B | `23e4ce4a5546dfe970326cb5e2a1533d9db037aae399aa3dfd613897a033b388` |

Default native image совпадает с прежним original-tokenizer helper image: проверена другая точка входа этого exact binary, не объявлена новая программа по новым helpers.

## Что означает CP/M здесь

У неизменённого source реальные extern peek/poke: test runtime задаёт native `LD A,(HL);RET` и `LD (HL),C;RET`, проверяя actual ABI. Default compiler print — OUT0x23; основной native режим исполняет его и снимает все байты stdout.

CP/M режим использует **явный test-only assembly runtime adapter** той же сборки: generated OUT0x23 заменяется вызовом сохраняющего регистры BDOS2 putchar (CALL5), а terminal DI/HALT — RET в process wrapper. Все алгоритмы, branches, main/printing functions сохраняются. Wrapper загружает SP0xeff0, вызывает main, пишет status1, восстанавливает incomingSP и возвращается в warmboot PC0/SP0xfe00. Emulator BDOS handler реально получает 146 байтов; намеренное изменение A/HL на BDOS return проверяет сохранение live registers.

Это adapter-derived CP/M executable из exact source, **не production compiler CP/M target flag, не реальное железо и не все входы токенизатора**. Go host BDOS работа исключена из T-states. Fixed original main имеет один вход. Прежние bounded helper30inputs расширяют evidence этого же source отдельно; source vocabulary ограничен десятью classified keywords, без полного языкового lexer/OOM обещания.

## Проверки и сохранение

Повторно прошли `TestOriginalTokenizerMainNativeFullState` (оба режима), `TestOriginalTokenizerFullStateVMZ80` (30 независимых входов) и оба SelfLanz whole-main режима: PASS 3.142 s. Каталог сохраняет все 193 пути и 23 bounded-covered paths; для этого exact tokenizer добавлена **whole-main evidence**, число программ не увеличено второй сборкой.

Из `minzc/`, Go1.26.8:

```sh
MINZ_ORIGINAL_TOKENIZER_MAIN_ARTIFACTS=/absolute/evidence go test ./pkg/pipeline -run '^TestOriginalTokenizerMainNativeFullState$|^TestOriginalTokenizerFullStateVMZ80$|^TestOriginalSelfLanzMainNativeFullState$' -count=1 -v
MINZ_SHOWCASE_ORACLE_CATALOG=/absolute/catalog.json go test ./pkg/nanz -run '^TestShowcaseExecutableOracleCatalog$' -count=1 -v
```

В `port23/` и `cpm-bdos2/` сохранены exact source, ASM/BIN, stdout, full captured state и independent oracle. JSON gzip без потери. `initial.txt` — первый успешный запуск; miscompile не найдено, source fix не требовался. `manifest.json` содержит SHA256 каждого payload, кроме себя. Полный slow pipeline этим этапом не запускался.
