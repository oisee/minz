# Self parser whole-main work in progress

Freeze5eaa1249 base1fd3d63f: exact originalsource, two-function tiny.nanz input, independent tokenizer/interner/12ASTnodes/file/stdout/Lanz expected beforeexecution. Scope: valid restricted parser grammar in this source, not general Nanz. Input76B,29tokens,12nodes,55stringbytes, output76B.

Confirmed source bug: out_str reads NUL but emit_lanz passed four ordinary SString literals. Raw and optimizedVM output84bytes with lengthprefixes insteadexpected76. Bookv7§18.1 distinguishes SString and CString. Source-only2976ac66 changes exactlyfour out_str callsites to c literals, filenames remain SString. RawVM and threeoptimizationmodes nowmatch independentAST/tokens/strings/globals/stdout/filebytes. No compilerpointerconversionsemanticchange.

Independent native before revealed second bug: defaultextern file_read(pathptr,bufptr) assigns BOTHHL; file_write similarlybothHL+lengthDE. Linked distinctRET symbols and actualformalcallback show bothfile_readargs8039 insteadpath8039/bufC000. Authoritative artifacts before/native-abi; before-native.txt diagnostic. Earlier before/native path is a rejected harness attempt: empty adjacent externlabels sharedPC, wrongdispatch/fileempty, notnativecorrectnessproof. It is retained as historicalnegative only. GeneralexternalABI fix assignedneighbor agent; nativewholemain stillpending. Tests deliberately retainthis failure untilsharedABIcorrection. Sourceannotations notusedtohideit.

VM heap must leave room for late literalstorage; full64K preallocation rejectsstringstorage. Host peekextends missingbytes like MZV realfilehost. VM fakeASM notused. NativeABIcallbacks test-owned PC authority, formalhomes, exactarena/outputbounds, fileonlytiny.nanz/tiny.lanz, no filesystemwrites outsidefixture. ProductionRuntimeCPM/TUI notmodified. Final fullnative3modes, fullRAM/globalevents and positivepacket will follow afterABI donor.

This is an intermediate report, not whole-source green completion. Durable donors5eaa1249/36848952/2976ac66/dbcbd68f retain source/failures/progress. Canonicalshare /mnt/safe/minz/grace/2026-10-12/self-parser-whole-main/.
