# Saved actual native review — direct DE/BC global words

Независимый audit завершён без VM/CPU replay. Owner actual28151 terminal PASS2.960, экспорт сохранён в permanent global-word-load-evidence/actual. Frozen source1f85, production изменения только matcher/emitter, runtime permissions не меняются.

| Source HIR candidate | Prior measured T/B | New actual T/B | Reduction |
|---|---:|---:|---:|
| original | 1221494 / 2091 | 1135370 / 2000 | 86124T (7.0507%), 91B |
| arena+append | 1147025 / 2197 | 1079636 / 2099 | 67389T (5.8751%), 98B |

Новое original выполняет109829 инструкций, both104388. Суммы30 actual per-call T и Steps совпадают с totals. Это реальный whole-program результат владельца; первоначальная local estimate46608T не была полным измерением и не подменяет таблицу.

BIN original1f0f5959668544e6d0cf1bdae7462b61f3927febcef6f3e9d74dce69cad74941; both4bdce86bd7a724f34e7eee180e8a6530b07f33cadd39e7a1a93813df6c841c13. Embedded BIN/ASM равны saved files. Go typed JSON hash проверяет sidecar, call frames, allocation exact compact bytes, MIR diagnostic SHA. Python не пересериализует typed digest. Все73/74 rooted access records проверены против actual bytes и symbol offsets; ED4B/5B wordloads17/18 строго width2/read/root-global. Root-address и destination opcode связаны с теми же bytes. Existing typed low/high permission recipe исчерпывается actual ED инструкцией; focused recipe tests независимо проверяют неправильные address/width/direction. Нет generic memory permit.

Finite stack snapshot остаётся только прежним19instruction AF/sourceA→destinationC width8, frames0/1. Адаптер SHA прежний. Ни sidecar runtime, ни frame contract не расширены. Уменьшение числа sidecars90→73 и92→74 отражает слияние двух semantic byte loads в одну rooted word instruction; shape+binary/typed hashes проверены, это не исполняемые PC counts.

Независимый source-bound host oracle exact SHA0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526 проверил все30 persistent calls, результаты,10globals,3полныхregions и3017ordered resource events. Всё совпадает с raw, target и соответствующим прежним CLI target. Function site maps/compatibility без context совпадают с прежними respective candidates. Source HIR обеих функций в both отличается ровно прежними transforms; новый backend не меняет sourceproof.

CompilerID честно новый: executable-sha256:ea9b0791f3c9cf57280e3ba6a578c40b36482aec2c92361e5ffdbc925b2d23f7. Прежний CLI1bd и его данные не перелицованы. Этот эксперимент использует свежую baseline factory, не сериализованный старый loop profile. Export не содержит свежего loop-profile collection и executed per-PC global-read ledger, поэтому такие claims не делаются. Последующая production integration/общие gates принадлежат root, данный review их не объявляет завершёнными.

Repro: audit_saved_actual.py CURRENT_ROOT PRIOR_SPEED_ROOT; verify_host_oracle.py baseline.json --source source.nanz; Go audit_typed.go.txt через текущие pipeline types. Host oracle скопирован из independent costmodel corpus oracle; авторство и narrow source scope сохранены. Только serialization/host interpretation, никаких CPU invocations.
