# Интеграция144: полный core после aggregate и bool widening

Checkpoint4587e753: **HIR PASS28,837 с; MIR2 PASS65,131 с; Nanz PASS15,893 с; VIR PASS2,347 с**. [Точный terminalлог](core.txt). Включены aggregate address/Alloca/spilledpointer corrections, signed/unsigned/boolwide widening, widestunsignedtypefix, fullpromotedoracle и исправление CLI measuredcostassertion. Точные исполняемые binaries с hashes сохранены в compiler-checkpoints. Catalogreview5f70a9fc появился позднее, не меняетproductioncode.

CLI4587e753 остаётся live отдельным процессом; current fullCLI PASS не заявляется. Старый fullpipelinea7954042 также жив, не перезапускается по elapsed. Новый replay прежних frozen parser/HashMap/PFCCO/Pascalheldout выполняется отдельно, это сохранённые prospectiveinputs, не новыйblindfreeze. Originalwidemath oraclepacket готовится в isolatedbranch после frozenbeforefailures; rootwholemodulePASS пока не заявлен.
