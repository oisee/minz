# Saved actual policy CLI v2: final independent audit

Все пять owner-run завершились 0. Reviewer не запускал Z80 повторно: проверены frozen saved payloads `memory-host-policy-cli-evidence` и preserved `minzc-policy-cli` SHA **795f69cea3ed2cc2301a0a70aeabd29c70c3f0065273597e59cbdc58fc712c72**. Source SHA 0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526 и три input inventory записи независимо совпали.

| actual CLI run | policy | actual winner | TStates | Bytes |
|---|---:|---:|---:|---:|
| full30 speed | preserve-speed | both, mask3 | 1076715 | 2100 |
| full30 size | preserve-speed | append, mask2 | 1076874 | 1979 |
| small collect | preserve-speed | arena, mask1 | 566 | 2122 |
| small supplied replay / external -o | preserve-speed | arena, mask1 | 566 | 2122 |
| small Legacy v2 control | legacy | arena, mask1 | 583 | 2121 |

Две actual full30 CLI цели использовали одинаковые четыре native candidates; это реальные Speed/Size вызовы, не derived-size заявление. Full30 candidate costs 1132449/1132290/1076874/1076715T и 2001/2122/1979/2100B. Все четыре native images, ASM, MIR/allocation/sidecar/frame hashes, per-call costs/steps/state точно совпали с ранее сохранённым policy1 API evidence; executable context здесь новый795f, и проверен самостоятельно.

Проверены 20 mandatory baseline-v2 envelopes, единая declared policy в каждой selection/association/candidate, точные aggregate precompile choices/compatibilities/full-function maps/source sites и 11 source loop-count sums. Разрешённые lineage shifts только arena s1→s4 и append s5→s7. ABI unchanged, whole source-unit hash bound; precompile proof не заявляет сохранение формы после compiler preparation.

Независимая исходная host-модель сверила все 264 candidate invocations (240 full +24 short), все globals, region snapshots и ordered resource events; full30 каждый candidate имеет 3017 events. Python native audit сверил реальные instruction bytes с embedded binary, exact global symbol/offset memory roots и конечные A-byte-only snapshot frames (0/1/0/1). Go typed auditor отдельно проверил JSON serialization hashes sidecar/frame, allocation compactbytes, MIR, ASM/BIN и cost sums для всех 20 payloads. No permission/ISA extension is inferred.

Decision SHA сверены по именно сохранённым selection-v2.json и association-v2.json bytes; selected ASM/BIN точно соответствуют declared winner. Submitted small association и replay selection byte-identical, external-selected.a80 совпал с replay selected.a80. Strict primary metric ties retain first mask. Legacy small context/policy0 явно отдельный; никакой bare-v1 association conversion не заявлена.

Все скрипты/логи/summary/raw inventory сохранены; derived nested v1-shaped diagnostics losslessly gzipped mtime0 и не являются самостоятельными Legacy attestations. Mandatory outer v2 policy остаётся частью evidence. No CPU replay; no compiler/source edits. Remaining public production root build/gates/publication относятся к parent stage.
