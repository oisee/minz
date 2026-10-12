# Original Pascal Hello: production CP/M runtime

База9e56cef9; исходники Hello/Factorial и независимое ожидаемое stdout заморожены826bb423 до запуска. Исторические отказы generatedASM сохранены0b5aa0ed. Production exactadapter70df304f, publicCLI .pas admissionf0fe0c27.

**Hello whole-main:** неизменный `examples/pascal/hello.pas`, точный `Hello from Pascal on Z80!\r\n`. Три режима ordinary/GraceSize/GraceSpeed × preserving/all-primary-register+flags-clobber BDOS hosts PASS. Настоящий COM ORG0100 → OS RET PC0000/SPfe00. Код/все64KiB неизменны вне именованных runtime/spill полей, incomingOSword сохранён; PUSH/CALL слова,private stack guardA5/status0 проверены. BDOS9 читает ровно исходный dollar-literal включая delimiter, границы image и ранний delimiter проверены; BDOS2 выводит CR/LF. COM1177B со linked1024stack, SHA8006154bf30c57df2681601ad582291eda465268cc7b21258c86dc6537756d15,841 CPU T без стоимости синтетическогоBDOS. Все3CLI binaries побайтно совпадают с checkedCOM. Gracegain не заявлен.

Новый production допуск проверяет **полные существующие** Pascal-generated WriteStr(u16→HL)/WriteCrLf(void) тела и AllClasses clobber, параметры/ASM inputs, точные инструкции и machinePC каждого доверенного CALL5. Без переписывания HIRruntime/исходника; dead-profile Pascal authority не расширялась. Никакой произвольный ASM/BDOS доступ не разрешён. Wrongfunction/input/register/clobber/extraCALL/CR mutations, moved/altered namedmachinebody и стороннийCALL5 отказаны. CLI разрешает .pas вместе с .nanz, остальные frontend/profile/backend ограничения сохраняются; unsupported decimal-runtime отказ проверен доoutputwrite.

**Factorial остается отдельным незакрытым production путем:** исходный main на тестовом CALLmain/HALT loader выводит5040CRLF при обоихBDOShost, но WriteI16/_wr16_dig не получили trustedruntime admission. Весь источник сохранён. Чистый FACT0..12 проверен независимым productmod65536: raw/finalVM + assemblednative3modes (39calls каждого). SignedInteger semanticdecode черезint16, VM/nativeassert принимают физическиеunsignedwordbits: FACT8bits40320decode−25216. Это не полный i16domain; основнойFact7=5040безoverflow. Main constantfolds Fact7, поэтому main не заявлен как recursiveCALLproof. VMASMstdout не симулируется.

Проверки: CPMMIR checker .054s; newnear-miss+Hello+originalREADME/impl/parser/tokenizer focused1.659s; final Hello/FACT .273s; CLI .054s; frozen executable replaylog included. No fullpipeline rerun, no physicalCPM claim.

Public command from repo root:
```
/mnt/safe/minz/grace/2026-10-12/pascal-cpm-main/executables/minzc examples/pascal/hello.pas -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o hello.com
```
Add `--grace --hir-grace --Osize` forSize or `--grace --hir-grace` forSpeed. Commands/ASM/COM/raw+finalMIR/ABI/stdout/BDOS9readledger archived. Frozen compiler/testpaths+SHA in frozen-executables.json; pipeline.test replay from minzc/pkg/pipeline with `-test.run '^(TestPascalHelloProductionCPM|TestOriginalPascalFactorialPureModel)$' -test.v -test.count=1`. Цель всего корпуса не объявлена завершённой.
