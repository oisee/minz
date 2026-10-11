# PFCCO PBQP теперь доступен в обычном CLI

Интеграция 2cfbd54b + b99cbf63 добавляет --contract-strategy pbqp до LUT/preallocation в обоих HIR API. Default остаётся greedy; unknown/disabled/ez80 requests отклоняются. Стратегия входит в compilation ID, поэтому профиль другой стратегии отказывает до изменения HIR. Нынешние четыре measured mode/replay комбинации сохраняют свой прежний выбор; explicit strategy с ними несовместима.

Полный Nanz с pinned ASM ABI C и условным вторым вызовом: greedy 87387 T / 63 B → PBQP 85343 T / 61 B на всех 256 inputs, независимые VM/native арифметика и persistentstate/SP. Weighted packet проходит обычный pipeline: off 8857 T, greedy/PBQP 7888 T при 52 B. Это измеренная польза на конкретных workloads, не универсальная оптимальность. Пакет из 38 файлов со source/ASM/BIN/allocation/nativeevidence скопирован SHA256-проверенной копией на шару.

После emulator RST fix повторные реальные parser/tokenizer/mapInPlace/profile/heldout проверки PASS 15,623 с. Интегрированные ContractStrategy/pinnednative/packet/replay admission группы тоже PASS. Полный CLI-c784-full и pipeline-cc502-full ещё выполняются; оба более ранние source checkpoints и не доказательство полного текущего PASS.
