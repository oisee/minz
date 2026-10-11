# Интеграция150: aggregate address classes и actual lifetime FAIL

Roote49054af исправляет aggregateaddress registerclass в HIRlowering: Struct/Array value16-bitaddress не получаетbytehome. **2304 nativeclasschecks rootPASS0,015 с** [лог](root.txt). Frozenoriginalbefore invalidLDindirection, classaftervalidASM сохранены вместе с source/ABI. Все58auditpayloads SHAverified/git-tracked; [report](../../reports/2026-10-11-082-aggregate-return-lifetime.md) и [план](../../reports/struct-return-lifetime/PLAN.md) наshare.

Послеclassfixvalidnative delayed(17,83)=83вместо17 иtwo_returns(17,83)=166вместо100, все30cases втрёхрежимах. Raw/finalVM сохраняютexpectedvalues. Callee возвращает адреслокальнойструктуры, освобождённойдоRET; laterCALL перезаписываетstorage. Leafinliningcontrol проходит, но не доказываетgeneralreturnlifetime. Callerownedhiddenbuffer ABI реализуетсяполноценно; scalarpromotion-onlyне заменяетего. Этот пакет фиксирует actualFAIL, не объявляет lifetimeкорректным.

OriginalLizpfunctional54independentcatalogpathproof не содержитcaptures; incompatibleknowncallback signature admission исправляется separately. SignedSrcTy nativeagreement149PASS. FullCLI/core4587e753PASS, currentaddressclass изменилсяпозже. Старыйpipelinea7954042live. Общая цель открыта.
