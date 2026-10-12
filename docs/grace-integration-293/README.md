# Typed spilled word stores: root

Root57a7cc36 интегрирует отдельный backendfix698e84db. В OpStore16 классификация spill source теперь использует MIR RegInfo.Ty; u16/i16 старший байт читается из spill вместо записи нуля. Настоящий u8 по-прежнему zero-extends.

78 forced native store cases × word/byte widths и HL/DE/BC/IX/IY/spilledpointer проверяют boundaryvalues, fullRAM/code и возврат PC/SP. Joint extern ABI +store regressions PASS0,087с; все CPM CLI tests PASS0,056с. FullMIR57a7cc36 запущен отдельно, здесь terminalresult не заявлен. Originalcanvaswholemain/framebufferproof ещё собирает подагент.

Selfparsermissingfirstparam выявлен отдельно: OpCallCond Uses не учитывает Args, что позволяетscratchзатиратьаргумент. Исправление у другого агента, в этотcheckpoint не входит. Oldfullpipeline1f2365be остаётсяlive; старыеfailures не переписаны.
