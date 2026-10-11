# Fresh final decimal-ABI CP/M static image

Compile and assembly only, no COM execution. The exact e51846f1 CLI SHA9ec48 compiles the separately named derived source574cef, which changes only the main input pointer literal to CString. Original source0e926 remains preserved unchanged.

The emitted ordinary ASM is preserved. Exactly22 OUT(23),A sites become calls to the six-pair-preserving BDOS2 bridge. The empty peek/poke bodies receive only their declared HL→A / HL,C ABI implementations. The one known main XOR A/DI/HALT terminal becomes XOR A/RET. Startup remains the independently reviewed single-ORG0100 wrapper with incoming BDOS/SP checks, private6002..7002 stack, two canaries, explicit status and restored system SP/JP0.

A fresh MZA built from e51846f1 emits2574 COM bytes. Every structured listing byte maps exactly once to COM[address-0100], with no holes or overlap; imageend remains below6000. COM SHA256087a9c7376591a82169b963891493ab6389cf26ebab728ef08a63e7664c73112. Build/source/wrapper/assembler identities and complete artifacts are adjacent.

Actual execution requires independent final review and root release. Warmboot exit0 will not count as success: capture exact output, wrapper status1, restored savedSP, both canaries, parser globals/regions, BDOS boundary and finite execution termination through MZE profile memory snapshot. Expected parser shape is17nodes (8088 node bump),24intern bytes (9018 string bump),pos45. This remains emulator CP/M/BDOS evidence, not a physical operating-system run or a PGO certificate.
