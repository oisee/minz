# Contributing to MinZ

MinZ is an experimental compiler. Reproductions and small, measured fixes are especially useful.

Build the core tools from the repository root:

```sh
make -C minzc mz mza mze
```

Run the [README hello program](../examples/nanz/hello_cpm.nanz) to check the local toolchain. For a compiler change, run focused Go tests in `minzc/` and the relevant example or corpus. The [fun build check](../fun/check.sh) exercises its showcase sources and records known assembler blockers. The repository's pull-request CI runs the required gate.

Open an issue or pull request with the source file, exact command, expected result, actual result, and compiler revision. If you change a documented feature, update its guide or example. Keep dated measurements and research claims in [reports](../reports/) or [docs](../docs/), rather than expanding the root README.

The [project backlog](../docs/project/BACKLOG.md) and [architecture guide](../docs/minz-compiler-architecture.md) provide context for larger work.
