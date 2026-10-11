# Константный u32 SHL вне HL

Base `d0fe5ab0`. Исправлен source-backed native miscompile в `genShift32`: `fun shift(a:u32)->u32{return a<<1}` с допустимым DWord-размещением source/result в DE возвращал `0x0000fffe` для `0xffff` вместо `0x0001fffe`. Прежний ASM и ошибка заморожены в `wide-shift-placement-before.txt`.

Важное ограничение утверждения: проверены неизменённый Nanz-источник и final MIR, но физические homes для этого воспроизведения заданы явно. Четыре ordinary source формы в `wide-shift-placement-probe.txt` автоматически распределяли u32 в память, а `genWideSpills` размещал результат в HL, скрывая сбой. Ошибка не выдаётся за падение automatic default на этих четырёх примерах.

Прежний путь DE/BC копировал через HL только младшие 16 бит, сдвигал не тот теневой HL и не возвращал старшие биты. Теперь non-HL использует SLA/RL по байтам обоих банков. Путь HL сохраняет native ADD/ADC. Независимый HL больше не используется как scratch; A и прочие пары не меняются. Флаги сдвига не обещаются сохранёнными.

Проверено:

- Source/final MIR, valid HL/DE/BC homes, 7 boundary inputs: 21 native вызов с ABI-return и SP, независимый uint32 и final VM oracle. Исходник/ASM/BIN + SHA256 — `artifacts/wide-shift-placement/manifest.json`.
- 1 470 forced native случаев, 5 pair layouts, counts 0/1/7/16/31/32. Проверяются результат, все HL/DE/BC в обоих банках, A и stack balance. Источник в отличной от destination паре сохраняется.
- Focused pipeline PASS 0.029 s, MIR2 PASS 3.374 s. Полный MIR2 PASS 25.915 s (`wide-shift-full-mir2.txt`).

Это исправление допустимых физических раскладок, а не новое allocation-table admission или доказательство оптимальности. Полный pipeline и SHA256 workload здесь не заявлены. Следующий этап — независимый original-source аудит `signed_arithmetic.nanz`.
