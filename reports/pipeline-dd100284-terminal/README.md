# Retained pipeline dd100284: terminal FAIL

Исходный immutable executable завершился с exit2 после примерно111минут. Процесс2837750 отсутствует; session79744 вернул terminal exit2. Наблюдательный timeout не был причиной остановки и прогон не перезапускался.

Пять top-level FAIL: три parser fixed-reference drift, TestOriginalTetrisCPMMainSourceAPIRefusal и TestOriginalZXAppsNative. Последний завершился panic на прежнем font oracle. Эти типы отказов уже отдельно воспроизведены и исправлены: parser470a884f, Tetris c443e1de, ZX80115d4f/da87dccc. См. parser-coalesced-reference, tetris-runtime-admission-transition и pipeline-94a41120-terminal. Полный retained FAIL не заменён задним числом на PASS.

Свежий pipeline350149ad продолжает работу на своём checkpoint и включает эти исправления. Он ещё не terminal; новый byte compare fix19b8880f туда не входит и имеет отдельные MIR2/forEach/heldout PASS.
