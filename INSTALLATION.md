# Install MinZ from source

The compiler is a Go project in `minzc/`. Use Go 1.24.3 or newer; the module's `go.mod` declares its toolchain version. Node.js is only needed for optional editor tooling, not for the compiler build.

```sh
git clone https://github.com/oisee/minz.git
cd minz/minzc
go build -o mz ./cmd/minzc
go build -o mza ./cmd/mza
go build -o mze ./cmd/mze
```

Run a compile-and-assemble smoke test from `minzc/`:

```sh
./mz ../examples/fibonacci.minz -o fibonacci.a80
./mza -o fibonacci.bin fibonacci.a80
```

`make test` runs the same smoke test and cleans up its output. `go test -short -timeout 5m ./pkg/mir2/... ./pkg/z80asm/... ./pkg/c89/... ./pkg/codegen/... ./pkg/parser/... ./pkg/pipeline/...` runs the current PR test set.

The two optional submodules contain external PL/M corpus material and ZVDB examples. Fetch them only if you need those inputs:

```sh
cd ..
git submodule update --init corpus/intel80tools examples/zvdb-minz
```

See [README.md](README.md) for language examples and [BACKLOG.md](BACKLOG.md) for the current support and test status.
