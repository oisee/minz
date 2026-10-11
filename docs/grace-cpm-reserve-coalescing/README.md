# CP/M guarded stack: bounded reserve coalescing

Защищённый runtime сохраняет корректность полного исходного selfLanz и tokenizer, но выполняет меньше проверок. Сравнение использует один source, input, публичный CLI и `--runtime cpm --runtime-stack 1024 --asserts none` на обеих сторонах.

| Программа | До, T | После, T | До, COM bytes | После, COM bytes |
|---|---:|---:|---:|---:|
| selfLanz original main | 983310 | 415614 | 4727 | 3968 |
| tokenizer original main | 869959 | 408919 | 8285 | 7397 |

Это −57.7% и −53.0% тактов; уменьшение на 759 и 888 байт. Static256 tokenizer остаётся 226178T/5815B без изменений; приведённый выигрыш относится к guarded1024, не к сравнению разных режимов. У baseline tokenizer compiler production точный 85773cf6, добавлен лишь тот же test oracle для guarded1024. Parser baseline ранее сохранённый frozen guarded COM.

После успешной проверки helper гарантирует 32 свободных байта ниже своего временного стека. Проверка использует шесть временных байт. Каждый PUSH/CALL требует минимум восемь оставшихся байт; PUSH расходует два, POP возвращает два. Поэтому перед новым checker сохраняются как минимум его шесть временных байт. Transformer объединяет проверки только в прямолинейных участках итогового ASM, сбрасывая резерв на labels/branches и CALL. Никакой callee не признаётся безопасным по имени; каждый call body проверяется существующим verifier, recursive callee начинает с reserve0 и выполняет собственную проверку.

Независимый verifier прослеживает reserve вдоль настоящих CFG рёбер: PUSH расходует, POP восстанавливает; все входящие состояния проверяются, включая путь без reserve. Branch bypass, unsafe join, loop с повторным расходом и четырнадцатый PUSH без новой проверки отклоняются. Точное тело checker и фактическое значение stack-limit проверяются; подмена EQU не может дать ложный reserve. Проверка отдельных emitted states остаётся консервативной, не делает заявления об оптимальном размещении guards.

Minimum успешного guard entry теперь 38 байт (6 временных +32 reserve), раньше 14. На малом стеке возможен более ранний controlled overflow; это допустимый и явно наблюдаемый отказ, status1, восстановленный incomingSP/OS return, result не опубликован. Matrix проверяет пороги 6,8,10,12,14,16,32,36,38,40,64, все байты ниже stack floor, сохранение regs/flags на успехе и все64KiB после overflow. BDOS internal stack потребности остаются отдельным условием ОС.

Actual saved public CLI COM совпадают с API побайтно. Независимые original parser/tokenizer host oracles проверяют полный stdout, все globals/arenas, весь64KiB allowed-write contract, code/literals и SP/guard/status. Recursive shared spill regression остаётся PASS. Полный MIR2 PASS11.134s; focused/runtime/admission tests PASS. Actual CP/M OS пока не проверена, native evidence использует BDOS2 hook.

```sh
mz examples/nanz/self_lanz_parser.nanz -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o parser.com
mz examples/nanz/self_tokenizer.nanz -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o tokenizer.com
```

ASM/COM обеих сторон, stdout/state/oracles, overflow capture и logs лежат в artifacts; compilers.json связывает frozen compiler. Нулевой runtime-stack/default port23 и measured profile contexts не изменяются.
