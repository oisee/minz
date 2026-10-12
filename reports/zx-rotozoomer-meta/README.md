# Pure Nanz compile-time rotozoomer

@roto_tables generates six 32-entry word tables on the MIR 2 VM at compile time; @roto_pixels emits 32 scoped pixelgroups. Runtime still samplesmutabletexture. Narrowbyte textureindex eliminates wordmultiply. All 32 frames inordinary/GraceSize/GraceSpeed match independentmodel/rawVM/finalVM/native. Generatornotretainedat runtime; generatedtablevalues checked; allcyan texturemutation checked.

10447 bytes,1885669 T forframe 0 (baseline 1291 bytes,2451932 T):23.1%fewerT atsubstantialcodesizecost. No additional Grace gain. Fastercombinedmeta+ASMvariant is ../zx-rotozoomer-kernel . Tests/sources/actualbinary/screens savedhere. ULAcontention/physicalhardware notmeasured.
