# PL/M Hello: original main with explicit CP/M console contract

The sample now contains `CALL BDOS(2, C)` and an explicit NUL in
`.('Hello from PL/M-80!', 0)`. Undeclared BDOS is a deliberately narrow
compiler console intrinsic, not a guessed external ABI; declared procedures
of that name retain ordinary semantics. See [the contract](../../docs/PLM_CPM_Console.md).

Directed tests passed: the complete PL/M package, intrinsic rejection cases,
exact addressed-byte-list semantics, and the retained historical adapter
regressions. The historical source and earlier evidence were not overwritten.

The original current main passed raw MIR, optimized MIR, and assembled Z80 in
ordinary, Grace Size, and Grace Speed. Each native mode was repeated with BDOS
preserving registers and with BDOS destroying A/F/BC/DE/HL/IX/IY. All six runs
printed `Hello from PL/M-80!\r\n`, returned at PC=0/SP=fe00, and retained memory
outside the admitted stack/status/spill regions. The exact CALL5 instruction
and each BDOS2 output byte were independently checked.

Each COM image is 1183 bytes, including a requested 1024-byte runtime stack;
each native execution used 14135 T-states excluding host-emulated BDOS service
cost. No Grace gain is claimed. This is execution with a model of CP/M console
services, not a physical CP/M deployment.

The exported source, raw/final MIR, allocation, ASM, COM, execution summaries,
and test logs are retained below. The manifest covers every payload in this
package and excludes itself. These checks postdate the previous full production
gate and do not silently change its checkpoint identity.
