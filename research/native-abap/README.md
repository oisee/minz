# Native ABAP parser evidence

The normal MinZ frontend uses the Go library in `minzc/pkg/abap/internal/abapparse`.

To independently regenerate reference JSON, install exactly `@abaplint/core@2.120.56` in an isolated Node 22 directory, put `parity.mjs` next to its `node_modules`, and run it on each source in the library's `testdata`. The script calls the upstream parser and ParserError rule directly. Node is needed only for this reference regeneration, not for ordinary compilation or frozen-reference tests.

The pinned abapiti recipe and generation hashes are documented in the library's `PROVENANCE.md`. `install-generated.py` installs only the exact recorded generation and verifies all inputs before writing any files.

Acceptance checks from `minzc`:

```sh
go test ./pkg/abap/...
```

The CLEAR regression sets PATH to an empty directory, explicitly calls the FORM, and checks the actual global object in raw/final VM and assembled Z80. It also checks initial memory and native PC/SP. The complete objective is in `reports/native-abap/objective.md`; numeric, string and CHANGING semantics and two independent full-program references are still required.
