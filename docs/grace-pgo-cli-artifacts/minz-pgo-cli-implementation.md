Implemented only in `minzc/cmd/minzc`; no commits.

- Collect: `--hir-profile-collect`, required `--hir-profile-workload`, optional `--hir-profile-run-result`.
- Apply: `--hir-profile`, required workload/model/`--hir-profile-decisions`; normal `-o` output.
- Uses existing context APIs and `ProfileSelection`. Legacy `--pgo` remains separate.
- Rejects incompatible flags/backends/frontends. Strict JSON rejects unknown fields, duplicates and trailing data. Limits: workload 4 MiB, model 64 KiB, profile 8 MiB.
- Requires external measured costs; no automatic measurement.

New profile tests, selected regression tests, build, and CLI collection smoke test passed using the requested Go and cache. Full command suite fails in existing callable-alias tests with `reassignment of callable alias "fp" is unsupported`.