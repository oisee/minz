# Spectrum disk transport reuse audit

Checked 2026-10-26, after Alice suggested bundling an openly licensed DOS to
simplify native staged file utilities. This is a component/source audit, not a
claim that all historical ROM rights have been established.

| Candidate | What was actually found | Integration decision |
|---|---|---|
| Original TR-DOS / Beta Disk ROM | The inspected project describes its input as the original TR-DOS5.03 ROM and its output as a disassembly. No MIT/BSD/public-domain grant for that ROM was found in the inspected project. Public availability of the dump is not a redistribution grant. | No ROM bundled. Keep ROM execution distinct from our API traps. |
| Scorpion ProfROM R8DOS | ProfROM release notes explicitly name R8DOS and RST08 services. Inspected Scorpion hardware/ROM repository contains firmware images and release notes, but no verified permissive ROM grant. | Recognize this as the API Alice recalled; do not invent compatibility or bundle firmware on the basis of a hardware repository. |
| +3DOS | Distinguish the original OS ROM from third-party disk-image tools. `ha1tch/plus3` is an Apache-2.0 Go image-management tool, not a +3DOS replacement ROM. | Possible independent format tool later; not evidence that the original ROM is MIT. |
| UnoDOS3 | Real Z80 OS sources with GPL-3.0 LICENSE. README specifies RST8 calls, `unodos.rom` + `unodos.sys`, and divMMC/Chloe280SE deployment. | An open external-firmware candidate, not a permissive/drop-in Beta Disk ROM. No GPL code or firmware imported into the MIT emulator by this audit. |
| NedoOS | Available source/mirror and TR-DOS/FAT/pipes functionality. No project-wide MIT/BSD grant confirmed. Component licenses must be distinguished: the inspected `release/doc/license.txt` contains special beta/development restrictions and mentions compiler/assembler code. | Do not label the entire kernel or all bundled tools MIT; resolve component-specific terms before reuse. |
| Existing MinZ TRD/API implementation | Repository MIT LICENSE; `minzc/pkg/spectrum/formats/trd.go` already implements TRD directory/sector handling, file search/load/save and `$3D13` API traps with explicit writeback. | Reuse this implementation for the first Spectrum staged transport. Strengthen/test the needed API subset rather than silently treating it as complete ROM behavior. |

No external ROM has been copied or enabled by this audit. The main file-pipeline
work can proceed using the existing MIT image/API layer. Native target code must
still follow its documented register, workspace and paging contracts; an API-trap
test is not a physical ROM test. In particular, the legacy simplified stubs in
`pkg/emulator/io_interceptor.go` must not be confused with the Spectrum machine's
actual `pkg/spectrum/formats/trd.go` path.

## Inspected primary sources

- [TR-DOS Disassembled README](https://github.com/programandala-net/tr-dos/blob/master/README.adoc)
- [ProfROM release notes, R8DOS services](https://speccy4ever.speccy.org/doc/ProfROM4.xx.015_ChangeLog.pdf)
- [Scorpion256TPlus hardware/ROM repository](https://zxgit.org/romych/Scorpion256TPlus)
- [UnoDOS3 README](https://github.com/source-solutions/unodos3/blob/main/README.md)
- [UnoDOS3 LICENSE](https://github.com/source-solutions/unodos3/blob/main/LICENSE)
- [NedoOS source mirror](https://github.com/alfishe/NedoOS)
- [NedoOS component beta license](https://github.com/alfishe/NedoOS/blob/main/release/doc/license.txt)
- [plus3 Go image tool and Apache-2.0 LICENSE](https://github.com/ha1tch/plus3)
- [esxDOS project API/features](https://www.esxdos.org/index.html): it documents RST08 file calls and TRD emulation, but that feature list is not an MIT license grant.
- Local `LICENSE`, `minzc/pkg/spectrum/formats/trd.go`, `stdlib/zx/scorpion_disk.nanz`.
