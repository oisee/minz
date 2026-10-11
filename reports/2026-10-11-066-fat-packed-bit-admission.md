# FAT packed-buffer: BIT admission correctness fix, further gap retained

Prospective workloads were frozen in08c24990/aea2289c before new candidate measurements. Full unchanged FAT12 module atf5c7a09e: write_fat12 evencluster4,value683 wrote low176 rather than171. Raw/finalVM independently agreed on expectedpacked53931(d2ab); native did not. Cause: scanBitCmpPatterns removed AND/zero for clst&1!=0, but physicalIY source cannot encode BIT0,IYL. genCmp correctly rejected invalid BIT encoding, then ordinary fallback used values whose definitions had already been skipped.

Matcher now verifies actual physical encodability **before** admitting/skipping definitions. IX/IY register halves and spilled register operands retain ordinary AND/shift/zero path; memory BIT only admits HL offsets0/1 or indexed legal displacement. No assumption that rejected opcode means NOP. Regression executes1536cases: IX/IY/spill × direct/shifted bit × all256values, hostile stale destination/zero registers, independentexpectedparity. Existing no-invalid-ASM-only tests were insufficient.

This first fix passes the original evencluster4 packed write and read/FAT16controls. Oddcluster9,value3445 **still fails**, retained in diagnosis log: expected low90 versusactual80. Separate diagnosis is liveHL overwritten by genShift16 routing IX destination throughHL. This commit does not claim all packed workload correct; the failure remains a required next fix. Full packed native test intentionally stillfails until then.

Existing public measured-mode API refuses the full original module because unreachable disk_read/disk_write extern declarations remain; no winner is returned. BothSpeed/Size refusal retained, notrelabelled success. Closed reachable-function admission remains nextaftercorrectness.

Combined directory workload atf5c7a09e passedall18: default13574T9668B, GraceSize13574T9846B, GraceSpeed10948T9862B. Earliercorrect13828T baseline/11202T hoist contexts become254T cheaper, exactlytwo observed st_word gains127T; only these measured paths are compared. CurrentBITfix changeswholemodulecontextsizes afterward, so olddirectoryfigures remain historical, not rewritten.

All exact baseline and firstfix source/MIR/ASM/BIN/allocation/log available underreports/fat-packed-prospective, futurefixedartifactsseparate. Originalsource/nibbleoracle unchanged. No actualdisk/CPMOS/automaticheldoutselection claim.
