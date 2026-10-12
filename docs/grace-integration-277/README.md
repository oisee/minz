# ASM inputs: исправленный HIR и native runtime повтор

Имена входных значений разрешаются до legacy физических aliases. Старый `(in x,y)` становился [x,x], потому что неизвестное registername→class0 совпадало с первым unannotatedparam. Теперь value names сохраняются; явные annotated A/C aliases работают; unknown input отвергается HIR verifier.

Root names/aliases/refusal PASS0,008с. После интеграции оба полных Tetris runtime сценария и SQLite native entry API PASS2,826с. SourceTetris/SQLite и независимыеmodels не менялись. Metadata counterexample иSHA сохранены в reports/asm-input-binding; это исправление use metadata, не заявленный новый nativevalue miscompile.

PublicCPM admission ещё требует bounded динамических global stores и exactcanonical TUIASM proof. Этот guard prototype остаётся отдельной работой; обычная native16bitRAM семантика не меняется.
