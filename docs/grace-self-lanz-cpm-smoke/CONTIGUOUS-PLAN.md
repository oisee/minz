# Contiguous CP/M diagnostic

This static variant preserves the old ordinary assembly and original literals. It is not a runnable faithful main demonstration: the known source CString assumption remains unresolved. No execution has occurred.

A single ORG 0100 emits only code/data. Stack addresses are EQU constants; startup initializes both canaries in RAM. Before any arena or stack writes, the BDOS pointer at 0006 must cover D000, the end of the source region. The final reviewed runnable wrapper will also check incoming SP against all owned regions and check canaries after main.

Assembly review must verify every listing byte equals COM[address-0100], COM size equals image_end-0100, and code_end remains below 6000. The historical multi-ORG image remains preserved as a rejected mapping diagnostic.
