# Fix review:5bd9f775 plus pointer-cycle guard

The original two blockers are closed by the reviewed code. VMprint.char writes exactlyone byte toStdout and reports writeerrors; the new actualbuiltin stdout test checks UTF8C3A9, embeddedNUL and aCString separator without replacingprint.char. Generatednames now scan originalHIR functions, globals, active environment and aliases. Global/local collision fixture preserves sourcevalues. NativePBQP output/storage/liveglobals fixture is meaningful but was still unexecuted when read.

The reflective scanner's pointervisited guard prevents recursiveTy/AST pointercycles. Traversal uses directkind/String rather thanInterface on unexportedtypefields, so this review found no reflectionprivatefield panic. Interfacesunwrap into pointerchecked traversal. No maps are traversed, but sourcebindingnames occur in originalfunction declarations rather thanSourceLocations maps.

One minor coverage gap was sentowner: scanner recognizesName fields but notTupleLetStmt.Names orForRangeStmt.Var/ForEachStmt.Var. Activeenv andlexicalrestore alreadyprotect currentlylivebindings, so this is not a demonstratedwrongcode hole. Explicitlyreserve these bindingfields to meet the promised allsourcebindingnamespace contract; unitguard can verify this without nativeCPU.

This read is bounded, not a finalfreeze claim. FinalsourceSHA, focusedtests and the pending actualmachine output/storage fixtures remain owner/root gates. No CPU or ownerfilemutation bythisreviewer.
