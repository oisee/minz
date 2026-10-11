# Frozen source review:c9c3e1dc

Exact owner9-path inventory was recomputed and matches:five production files andfour test files. Workingtree was clean when read. No remaining concrete source acceptance/correctness hole was identified in this bounded review. The previous name-capture and UTF8byte-reencoding blockers are closed.

Name hygiene skips mir2.Ty before traversing recursive descriptor pointers, tracks visitedAST pointers, reservesName/Var/Names fields plusglobals/activeenv/aliases, andusesfreshsequence for uniqueness. Tests separately cover syntheticfuturetuple/loopbinders andreal supportedrecursiveparameter lowering; these are not mislabeled as actualmachine ABI tests.

VM storage validates totalMaxMemory beforeanyheap/cachemutation, then appliesS/L/Ckind representation andformatlengthlimits. Boundary65535 is tested with explicitlylargerVMmemory; default64K rejection is atomic. ParserchecksprocessedUTF8bytecounts. Nativeemptydata syntax is canonicalDB0/DB0,0 withoutdanglingcomma. Directliteral@print choosescountedS/L; opaque pointerCStringroute is unchanged. VMprint.char nowwritesrawbytes, andactual builtin raw/optimizedVMtestscheckfirstloads2/65/2 plusUTF8/NUL.

Recommend one bounded owner nativegate:TestLiteralPrintingNative's12 independentoutput cases andTestLiteralStorageNative's3 realfirstload/pool cases, using fullpipeline/PBQPassembledbinaries. ExpectedbytescoverS/C/L,empty,embeddedNUL,UTF8,S255,L256,adjacentCStringseparator, liveglobal4660→4661 andunchangedcollisionglobals, terminalPC/SP. Do notduplicateCPU here. This validatesoutput andstate, but no generic allreads/nooverreadclaim is made withoutstepwiseaccess instrumentation. Full65535payload is admissiontestonly, notnativeimagecapacityclaim.

Native tests were compiled butnotexecuted atreviewtime. Actualterminalandretainedsource/binaries/logs remainroot'snextgate; no broadPASS claimed. OriginalSelfLanz main'splainliteralNULscan remainsdistinctsourcebug andisnotchangedbythese5productionfiles.
