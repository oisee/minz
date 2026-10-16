# Pinned native abaplint parser

Generated from abapiti commit `17ed93a9c60c8e0854f66b545ff33f8f92e8cdc7` and its embedded, SHA-verified abaplint core 2.120.56, commit `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

Generation uses `abapiti abaplint --target go`, with the default full-core scope: no reachability pruning for a selected workload. Generation reported 2055 classes, 75 interfaces and 108 fingerprinted overrides. The generated runtime can explicitly refuse untranslated operations. These refusals are errors; the API never falls back to Node.

`hir.go`, `runtime.go`, and `promise.go` are generated files. The installation script verifies their input hashes, changes only the package name, removes the optional profiling build constraint and adds a generated-file header. Profiling-only duplicates and the command-line lint driver are omitted. `api.go` is the handwritten typed library adapter. Both upstream MIT licenses accompany the generated source.

## Reproduction

At the pinned abapiti checkout, build `./cmd/abapiti`, then run:

```sh
abapiti abaplint --target go -o /absolute/path/to/generated
python3 research/native-abap/install-generated.py /absolute/path/to/generated/go
```

Run the installer from the MinZ repository root. It refuses a generation that differs from the recorded inputs. This first integration deliberately retains the complete generated closure; its 34-MB source size is a known engineering cost. A later reduction must preserve general parser reachability and repeat parity checks, rather than prune against the selected example workload.

## Reference fixtures

`testdata/*.abap` and matching JSON trees were generated independently using TypeScript abaplint 2.120.56 on Node 22.23.3. The reference adapter is `research/native-abap/parity.mjs`; it exports structural and flat trees, leaf tokens, positions and the upstream ParserError diagnostics. The ordinary Go tests read frozen JSON and do not need Node/npm. These ten cases are parser parity evidence, not an ABAP semantic conformance claim.
