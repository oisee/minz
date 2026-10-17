# Native ABAP parser evidence

The normal MinZ frontend uses the Go library in `minzc/pkg/abap/internal/abapparse`.

To independently regenerate reference JSON, install exactly `@abaplint/core@2.120.56` in an isolated Node 22 directory, put `parity.mjs` next to its `node_modules`, and run it on each source in the library's `testdata`. The script calls the upstream parser and ParserError rule directly. Node is needed only for this reference regeneration, not for ordinary compilation or frozen-reference tests.

The pinned abapiti recipe and generation hashes are documented in the library's `PROVENANCE.md`. `install-generated.py` installs only the exact recorded generation and verifies all inputs before writing any files.

Acceptance checks from `minzc`:

```sh
go test ./pkg/abap/...
```

The CLEAR regression sets PATH to an empty directory, explicitly calls the FORM, and checks the actual global object in raw/final VM and assembled Z80. It also checks initial memory and native PC/SP. The complete objective is in `reports/native-abap/objective.md`; numeric, string and CHANGING semantics and two independent full-program references are still required.

## Independent FORM semantics reference

The six source/result pairs in `minzc/pkg/abap/testdata/form-reference` use the independent `@abaplint/transpiler` and `@abaplint/runtime` 2.14.8. They are not executed by MinZ to obtain expected values. Each JSON records the source SHA256, provider version, Node version and observed globals. Tests refuse a modified source.

To reproduce in an isolated directory, copy `reference.mjs`, `semantic-package.json` as `package.json`, and `semantic-package-lock.json` as `package-lock.json`; run `npm ci`, then `node_modules/node/bin/node reference.mjs SOURCE.abap`. Node is a reference-generation dependency only. The harness calls the registered ABAP FORM probe after loading the unchanged source and snapshots the global objects.

Real PERFORM calls use the reference entry. For compatibility with compiler assertion comments, a FORM with exactly one CHANGING parameter also has a plain-name scalar wrapper: its USING arguments are separate local objects, its output starts at zero, and the wrapper calls the same reference implementation. This wrapper is an external test convention; it is not the semantics of PERFORM.

Untyped FORM parameters and VALUE/LIKE modes are explicitly refused until implemented. Literal USING arguments are admitted only when a conservative, transitive effect check proves the corresponding formal cannot be written. CHANGING and writable USING require actual variables.

## Independent division reference

Copy `division-reference.mjs` into the isolated semantic-reference directory
prepared with the package and lock files above. Run:

```sh
node_modules/node/bin/node division-reference.mjs /ABS/PATH/arithmetic.abap /ABS/PATH/inputs.json
```

Both inputs live in `minzc/pkg/abap/testdata/arithmetic-reference`. The script
uses a fresh reference runtime for each case, injects globals after loading
the unchanged source, calls FORM probe and reads actual global objects. The
JSON fingerprints both source and inputs. This runtime is not an overflow
oracle because its Integer.set overflow check is disabled; fault expectations
are specification-derived. Node remains a reference-generation dependency only.

## Complete character reports

Copy `character-reference.mjs` into the isolated semantic-reference directory
with the pinned package/lock files above. Run
`node_modules/node/bin/node character-reference.mjs SOURCE.abap` on each
source under `minzc/pkg/abap/testdata/character-reference`. This script executes
the full report, with no additional FORM call or injected inputs. It captures
actual globals and MemoryConsole output; JSON fingerprints the complete source.
The MinZ test checks unchanged-source execution in raw/final VM and assembled
Z80. Ordinary HALT and CalledMain return contracts are checked explicitly.
These reports establish fixed ASCII character behavior, not full dynamic
string, selection UI or SAP runtime compatibility.

## Complete owned-string reports

Use `string-reference.mjs` with the same isolated pinned semantic-reference
package/lock files. Run `node_modules/node/bin/node string-reference.mjs SOURCE.abap`
on the sources under `minzc/pkg/abap/testdata/string-reference`. It executes
the complete source and records x/y/c/seen plus MemoryConsole output, without
extra calls or mutations. These fixtures test copy-only dynamic string lengths
and reference parameters, including wide integer state. MinZ's bounded storage
capacity faults are separate contract probes, not a property of the reference
runtime's allocator.
