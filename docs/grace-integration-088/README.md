# Интеграция 088: адресный hoist FAT12

7719dffb расширяет MIR Grace pure-address hoist: PtrAdd(base,Add(offset,index)) превращается в PtrAdd(PtrAdd(base,offset),index) только для invariant base/offset,16-bit modular address и exclusive inner sum. Normal module-global AddrOf может быть вынесен как total address constant. Hard classes, wide/varying sum, unknownsymbol и calls/ASM/ports сохраняют отказ. Loads/stores не перемещаются.

Actual unchanged FAT12 write_dir_entry после separate liveAF correction: Grace Speed13828→11202T,9869→9885B. Default/GraceSize unchanged. Все18slot/mode случаев raw/finalVM иnative diskABI/512window/all64KiB/SP проходят. Это одна cache-hit procedure с explicitdiskadapters, не настоящаяCPMOS и не heldoutPGO. Native wrap/orderchecksum иsemantic fault-prefix/zero/alias/extra-use cases проходят.

Root integrated MIR2focused PASS0,017s; pipeline invariant+FAT+byte-load cases PASS31,999s. Agent fullMIR2PASS8,406s. Новыйrootcore5fe165dd прошёл HIR/MIR2/Nanz/VIR, CLI ещё выполняется; он предшествуетhoist. Fullpipeline3e143f9f живёт вsession71745 и предшествуетновымbackendchanges. ПолныеPASSтекущегоcheckpoint не заявляются.

67payload SHA проверены и copiedbyteverified вdatedsharecompiler-docs/grace-fat-address-artifacts. Source/ASM/BIN/finalMIR/allocation/result сохранены дляобеихсторон. Общаяцельактивна; начинается отдельныйаудит, какие actualfunctions не выигрывают от registershape tables и почему.
