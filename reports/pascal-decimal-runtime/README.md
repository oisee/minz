# Pascal signed decimal and Factorial production runtime

Заморозкаe51438a6, исходныйfactorial.pas неизменён. Before0b009c99: WriteI168000 печатал32768 вместо−32768,FFFF65535 вместо−1 в preserving и fullBDOSclobber host. Это generatedruntime/frontendlowering ошибка, не переопределение оракула. Exactbefore ASM/BIN/MIR/stdout retained.

Semanticfix5f95cb1f разделил signedInteger WriteI16 и unsignedWord WriteU16. Signedprefix сохраняетIX/HL перед выводомминуса, unsignedmod16 magnitude корректен для−32768. Pascal lowerWrite выбирает helper по actualTyI16/TyU16, typedInteger/Wordregression included. Shared completegeneratorbody не используется как decimaloracle: независимый fmt/int16/product модель.

Productiona4915bbc +private-labelbinding1eb745c4: полные ASMтела, physicalHL/input/type, AllClasses, helperdependencies и реальныеnamedmachineinstructionPCs проверяются. _wr16_dig private registerprotocol допускается только exact trustedASMcaller; publicMIRCALL отказан. Internalbranchlabels сверяются с фактическимиPC, изменённыйEQU/label/helper/code/clobber/sign/divisor/callhome отказаны. Никакого arbitraryASMtrust. Profiledeadcanonicalruntime policy не расширена.

**Полные домены:** signed65536patterns ×2BDOShost обычныйnativePASS1.777s. Production guarded image signed65536+unsigned65536×2hosts=262144nativeexecutions PASS31.617s, полный64KiBRAM/code outside namedprivate stack unchanged, status0/guardA5, exactstdout/SP. Unsignedhelper дополнен явно test-owned каноническим helper в отдельный formattertestmodule; исходный wholeFactorial проверен отдельно неизменным. Fullformatterdomain прогнанordinarypolicy, whole3modes images и exactbody authority checked; не утверждается независимый fullformatterdomain вкаждомmode. HelpersASMнеисполняютсяfakeVMhosts.

**Исходный Factorial whole main:** 3режима ×preserving/fullA/F/BC/DE/HL/IX/IYclobberBDOS2 PASS5040CRLF, ORG0100→OSRETPC0SPfe00, stackword/code/fullRAM guards. COM1403B,SHAc2526fe46ea531b7cdbb902815113677198053b578ec05de388f459c158e6765,3285CPU T безсинтетическойBDOSстоимости. CLI3режима сохраняетпобайтнототжеCOM. MainконстантноfoldsFACT7; recursiveFactсампроверенпредыдущимbounded0..12raw/finalVM/nativepacket, неmainrecursionclaim. РежимыsameBIN, noGracegainclaim.

7negativeMIRmutations +machinebody/EQU/call tamper controlsPASS. ExistingHello/README/parser/tokenizer+wholefocused1.463s; CPM/typed/CLIscoped packagesPASS, logs include incidental nativeprint artifactskip notwholecorpusclaim. Frozencompiler+testSHA pathsin frozen-executables.json. No fullpipeline rerun, no actualphysicalCPM.

Public CLI from repo root:
```
/mnt/safe/minz/grace/2026-10-12/pascal-decimal-runtime/executables/minzc examples/pascal/factorial.pas -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o factorial.com
```
Size add --grace --hir-grace --Osize; speed --grace --hir-grace. Frozenpipeline.test from minzc/pkg/pipeline `-test.run '^(TestPascalHelloProductionCPM|TestPascalDecimalAdapterRejectsMutations)$' -test.v -test.count=1`. The fullformattertest uses `-test.run '^TestPascalDecimalProductionFullDomain$'`; source unchanged and test-owned unsignedadd explicitly above.

Goalremaining elsewhere не объявлены закрытыми. ByteformatterWriteU8 и arbitrary ASM/callbackruntime сюда не относятся.
