# Literal call cycle rematerialization

База: `08e3ae7f`. Исходник и две новые training / четыре heldout нагрузки заморожены в `5897e1de` до изменения генератора. Production commits: `f1b1e289`, `a3a0e4b3`, `fb5771c2`. Изменён только `z80codegen_call.go`; pipeline, контракты и interference не менялись.

Прямой CALL получает фактическое место параметра вызываемой функции. В замкнутом цикле перестановки байтов генератор заменяет перенос константы непосредственной загрузкой после остальных переносов. Допуск требует единственный OpConst того же целочисленного типа в текущем блоке перед этим CALL, отсутствие алиаса параметра и отсутствие последующих использований по CFG. Indirect calls, flags, spills и широкие циклы исключены. Исходная загрузка константы остаётся и входит в стоимость. Дешёвый обмен слов через EX DE,HL сохраняется. Сохранение регистров, флагов и проверки стека выполняются прежними механизмами.

Первый вариант заменял все мёртвые литералы. Семантические проверки прошли, но greedy вырос с 5465 до 5477 байт, а init — с 15790 до 15841 T: непосредственная загрузка иногда дороже переноса регистра. Это отрицательное доказательство сохранено в `uncosted-negative`. Итоговый вариант ограничен циклами и оставляет greedy ASM/BIN без изменений.

| Неизменённый tokenizer, PBQP | До | После |
|---|---:|---:|
| Обычный BIN | 5593 B | **5533 B** |
| BIN с явно указанными native adapters, ORG 0400 | 5597 B | **5537 B** |
| init_keywords | 16868 T | **16273 T** |
| Историческая последовательность двух входов | 327286 T | **326691 T** |
| Новая frozen training последовательность | 412898 T | **412303 T** |

Независимые оракулы lexer/interner, состояния памяти, вывода, кода и SP проходят. Scan/render не изменились. Greedy остаётся 5465 B / 15790 init T; новое training — 411820 T. **PBQP всё ещё проигрывает greedy по обоим критериям.** Это полезное улучшение production PBQP output, но не новая победа селектора и не завершение цели PFCCO.

Speed и Size детерминированно выбирают greedy на новом training. Его точные сохранённые ASM/BIN проходят четыре heldout входа без нового поиска: 885117 T / 5465 B. Независимый audit сравнивает training и heldout BIN побайтно. Это prospective проверка выбора стратегии в test-owned native harness всей программы, а не public general PGO API. Исходник не менялся; runtime Z3 и production profile hook не добавлялись.

Native тест проверяет байтовый цикл и сохранение word exchange, live/nonlive случаи: 256 литералов × 256 других значений × 4 = 262144 реальных CALL/RET. Проверяются регистры, все 256 значений F, IX/IY и SP. Для слов второе значение равно x*257; это **не весь домен 65536 word literals**. RET callee проверяет перенос аргументов на границе вызова; реальные тела функций проверяются tokenizer. Дополнительно проходят 256 mixed byte/word/half/identity случаев, шесть отрицательных контролей определения константы и существующие pair-half тесты обоих production entrypoints.

Финальные scoped native проверки: PASS, 0.229 s. Полный MIR2: PASS, 87.127 s на `f1b1e289`. После ограничения word cycles и проверки присутствия exact CALL полный suite не повторялся; финальный executable `fb5771c2` прошёл focused native, tokenizer, prospective, runtime и packet проверки. Guarded original SelfLanz: 135 stdout bytes, 419878 T / 3987 B. Guarded tokenizer: 146 bytes, 411987 T / 7391 B. Полные state/SP/stack guards проходят. Stateful packet остаётся 8857 → 7888 T / 52 B. Проверка физического CP/M hardware не заявляется.

Воспроизведение: frozen executable указан с SHA256 в `build.json`. Из каталога `minzc/pkg/pipeline` запустить:

```sh
/path/to/pipeline.test -test.v -test.run '^(TestCallLiteralTokenizerProspective|TestPFCCOOriginalTokenizerRenderingConsumer|TestCPMRuntimeGuardedTokenizerMain|TestCPMRuntimeOriginalSelfLanzMain|TestPFCCOMeasuredNativePacket)$'
```

`MINZ_CALL_LITERAL_TOKENIZER_EXPORT` задаёт новый абсолютный каталог prospective результатов; `PFCCO_TOKENIZER_EXPORT` — новый абсолютный каталог исторических результатов. Исходники, options, MIR, allocation, ASM/BIN, наблюдения и отрицательный вариант сохранены. Проверка SHA и независимых сравнений:

```sh
python3 reports/call-literal-rematerialization/audit.py
```

Полная копия: `/mnt/safe/minz/grace/2026-10-12/call-literal-rematerialization/`; frozen executable — `pipeline.test` в этой копии.
