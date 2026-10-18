# Native ABAP frontend services on CP/M

This slice compiles the unchanged `examples/abap/zpickfile_cpm.abap` report into
native Z80. F4 opens a CP/M file picker, GUI_UPLOAD reads ASCII records into owned
character table rows, SPLIT constructs typed fleet rows, and WRITE prints the
rows and total. SALV and the Agon adapter are separate subsequent slices.

## Contract and admission

The reference is a **specification oracle**, not an SAP execution trace.
The independent abaplint runtime does not implement frontend services.
The official contracts used are:

- [FILE_OPEN_DIALOG](https://help.sap.com/docs/abap_platform_new/5a005e044eef436f8b27bbd3f73a3cfc/dd66b1a76d7044ff8fd46c04fdaec220.html): RC -1 failure, 0 cancel, positive selection count; ACTION_OK 0 / ACTION_CANCEL 1; fresh output table/status.
- [DIRECTORY_LIST_FILES](https://help.sap.com/docs/SAP_NETWEAVER_731_BW_ABAP/5a005e044eef436f8b27bbd3f73a3cfc/50fcd03965224789a52558a2e29ed834.html): directory/filter produce a file table and count.
- [GUI_UPLOAD](https://help.sap.com/docs/SAP_NETWEAVER_750/5a005e044eef436f8b27bbd3f73a3cfc/1dac0155370648569fe843170e07c4da.html?version=7.5.21): ASC text; READ_BY_LINE='X' retains CR/LF, so this initial stripping implementation explicitly refuses that option.
- [SPLIT](https://help.sap.com/docs/SAP_NETWEAVER_731_BW_ABAP/cfae740a0a21455dbe6e510c2d86e36a/fceb33f3358411d1829f0000e829fbfe.html): final target retains remaining delimiters; actual target truncation sets SY-SUBRC 4.

Only the named methods/options listed in the task are admitted. Unknown methods,
sections, options, exceptions, multiselection, BIN/DAT, codepages, separators,
READ_BY_LINE, GUI_UPLOAD function-module form and SPLIT INTO TABLE are refused.
Some syntactically invalid call forms are rejected by the native ABAP grammar
before semantic admission. There is no generic method fallback or DATASET code.

The CP/M adapter accepts flat current-drive/A: ASCII 8.3 names. Directory inputs
are empty, `.` or `A:`. Patterns support complete `*` components and `?` positions,
not host paths, partial stars or SAP's extended GUI filter descriptions. Unsupported
runtime values fail the method rather than access a host path. A catalog contains
at most 128 names; excess names fault 7 before publishing table rows. Unsupported
targets explicitly identify the CP/M-only boundary; Agon MOS/ZX require adapters.

GUI_UPLOAD strips CRLF/LF (also handles lone CR) and treats ^Z as EOF. Fixed c rows
truncate while consuming the entire input line. Owned bounded strings fault 3 on
exhaustion; non-ASCII bytes fault 4. Table overflow faults 7 before the next row
write, closes the file, and bypasses the report suffix. Mapped FILE_OPEN_ERROR and
OTHERS set SY-SUBRC; unmapped errors fault 8. Checked text-to-i conversion reuses
the decimal input grammar: invalid text fault 4, signed overflow fault 5.

## Implementation boundaries

Common compiled helpers execute in raw MIR2 VM, final MIR2 VM and native Z80.
Scripted tests supply only record-level BDOS calls, directory entries and keys;
they do not implement line parsing, picker, SPLIT or table operations on the host.
The six private leaf adapters implement open, close, read, search-first,
search-next and nonblocking key reads. Read/search temporarily select the private
DMA and restore CP/M's default 0x80 afterward. This is the explicit adapter
contract; arbitrary incoming DMA restoration is not claimed.

Native BDOS search FF cannot distinguish an empty match from an underlying
provider failure. Host catalog helpers retain a separate error, but native
listing maps FF to no entries. No hidden errno channel is invented. MZE resolves
listing and open through the same original-byte ASCII 8.3 normalization, rejects
case collisions, Unicode aliases, symlinks and invalid FCB padding/drives, and
sorts normalized names. Handles belong to FCB addresses; failed reopen clears the
old handle. Sequential reads preserve partial-record ^Z padding, distinguish EOF
from I/O errors and advance CR/EX/S2 without wrapping exhausted extents to zero.

Elementary c/string table rows own their byte images, including bounded string
headers; APPEND/READ copy values. Runtime locals are namespaced by the independent
runtime-bindings foundation fix, preserving source globals named n/i/count.
The line work buffer includes table width even without a line work area; a
1536-byte row reproduces memory corruption without that bound and passes with it.

## Frozen report and real CP/M result

| Fixture | SHA-256 |
|---|---|
| zpickfile_cpm.abap | df1637cf8701472602df14f2e101395c36477d34507be6cb519e03aaa798c93a |
| FLEET.CSV (actual CRLF) | 4bcb4a855c60745e65513227695bc1b11701cd2b3511c1bb7ff1e421fe8f6f49 |
| EXTRA.CSV (actual LF) | cf07eca0dfcff2cc1e7160e30b917c63c18ea7119915f4724401eb2148dfbb62 |

`TestABAPFrontendCPMCLI` checks these identities and actual .a80/.com byte equality.
Real MZE receives `ESC O S`, Down (`ESC [ B`), Enter, Enter over stdin:

```text
Atlas North 40
Beacon South 32
Total seats: 72
```

The cancel case retains EXTRA.CSV and prints `Spare Depot 8` / total 8. A missing
filename prints `File not found`. Seventeen input rows fault 7 during upload;
invalid numeric text faults 4. Both terminal faults print the bounded application
entry diagnostic and bypass report output, then return to CP/M with the fault in
A and `_abap_error_code`. MZE's shell exit status is 0 for an OS return; it does
not reinterpret arbitrary application A values as shell errors.

Measured application: COM 32,207 bytes, image end 0x7ECF, resident stack
0x7ACF..0x7ECF (1024 bytes). Select/cancel minimum SP 0x7E8D (66 bytes used);
missing/fault cases minimum SP 0x7EA7 (40 bytes used). Native sentinel tests and
real MZE independently verify return PC/SP, unchanged stack canary A5, and stored
faults. Real MZE returns PC=0000 / SP=FE00. The application entry additionally
checks actual CP/M vector/return-span admission; FE00 is a measured emulator TPA,
not a universal claim about real CP/M memory layouts.

## Validation and dependencies

Required dependency: reviewed ordinary CP/M application-entry commit
2aab6f711a4e0080807b2fdca34c0ad460b77df0 over main76495bd8f, applied separately.
Runtime binding foundation is separately reviewed/integrated3ff9faa33. The final
frontend patch excludes both dependency changes.

Tests cover raw/final VM and native Z80 for owned rows, SPLIT remainder/aliasing/
truncation, numeric conversion, record boundaries, CRLF/LF/^Z, long/empty input,
missing/read errors, capacity cleanup, directory filtering/publication, F4
selection/cancel and source-global preservation. Native provider tests cover
canonical identity, Unicode mismatch regression, collisions, symlinks, directory
slot wrap, failed occupied-FCB reopen, CR/EX/S2 rollover, invalid extent, reset,
drive change and failed/new search iterator lifetime.

Required mutants were run individually and restored: M1 retains CR, M2 cancellation
selects a file, M3 SPLIT drops remaining text. Each fails semantic assertions;
logs distinguish runtime failures from build errors. The wide-row bound also has
an independent red proof. Final gate status and exact commands are preserved in
`/tmp/minz-abap-f4-alv-evidence/frontend-services`, mirrored by the coordinator to
the persistent artifact root. No network, commit or push was performed by this
worker.
