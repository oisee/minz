# Corrected native15 saved audit

PASS:12 outputcases and3 storagecases from the corrected retained executable8d60ea6b6a80a76e89046cde9bbeacd9946ea5abc85a6883f469e5821f8801db. Every output was checked against independent byteconstants includingUTF8,embeddedNUL,empty,S255,L256 andtheCStringneighborseparator. All12 preservedcollisionglobals and committedanswer4661. All3 storagecases matchednativepoolencoding andactualfirstloadsS2/C65/L2. Exactsavedsource text, bootstrap+targetASM, terminalHALTbytePC/SP,errorabsence andpositivecycles were checked.

An independent offline assembler rebuiltall15 savedassembledtexts andmatchedbinary.bin byte-for-byte. This wasassembly only, notCPUreplay. The source/output/storage expectedvalues didnotcomefromVMexecution. First15failurehistory remainsuntouched; correctedpass doesnoterase the realallocated-argument issue or twofixtureissues.

This proves the boundedcases'output/state andsource→ASM/BINbinding. Itdoesnotprove absenceofeveryextra native dataread, genericintrinsicABI correctness, orfull65535payloadfit. NoCPU wasexecuted bythisreviewer. Portable tools/audit_strings.py supportsraw/gzipartifactfiles; tools/reassemble_strings.go reproducesassemblybinding fromproject'sminzc module.
