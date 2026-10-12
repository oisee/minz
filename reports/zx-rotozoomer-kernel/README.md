# Metafunction-generated ZX 32×48 rotozoomer

The half-cell mode retains Alice's PAPER/INK split: upper 8×4 is PAPER, lower 8×4 is INK, sharedBRIGHT. Bitmap is fixed, only 768 attributes change.

Two genuine Nanz compile-time metafunctions run on the MIR 2 VM. @roto_tables emits eight 32-word setup tables: steps, truncated halfsteps, origins and row increments. @roto_kernel emits a Z 80 walker with four pixel operations per loop. The generators emit Nanz text containing explicit asm; they are not retained as runtime code.

Applied antique-toy chapter 7 principles: prepare everything before the inner loop; use page-aligned texture (copied once to c 000), highbyte/nibble address construction, sequential output, register-held coordinates and generated repeated operations. EXX keeps loopcounters separate; alternateAF holds packedPAPER while selectingINK. No SMC and no storedframes. Source remains a Nanz program with a small explicitlygeneratedASM kernel.

| Variant | Bytes | Frame 0 T | Baseline ratio |
|---|---:|---:|---:|
| Pure Nanz baseline |1291|2451932|1.00|
| Nanz meta tables + unrolledNanz row |10447|1885669|1.30×|
| Meta setup + compactASM walker |1390|341163|7.19×|
| Meta setup + generatedfour-pixelASM group |1616|312363|7.85×|

Final ordinary/GraceSize/GraceSpeed allmatch:32 frames each, every 768 attribute and 6144 bitmap bytes, exactreturnPC/SP and immutablecode/data; preciselydeclaredspill/setup words aremutable. Changedruntime texture rendersallcyan, proving noframeplayback. Pure Nanzmeta variant additionallychecks raw/finalVM. Kerneltest executesactualASM; VMOpAsm doesnotserve asoracle. TestsPASS 2.775 s.

312363 T is roughly 11.2 fps at 3.5 MHz beforeULAcontention, interrupts anddisplaypacing. Actualframe 0 measureincludesNanzsetup/CALL/RET; textureinitializationexcludedasone-timecost. FullROMmainpacing andhardware notvalidated. Book's illustrative~10 Tperattributecell is not a completebudget forour twoindependent 8.8 samples; we useactualmeasuredT instead.

native-preview.gif decodes actualnative screen dumps with 90 ms previewdelay (not a hardwarecapture). Compactrotozoomer.bin loadsat 8000, entry 8000. Kerneltextureoccupiesc 000-c 0 ff; loadingrequires 48 KRAM withthatregionavailable. MainusesstandardROM EI/HALT; ROMrunner/TAP andfurtheroptimizationremainnextwork.

Reference: https://github.com/oisee/antique-toy/blob/7313 ecfaab 710 d 1 a 105 b 7 bff 3 be 19 aaf 04 dc 5935/chapters/ch 07-rotozoomer/draft.md . Test TestZXHalfCellRotozoomerKernelNative, export MINZ_ZX_ROTO_KERNEL_EXPORT. Pure Nanzvariant: ../zx-rotozoomer-meta/source.nanz, TestZXHalfCellRotozoomerMetaNative/MetaGeneratedSetup.
