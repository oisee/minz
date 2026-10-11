# Fresh production CLI saved audit

Срез завершён: owner13807 четыре actual CLI runs terminal0; audit не запускает CPU. Executable dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5 сохранён minzc-global-word-load-production. Новые source-profile и все baseline/precompile context привязаны к этому exact exe; прежние CLI1bd и testea9b остаются отдельными историческими контекстами.

| Mask | Full30 actual T | Bytes | Rooted memory records | Frames |
|---|---:|---:|---:|---:|
| original0 | 1135370 | 2000 | 73 | 0 |
| arena1 | 1135211 | 2121 | 75 | 1 |
| append2 | 1079795 | 1978 | 72 | 0 |
| both3 | 1079636 | 2099 | 74 | 1 |

Speed выбирает3, size2. Это четыре реальные варианты на одном workload/context, повторены для обеих целей без изменения candidates. Новый direct-load backend сокращает стоимость всех; нельзя переносить прежние3099T выигрыша arena: здесь обе measured пары дают159T. Никакой универсальный winner вне этого workload не заявляется.

Все восемь full30 и восемь small2call native payloads прошли independent exact-source host oracle: results,10 globals,3полныхregions иorderedevents. Full30 содержит3017events у каждого варианта. Source SHA0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526. BIN/ASM/MIR saved файлы совпадают с embedded; Go typed JSON sidecar/frame hash, allocation compact bytes SHA и cost sums проверены отдельно, без Python typed reserialization. Symbol-bound ED4B/5B width2/read/address/opcode roots проверены у всех16. Finite AF→C snapshot remainsexact19instructions, no permission widening.

Typed precompile records всех16 проверены: schema/choices, original+actual compatibility/functionmaps, selected source pair иполный11sites map с изменением только arena s1→s4 иappend s5→s7, сохранённые origins. Profile count sums/percall observations exactraw, отдельная instrumentation semantics, CompletedCalls30/2 Failed0. Strict loaded source profile принадлежит текущему executable.

CLI decision selectionSHA хэширует exact pretty selection.json bytes. Selected BIN иASM побайтно равны candidate evidence; speed BIN4bdce86bd7a724f34e7eee180e8a6530b07f33cadd39e7a1a93813df6c841c13; size BIN1648f6983cac2efaef5b18ed84982920f71b99d68cc74f9dfb02a9595f644975. Smallcollect/replay costs604/583/663/642T, выбранаarena1 583T2121B; обеselection/profile/decision/nativeoutput полностью идентичны. Это actual same-executable supplied-profile replay. В данном срезе внешний-o не запускался; прежний его gate не называется свежим.

Independent audit tools: audit_output.py, audit_native.py (actualdynamicT/B/records), audit_proofs.py(allfourruns), all16-host-audit.txt, all16-typed-audit.txt, ed-root-audit.txt. Host oracle иtyped helper взяты из предыдущего independent review с сохранённым авторством. No CPU replay; defaultbudget/fullpackage gates принадлежат root и здесь не объявляются завершёнными.
